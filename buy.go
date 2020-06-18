package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/fiatjaf/go-lnurl"
	"github.com/fiatjaf/lnpay-go"
	"github.com/gorilla/mux"
)

func buyFile(w http.ResponseWriter, r *http.Request) {
	file_id := mux.Vars(r)["file"]
	qs := r.URL.Query()
	msatoshi, _ := strconv.ParseInt(qs.Get("amount"), 10, 64)

	var file File
	err := pg.Get(&file, `
SELECT id, seller, price_msat, metadata, magnet
FROM files
WHERE id = $1
    `, file_id)
	if err != nil {
		json.NewEncoder(w).Encode(lnurl.ErrorResponse("File doesn't exist."))
		return
	}

	magnet, err := metainfo.ParseMagnetURI(file.Magnet)
	if err != nil {
		json.NewEncoder(w).Encode(lnurl.ErrorResponse("Magnet is broken."))
		return
	}

	var m Metadata
	err = json.Unmarshal(file.Metadata, &m)
	if err != nil {
		json.NewEncoder(w).Encode(lnurl.ErrorResponse("Bug in the database record."))
		return
	}

	if msatoshi == 0 {
		// first lnurlpay call
		json.NewEncoder(w).Encode(lnurl.LNURLPayResponse1{
			Tag:             "payRequest",
			Callback:        s.ServiceURL + "/~/" + file_id,
			MaxSendable:     file.PriceMsat,
			MinSendable:     file.PriceMsat,
			EncodedMetadata: string(makeMetadata(file.Seller, m, magnet)),
		})
	} else {
		// second lnurlpay call
		lntx, err := lnpending.Invoice(lnpay.InvoiceParams{
			NumSatoshis:     file.PriceMsat * 1000,
			DescriptionHash: hex.EncodeToString(makeMetadata(file.Seller, m, magnet)),
			PassThru:        map[string]interface{}{"file_id": file.Id},
		})
		if err != nil {
			json.NewEncoder(w).Encode(lnurl.ErrorResponse("Failed to generate invoice."))
			return
		}

		json.NewEncoder(w).Encode(lnurl.LNURLPayResponse2{
			Routes:     make([][]lnurl.RouteInfo, 0),
			PR:         lntx.PaymentRequest,
			Disposable: lnurl.TRUE,
			SuccessAction: lnurl.Action(
				"Your torrent URL:",
				s.ServiceURL+"/~/"+lntx.ID[5:]+"/announce",
			),
		})
	}
}

func buyFileStream(w http.ResponseWriter, r *http.Request) {

}

func makeMetadata(seller string, m Metadata, magnet metainfo.Magnet) []byte {
	j, _ := json.Marshal([][]string{
		{"text/plain", fmt.Sprintf("File '%s' from user %s", m.Name, seller)},
		{"text/vnd.filemarket.name", m.Name},
		{"text/vnd.filemarket.seller", seller},
		{"text/vnd.filemarket.description", m.Description},
		{"application/x-magnet-infohash", hex.EncodeToString(magnet.InfoHash[:])},
	})
	return j
}
