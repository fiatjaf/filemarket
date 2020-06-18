package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/gorilla/mux"
	"github.com/kr/pretty"
)

type AnnounceRequest struct {
	InfoHash   [20]byte
	PeerId     [20]byte
	Downloaded int64
	Left       int64
	Uploaded   int64
	Event      string
	NumWant    int32
	IP         net.IP
	Port       uint16
}

type AnnounceResponse struct {
	Complete   int64                  `bencode:"incomplete"`
	Incomplete int64                  `bencode:"incomplete"`
	Interval   int64                  `bencode:"interval"`
	Peers      []AnnounceResponsePeer `bencode:"peers"`
}

type AnnounceResponsePeer struct {
	PeerId string `bencode:"peer id" json:"peer_id"`
	IP     string `bencode:"ip" json:"ip"`
	Port   uint16 `bencode:"port" json:"port"`
}

func trackerError(w http.ResponseWriter, err error) {
	bencode.Marshal(struct {
		FailureReason string `bencode:"failure reason"`
	}{err.Error()})
}

func handleAnnounce(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]

	ann, err := parseAnnounce(r)
	if err != nil {
		trackerError(w, err)
		return
	}

	pretty.Log(key)
	pretty.Log(ann)

	// is this the seller?
	spl := strings.Split(key, ":")
	var saleId = spl[0]
	var sellerHash string
	if len(spl) == 2 {
		sellerHash = spl[1]
	}

	// this represents a real purchase?
	var sellerId string
	err = pg.Get(&sellerId, `
SELECT seller FROM sales
WHERE id = $1 AND status = 'pending'
    `, saleId)
	if err != nil {
		trackerError(w, err)
		return
	}

	// build the announcement response
	resp := AnnounceResponse{
		Interval:   10 * 60 * 1000, // 10min
		Incomplete: 0,
		Complete:   0,
		Peers:      []AnnounceResponsePeer{},
	}

	// is this the seller?
	if makeSellerHash(saleId, sellerId) == sellerHash {
		// track the seller online
		peerj, _ := json.Marshal(AnnounceResponsePeer{
			PeerId: string(ann.PeerId[:]),
			IP:     ann.IP.String(),
			Port:   ann.Port,
		})
		err := rds.Set("fm:online:"+sellerId, string(peerj), time.Minute*30).Err()
		if err != nil {
			log.Error().Err(err).Str("seller", sellerId).
				Msg("failed to save online presence")
		}
		return
	} else {
		// is the seller is online?
		if peerj, err := rds.Get("fm:online:" + sellerId).Result(); err == nil {
			// add the seller to the announcement response
			var peer AnnounceResponsePeer
			json.Unmarshal([]byte(peerj), &peer)

			resp.Complete = 1
			resp.Peers = append(resp.Peers, peer)
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	bencode.NewEncoder(w).Encode(resp)
}

func parseAnnounce(r *http.Request) (ann *AnnounceRequest, err error) {
	ann = &AnnounceRequest{}

	for k, vv := range r.URL.Query() {
		v := vv[0]
		switch k {
		case "info_hash":
			ann.InfoHash = parseBinaryParam(v)
		case "peer_id":
			ann.PeerId = parseBinaryParam(v)
		case "port":
			p, err := strconv.ParseUint(v, 10, 16)
			if err != nil {
				return nil, err
			}
			ann.Port = uint16(p)
		case "left":
			p, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, err
			}
			ann.Left = p
		case "uploaded":
			p, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, err
			}
			ann.Uploaded = p
		case "downloaded":
			p, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, err
			}
			ann.Downloaded = p
		case "event":
			ann.Event = v
		case "numwant":
			p, err := strconv.ParseInt(v, 10, 32)
			if err != nil {
				return nil, err
			}
			ann.NumWant = int32(p)
		case "ip", "ipv4", "ipv6":
			ann.IP = net.ParseIP(v)
		}
	}

	if ann.NumWant < 0 {
		ann.NumWant = 50
	}

	if ann.IP == nil {
		ann.IP = net.ParseIP(r.Header.Get("X-Forwarded-For"))
		if ann.IP == nil {
			ann.IP = net.ParseIP(r.RemoteAddr)
		}
	}

	return ann, nil
}

func parseBinaryParam(s string) (buf [20]byte) {
	copy(buf[:], s)
	return buf
}

func makeSellerHash(saleId, sellerId string) string {
	h := sha256.Sum256([]byte(saleId + ":" + sellerId + ":" + s.SecretKey))
	return hex.EncodeToString(h[:])
}
