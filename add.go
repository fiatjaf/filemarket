package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/fiatjaf/go-lnurl"
	"github.com/lucsky/cuid"
)

func addFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1000000)
	err := r.ParseMultipartForm(1000000)
	if err != nil {
		w.WriteHeader(400)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(400)
		return
	}

	session, err := store.Get(r, "auth")
	if err != nil {
		w.WriteHeader(500)
		return
	}

	name := r.FormValue("name")
	user, _ := session.Values["user"].(string)
	price, _ := strconv.Atoi(r.FormValue("price"))
	description := r.FormValue("description")
	fileId := cuid.Slug()

	if name == "" || price <= 0 || user == "" {
		w.WriteHeader(400)
		return
	}

	// load torrent file and turn it into a magnet
	metainfo, err := metainfo.Load(file)
	if err != nil {
		log.Warn().Err(err).Msg("can't parse metainfo file")
		w.WriteHeader(400)
		return
	}
	magnet := metainfo.Magnet(name, metainfo.HashInfoBytes())

	// store file
	metaj, _ := json.Marshal(Metadata{name, description})
	_, err = pg.Exec(`
INSERT INTO files (id, seller, price_msat, magnet, metadata)
VALUES ($1, $2, $3, $4, $5)
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
	json.NewEncoder(w).Encode(struct {
		HostMagnet string `json:"host_magnet"`
		URL        string `json:"url"`
		LNURL      string `json:"lnurl"`
	}{
		magnet.String(),
		s.ServiceURL + "/file/" + fileId,
		lnurlpay,
	})
}
