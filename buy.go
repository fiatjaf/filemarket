package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fiatjaf/go-lnurl"
	"github.com/fiatjaf/lnpay-go"
	"github.com/gorilla/mux"
	"gopkg.in/antage/eventsource.v1"
)

func buyFile(w http.ResponseWriter, r *http.Request) {
	file_id := mux.Vars(r)["file"]
	qs := r.URL.Query()
	msatoshi, _ := strconv.ParseInt(qs.Get("amount"), 10, 64)

	// optional, used to mix the lnurlpay flow with the client
	session := qs.Get("session")

	var file File
	err := pg.Get(&file, `
SELECT `+FILEFIELDS+`
FROM files
WHERE id = $1
    `, file_id)
	if err != nil {
		json.NewEncoder(w).Encode(lnurl.ErrorResponse("File doesn't exist."))
		return
	}

	if msatoshi == 0 {
		// first lnurlpay call
		if ies, ok := userstreams.Get(session); ok {
			ies.(eventsource.EventSource).SendEventMessage(
				"Got lnurlpay call for file "+file_id, "message", "")
		}

		json.NewEncoder(w).Encode(lnurl.LNURLPayResponse1{
			Callback:        s.ServiceURL + "/~/buy/" + file_id + "?session=" + session,
			Tag:             "payRequest",
			MaxSendable:     file.PriceMsat,
			MinSendable:     file.PriceMsat,
			EncodedMetadata: string(file.MakeMetadata()),
		})
	} else {
		// second lnurlpay call
		h := sha256.Sum256(file.MakeMetadata())

		lntx, err := lnpending.Invoice(lnpay.InvoiceParams{
			NumSatoshis:     file.PriceMsat / 1000,
			DescriptionHash: hex.EncodeToString(h[:]),
			PassThru: map[string]interface{}{
				"file_id": file.Id,
				"session": session,
			},
		})
		if err != nil {
			log.Error().Err(err).Msg("failed to generate lnpay invoice")
			json.NewEncoder(w).Encode(
				lnurl.ErrorResponse("Failed to generate invoice."))
			return
		}

		if ies, ok := userstreams.Get(session); ok {
			ies.(eventsource.EventSource).SendEventMessage(
				"Sending invoice for file "+file_id+" to wallet.", "message", "")
		}

		magstr, err := file.BuyerMagnet(lntx.ID[5:])
		if err != nil {
			json.NewEncoder(w).Encode(
				lnurl.ErrorResponse("Failed to compute magnet."))
			return
		}

		json.NewEncoder(w).Encode(lnurl.LNURLPayResponse2{
			Routes:        make([][]lnurl.RouteInfo, 0),
			PR:            lntx.PaymentRequest,
			Disposable:    lnurl.TRUE,
			SuccessAction: lnurl.Action(magstr, ""),
		})
	}
}
