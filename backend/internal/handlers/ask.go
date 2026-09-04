package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const maxAskBodyBytes = 64 * 1024

func (a *API) ask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var body struct {
		Question string `json:"question"`
		Refresh  bool   `json:"refresh"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAskBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	question := strings.TrimSpace(body.Question)
	if question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question is required"})
		return
	}
	force := body.Refresh
	data, err := a.agg.Ask(r.Context(), question, force)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (a *API) accountCosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := strings.TrimSpace(r.URL.Query().Get("account"))
	if account == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account is required"})
		return
	}
	refresh := strings.TrimSpace(r.URL.Query().Get("refresh"))
	force := refresh != "" && refresh != "0" && refresh != "false"
	data, err := a.agg.AccountCosts(ctx, account, force)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}
