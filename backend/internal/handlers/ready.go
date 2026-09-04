package handlers

import (
	"net/http"
)

func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	status := a.agg.Ready(r.Context())
	code := http.StatusOK
	if !status.Ready {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, status)
}
