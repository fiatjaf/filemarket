package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

func listFiles(w http.ResponseWriter, r *http.Request) {
	filter := ""
	args := []interface{}{}

	seller := r.URL.Query().Get("seller")
	if seller != "" {
		filter = "WHERE seller = $1"
		args = append(args, seller)
	}

	var files []File
	err := pg.Select(&files, `
SELECT id, seller, price_msat, metadata, magnet, nsales
FROM files
INNER JOIN (
  SELECT file_id, count(*) AS nsales
  FROM sales
  GROUP BY file_id
)ssum ON file_id = files.id
`+filter+`
ORDER BY nsales DESC
    `, args...)
	if err != nil || err != sql.ErrNoRows {
		log.Error().Err(err).Msg("database error when listing files")
		return
	}

	if len(files) == 0 {
		files = make([]File, 0)
	}

	json.NewEncoder(w).Encode(files)
}
