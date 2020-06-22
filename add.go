package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/fiatjaf/go-lnurl"
	"github.com/gorilla/mux"
	"github.com/lucsky/cuid"
)

func addFile(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	user := rds.Get("fm:auth-session:" + session).Val()
	if user == "" {
		w.WriteHeader(401)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1000000)
	err := r.ParseMultipartForm(1000000)
	if err != nil {
		w.WriteHeader(400)
		return
	}

	fileUploaded, _, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(400)
		return
	}

	name := r.FormValue("name")
	price, _ := strconv.Atoi(r.FormValue("price"))
	description := r.FormValue("description")
	fileId := cuid.Slug()

	if name == "" || price <= 0 || user == "" {
		w.WriteHeader(400)
		return
	}

	// load torrent file and sanitize it
	var torrentFileBytes []byte
	if torrent, err := metainfo.Load(fileUploaded); err != nil {
		log.Warn().Err(err).Msg("can't parse metainfo file")
		w.WriteHeader(400)
		return
	} else {
		info, err := torrent.UnmarshalInfo()
		if err != nil {
			log.Warn().Err(err).Msg("invalid metainfo info")
			w.WriteHeader(400)
			return
		}
		info.Private = lnurl.TRUE
		infobytes, _ := bencode.Marshal(info)
		torrent.InfoBytes = bencode.Bytes(infobytes)
		torrent.Announce = ""
		torrent.AnnounceList = nil
		torrent.Nodes = nil
		torrent.CreationDate = 0
		torrent.Comment = s.ServiceURL + "/#/file/" + fileId
		torrent.CreatedBy = "filemarket.bigsun.xyz"
		torrent.UrlList = nil

		torrentFileBytes, _ = bencode.Marshal(torrent)
	}

	// store file
	var file File
	metaj, _ := json.Marshal(Metadata{name, description})
	err = pg.Get(&file, `
INSERT INTO files (id, seller, price_msat, metadata, torrent)
VALUES ($1, $2, $3, $4, $5)
RETURNING `+FILEFIELDS+`
    `, fileId, user, price, metaj, torrentFileBytes)
	if err != nil {
		log.Error().Err(err).
			Str("seller", user).Int("price", price).
			Msg("failed to save file")
		w.WriteHeader(500)
		return
	}

	lnurlpay, _ := lnurl.LNURLEncode(s.ServiceURL + "/~/" + fileId)
	json.NewEncoder(w).Encode(struct {
		Id    string `json:"id"`
		URL   string `json:"url"`
		LNURL string `json:"lnurl"`
	}{
		fileId,
		s.ServiceURL + "/file/" + fileId,
		lnurlpay,
	})
}

func hostFile(w http.ResponseWriter, r *http.Request) {
	// this endpoint returns data private to the file seller
	// (the torrent file used to seed)
	file_id := mux.Vars(r)["file"]
	qs := r.URL.Query()

	// check permission
	session := qs.Get("session")
	user := rds.Get("fm:auth-session:" + session).Val()
	if user == "" {
		w.WriteHeader(401)
		return
	}

	var file File
	err := pg.Get(&file, `
SELECT `+FILEFIELDS+`
FROM files
WHERE id = $1
    `, file_id)
	if err != nil {
		w.WriteHeader(404)
		return
	}

	torrentFile, err := file.HostTorrentFile()
	if err != nil {
		w.WriteHeader(500)
		return
	}

	http.ServeContent(w, r, "filemarket-"+file.Id+".torrent",
		time.Now(), bytes.NewReader(torrentFile))
}
