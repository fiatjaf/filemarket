package main

import "github.com/jmoiron/sqlx/types"

type File struct {
	Id        string         `db:"id" json:"id"`
	Seller    string         `db:"seller" json:"seller"`
	PriceMsat int64          `db:"price_msat" json:"price_msat"`
	Magnet    string         `db:"magnet" json:"magnet"`
	Metadata  types.JSONText `db:"metadata" json:"metadata"`
	NSales    int            `db:"nsales" json:"nsales"`
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
