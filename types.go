package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/jmoiron/sqlx/types"
)

type File struct {
	Id        string         `db:"id" json:"id"`
	Seller    string         `db:"seller" json:"seller"`
	PriceMsat int64          `db:"price_msat" json:"price_msat"`
	Torrent   []byte         `db:"torrent" json:"torrent"`
	Metadata  types.JSONText `db:"metadata" json:"metadata"`
	NSales    int            `db:"nsales" json:"nsales"`
}

const FILEFIELDS = "id, seller, price_msat, torrent, metadata"

func (file File) MakeMetadata() []byte {
	var m Metadata
	json.Unmarshal(file.Metadata, &m)

	torrent, _ := metainfo.Load(bytes.NewBuffer(file.Torrent))

	j, _ := json.Marshal([][]string{
		{"text/plain", fmt.Sprintf("File '%s' (%s) from user %s.", m.Name, file.Id, file.Seller)},
		{"text/vnd.filemarket.name", m.Name},
		{"text/vnd.filemarket.seller", file.Seller},
		{"text/vnd.filemarket.description", m.Description},
		{"application/x-infohash", torrent.HashInfoBytes().HexString()},
	})

	return j
}

func (f File) HostTorrentFile() ([]byte, error) {
	torrent, err := metainfo.Load(bytes.NewBuffer(f.Torrent))
	if err != nil {
		return nil, err
	}

	key := f.Id + ":" + makeSellerHash(f.Seller)
	torrent.Announce = s.ServiceURL + "/~/" + key + "/announce"

	return bencode.Marshal(torrent)
}

func (f File) BuyerMagnet(saleId string) (string, error) {
	torrent, err := metainfo.Load(bytes.NewBuffer(f.Torrent))
	if err != nil {
		return "", err
	}

	var metadata Metadata
	err = f.Metadata.Unmarshal(&metadata)
	if err != nil {
		return "", err
	}

	magnet := torrent.Magnet(metadata.Name, torrent.HashInfoBytes())
	magnet.Trackers = append(magnet.Trackers, s.ServiceURL+"/~/"+saleId+"/announce")

	return magnet.String(), nil
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
