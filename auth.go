package main

import (
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/fiatjaf/etleneum/types"
	"github.com/fiatjaf/go-lnurl"
	"gopkg.in/antage/eventsource.v1"
)

var userstreams = cmap.New()

func authUser(w http.ResponseWriter, r *http.Request) {

}

func authUserStream(w http.ResponseWriter, r *http.Request) {
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
	}()

	accountId := rds.Get("auth-session:" + session).Val()
	if accountId != "" {
		// we're logged already, so send account information
		go func() {
			time.Sleep(100 * time.Millisecond)
			var acct types.Account
			err := pg.Get(&acct, `SELECT `+types.ACCOUNTFIELDS+` FROM accounts WHERE id = $1`, accountId)
			if err != nil {
				log.Error().Err(err).Str("session", session).Str("id", accountId).
					Msg("failed to load account from session")
				return
			}
			es.SendEventMessage(`{"account": "`+acct.Id+`", "balance": `+strconv.FormatInt(acct.Balance, 10)+`, "secret": "`+getAccountSecret(acct.Id)+`"}`, "auth", "")
		}()

		// we're logged already, so send history
		go func() {
			time.Sleep(100 * time.Millisecond)
			notifyHistory(es, accountId)
		}()

		// also renew this session
		rds.Expire("auth-session:"+session, time.Hour*24*30)
	}

	// always send lnurls because we need lnurl-withdraw even if we're
	// logged already
	go func() {
		time.Sleep(100 * time.Millisecond)
		auth, _ := lnurl.LNURLEncode(s.ServiceURL + "/lnurl/auth?tag=login&k1=" + session)
		withdraw, _ := lnurl.LNURLEncode(s.ServiceURL + "/lnurl/withdraw?session=" + session)

		es.SendEventMessage(`{"auth": "`+auth+`", "withdraw": "`+withdraw+`"}`, "lnurls", "")
	}()

	es.ServeHTTP(w, r)
}
