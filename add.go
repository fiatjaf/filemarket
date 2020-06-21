package main

import (
	"encoding/json"
	"net/http"
	"strconv"

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

	// load torrent file and turn it into a magnet
	metainfo, err := metainfo.Load(fileUploaded)
	if err != nil {
		log.Warn().Err(err).Msg("can't parse metainfo file")
		w.WriteHeader(400)
		return
	}
	magnet := metainfo.Magnet(name, metainfo.HashInfoBytes())

	// store file
	var file File
	metaj, _ := json.Marshal(Metadata{name, description})
	err = pg.Get(&file, `
INSERT INTO files (id, seller, price_msat, magnet, metadata)
VALUES ($1, $2, $3, $4, $5)
RETURNING `+FILEFIELDS+`
    `, fileId, user, price, magnet.String(), metaj)
	if err != nil {
		log.Error().Err(err).
			Str("seller", user).Int("price", price).
			Str("magnet", magnet.String()).
			Msg("failed to save file")
		w.WriteHeader(500)
		return
	}

	lnurlpay, _ := lnurl.LNURLEncode(s.ServiceURL + "/~/" + fileId)
	magstr, _ := file.HostMagnet()
	json.NewEncoder(w).Encode(struct {
		Id         string `json:"id"`
		HostMagnet string `json:"host_magnet"`
		URL        string `json:"url"`
		LNURL      string `json:"lnurl"`
	}{
		fileId,
		magstr,
		s.ServiceURL + "/file/" + fileId,
		lnurlpay,
	})
}

func hostFile(w http.ResponseWriter, r *http.Request) {
	// this endpoint returns data private to the file seller
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

	magstr, err := file.HostMagnet()
	if err != nil {
		w.WriteHeader(501)
		return
	}

	json.NewEncoder(w).Encode(magstr)
}
