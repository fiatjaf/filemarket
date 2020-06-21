package main

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/fiatjaf/go-lnurl"
	cmap "github.com/orcaman/concurrent-map"
	"gopkg.in/antage/eventsource.v1"
)

var userstreams = cmap.New()

func authUser(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	k1 := params.Get("k1")
	sig := params.Get("sig")
	key := params.Get("key")

	if ok, err := lnurl.VerifySignature(k1, sig, key); !ok {
		log.Debug().Err(err).Str("k1", k1).Str("sig", sig).Str("key", key).
			Msg("failed to verify lnurl-auth signature")
		json.NewEncoder(w).Encode(lnurl.ErrorResponse("signature verification failed."))
		return
	}

	session := k1
	log.Debug().Str("session", session).Str("pubkey", key).Msg("valid login")

	// there must be a valid auth session (meaning an eventsource client) otherwise something is wrong
	ies, ok := userstreams.Get(session)
	if !ok {
		json.NewEncoder(w).Encode(lnurl.ErrorResponse("there's no browser session to authorize."))
		return
	}

	// get the account id from the pubkey
	_, err = pg.Exec(`
INSERT INTO users (id) VALUES ($1)
ON CONFLICT (id) DO NOTHING
    `, key)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("failed to ensure account")
		json.NewEncoder(w).Encode(lnurl.ErrorResponse("failed to ensure account with key " + key + "."))
		return
	}

	// assign the account id to this session on redis
	if rds.Set("fm:auth-session:"+session, key, time.Hour*24*30).Err() != nil {
		json.NewEncoder(w).Encode(lnurl.ErrorResponse("failed to save session."))
		return
	}

	es := ies.(eventsource.EventSource)

	// send notifications
	go sendUserNotifications(es, key, session)

	json.NewEncoder(w).Encode(lnurl.OkResponse())
}

func userStream(w http.ResponseWriter, r *http.Request) {
	var es eventsource.EventSource
	session := r.URL.Query().Get("session")

	if session == "" {
		session = lnurl.RandomK1()
	} else {
		// check session validity as k1
		b, err := hex.DecodeString(session)
		if err != nil || len(b) != 32 {
			session = lnurl.RandomK1()
		} else {
			// finally try to fetch an existing stream
			ies, ok := userstreams.Get(session)
			if ok {
				es = ies.(eventsource.EventSource)
			}
		}
	}

	if es == nil {
		es = eventsource.New(
			&eventsource.Settings{
				Timeout:        5 * time.Second,
				CloseOnTimeout: true,
				IdleTimeout:    1 * time.Minute,
			},
			func(r *http.Request) [][]byte {
				return [][]byte{
					[]byte("X-Accel-Buffering: no"),
					[]byte("Cache-Control: no-cache"),
					[]byte("Content-Type: text/event-stream"),
					[]byte("Connection: keep-alive"),
					[]byte("Access-Control-Allow-Origin: *"),
				}
			},
		)
		userstreams.Set(session, es)
		go func() {
			for {
				time.Sleep(25 * time.Second)
				es.SendEventMessage("", "keepalive", "")
			}
		}()
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		es.SendRetryMessage(3 * time.Second)
		es.SendEventMessage(session, "session", "")
	}()

	key := rds.Get("fm:auth-session:" + session).Val()
	if key != "" {
		// we're logged already, so send account information
		go func() {
			time.Sleep(100 * time.Millisecond)
			go sendUserNotifications(es, key, session)
		}()

		// also renew this session
		rds.Expire("fm:auth-session:"+session, time.Hour*24*30)
	}

	es.ServeHTTP(w, r)
}

func sendUserNotifications(es eventsource.EventSource, key, session string) {
	es.SendEventMessage(key, "id", "")

	var walletKey string
	err := pg.Get(&walletKey, `
SELECT wallet_key
FROM users
WHERE id = $1 AND wallet_key IS NOT NULL
    `, key)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Error().Str("key", key).Err(err).
				Msg("failed to retrieve user wallet key")
		}
		return
	}

	es.SendEventMessage(walletKey, "wallet-key", "")

	details, err := lnp.Wallet(walletKey).Details()
	if err != nil {
		log.Error().Str("key", key).Str("wallet_key", walletKey).Err(err).
			Msg("failed to retrieve lnpay user wallet info")
		return
	}

	es.SendEventMessage(strconv.FormatInt(details.Balance, 10), "balance", "")
}
