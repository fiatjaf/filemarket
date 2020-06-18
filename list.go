package main

import (
	"encoding/json"
	"net/http"
)

func listFiles(w http.ResponseWriter, r *http.Request) {
	var files []File
	err := pg.Select(&files, `
SELECT id, seller, price_msat, metadata, magnet, nsales
FROM files
INNER JOIN (
  SELECT file_id, sum(*) AS nsales
  FROM sales
  GROUP BY file_id
)ssum ON file_id = files.id
ORDER BY nsales DESC
    `)
	if err != nil {
		log.Error().Err(err).Msg("database error when listing files")
		return
	}

	json.NewEncoder(w).Encode(files)
}
