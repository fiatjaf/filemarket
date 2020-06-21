package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/jmoiron/sqlx/types"
)

type File struct {
	Id        string         `db:"id" json:"id"`
	Seller    string         `db:"seller" json:"seller"`
	PriceMsat int64          `db:"price_msat" json:"price_msat"`
	Magnet    string         `db:"magnet" json:"magnet"`
	Metadata  types.JSONText `db:"metadata" json:"metadata"`
	NSales    int            `db:"nsales" json:"nsales"`
}

const FILEFIELDS = "id, seller, price_msat, magnet, metadata"

func (file File) MakeMetadata() []byte {
	var m Metadata
	json.Unmarshal(file.Metadata, &m)

	magnet, _ := metainfo.ParseMagnetURI(file.Magnet)

	j, _ := json.Marshal([][]string{
		{"text/plain", fmt.Sprintf("File '%s' (%s) from user %s.", m.Name, file.Id, file.Seller)},
		{"text/vnd.filemarket.name", m.Name},
		{"text/vnd.filemarket.seller", file.Seller},
		{"text/vnd.filemarket.description", m.Description},
		{"application/x-magnet-infohash", hex.EncodeToString(magnet.InfoHash[:])},
	})

	return j
}

func (f File) HostMagnet() (string, error) {
	m, err := metainfo.ParseMagnetURI(f.Magnet)
	if err != nil {
		return "", err
	}

	key := f.Id + ":" + makeSellerHash(f.Seller)

	m.Trackers = append(m.Trackers, s.ServiceURL+"/~/"+key+"/announce")
	return m.String(), nil
}

func (f File) BuyerMagnet(saleId string) (string, error) {
	m, err := metainfo.ParseMagnetURI(f.Magnet)
	if err != nil {
		return "", err
	}
	m.Trackers = append(m.Trackers, s.ServiceURL+"/~/"+f.Id+"/announce")
	return m.String(), nil
}

type Sale struct {
	Id     string `db:"id"`
	FileId string `db:"file_id"`
	Status string `db:"status"`
}

type Metadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
