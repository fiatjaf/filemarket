package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	Complete   int64                  `bencode:"complete"`
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
		log.Warn().Err(err).Str("url", r.URL.String()).Msg("failed to parse announce")
		trackerError(w, err)
		return
	}

	pretty.Log(key)
	pretty.Log(ann)

	// build the announcement response
	resp := AnnounceResponse{
		Interval:   10 * 60 * 1000, // 10min
		Incomplete: 0,
		Complete:   0,
		Peers:      []AnnounceResponsePeer{},
	}

	// build the peer object to be stored later
	peerj, _ := json.Marshal(AnnounceResponsePeer{
		PeerId: string(ann.PeerId[:]),
		IP:     ann.IP.String(),
		Port:   ann.Port,
	})

	spl := strings.Split(key, ":")
	var id = spl[0] // can be either a saleId or a fileId
	var sellerHash string
	if len(spl) == 2 {
		sellerHash = spl[1]
	}

	var file File
	err = pg.Get(&file, `
SELECT `+FILEFIELDS+`
FROM files
WHERE id = $1
   OR id = (SELECT file_id FROM sales WHERE id = $1)
    `, id)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Error().Err(err).Msg("database error on /announce")
		}
		trackerError(w, err)
		return
	}

	side := ""

	if makeSellerHash(file.Seller) == sellerHash {
		// this is the seller
		side = "seller"
	} else if id != file.Id {
		// this is the buyer: key == saleId
		side = "buyer"
	}

	log.Debug().Str("file", file.Id).Str("seller", file.Seller).Str("side", side).
		Msg("announce")

	// is this the seller? key == fileId + hash(sellerId, secret)
	if side == "seller" {
		// track that he is online and store a peer object to serve later to the buyer
		pstr := string(peerj)
		err := rds.Set("fm:online:seller:"+file.Seller, pstr, time.Minute*30).Err()
		if err != nil {
			log.Error().Err(err).Str("seller", file.Seller).
				Msg("failed to save online presence")
		}
		rds.Expire("fm:online:seller:"+file.Seller, time.Minute*30)

		// also store the fact that the seller was online today
		rds.HSet("fm:seeded:"+file.Id, time.Now().Format("20060102"), "t")
		rds.Expire("fm:seeded:"+file.Id, time.Hour*24*90)

		// if a buyer is online, tell that so the seller may attempt to connect
		if d, err := rds.HGetAll("fm:online:buyer:" + file.Id).Result(); err == nil {
			for _, peerj := range d {
				var peer AnnounceResponsePeer
				json.Unmarshal([]byte(peerj), &peer)

				resp.Incomplete += 1
				resp.Peers = append(resp.Peers, peer)
			}
		} else {
			log.Warn().Err(err).Msg("error fetching buyer peers")
		}
	} else if side == "buyer" {
		saleId := id

		// track that this buyer is online so later the seller can try to connect
		rds.HSet("fm:online:buyer:"+file.Id, saleId, string(peerj))
		rds.Expire("fm:online:buyer:"+file.Id, time.Minute*30)

		// if the seller is online, tell that to the buyer
		if peerj, err := rds.Get("fm:online:seller:" + file.Seller).Result(); err == nil {
			log.Print("seller " + peerj)
			// add the seller to the announcement response
			var peer AnnounceResponsePeer
			json.Unmarshal([]byte(peerj), &peer)

			resp.Complete = 1
			resp.Peers = append(resp.Peers, peer)
		} else {
			log.Warn().Err(err).Msg("error fetching seller peer")
		}
	} else {
		trackerError(w, errors.New("you have no access to this file."))
		return
	}

	pretty.Log(resp)

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

	if ann.IP == nil || isUnusableIP(ann.IP) {
		ann.IP = net.ParseIP(
			strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0],
		)
		if ann.IP == nil {
			spl := strings.Split(r.RemoteAddr, ":")
			remoteIP := strings.Join(spl[0:len(spl)-1], ":")
			remoteIP = strings.Trim(remoteIP, "[]")
			ann.IP = net.ParseIP(remoteIP)
		}
	}

	return ann, nil
}

func parseBinaryParam(s string) (buf [20]byte) {
	copy(buf[:], s)
	return buf
}

func makeSellerHash(sellerId string) string {
	h := sha256.Sum256([]byte(sellerId + ":" + s.SecretKey))
	return hex.EncodeToString(h[:])
}

func isUnusableIP(ip net.IP) bool {
	return ip.Equal(net.IPv4allsys) ||
		ip.Equal(net.IPv4allrouter) ||
		ip.Equal(net.IPv4zero) ||
		ip.Equal(net.IPv4(127, 0, 0, 1)) ||
		ip.Equal(net.IPv6zero) ||
		ip.Equal(net.IPv6unspecified) ||
		ip.Equal(net.IPv6loopback) ||
		ip.Equal(net.IPv6interfacelocalallnodes) ||
		ip.Equal(net.IPv6linklocalallnodes) ||
		ip.Equal(net.IPv6linklocalallrouters)
}
