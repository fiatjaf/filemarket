package main

import (
	"encoding/json"
	"net/http"

	"github.com/fiatjaf/lnpay-go"
)

func receivePaymentWebhook(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	defer r.Body.Close()

	var ev lnpay.WebhookWalletReceive
	err := json.NewDecoder(r.Body).Decode(&ev)
	if err != nil {
		log.Error().Err(err).Msg("error decoding receive webhook")
		return
	}

	if lnpendingId != ev.Data.Wtx.Wal.ID {
		log.Warn().Str("pending-wallet-id", lnpendingId).Interface("webhook", ev).
			Msg("got webhook for unexpected wallet")
		return
	}

	fileId_, _ := ev.Data.Wtx.PassThru["file_id"]
	fileId, _ := fileId_.(string)
	saleId := ev.Data.Wtx.LnTx.ID[5:]

	_, err = pg.Exec("INSERT INTO sales (id, file_id) VALUES ($1, $2)", saleId, fileId)
	if err != nil {
		log.Error().Err(err).Interface("webhook", ev).Msg("failed to save payment")
		return
	}
}
