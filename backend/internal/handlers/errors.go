package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

func writeAPIError(w http.ResponseWriter, err error) {
	var invalid *service.InvalidRequestError
	if errors.As(err, &invalid) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": invalid.Message})
		return
	}
	var unknown *service.UnknownAccountError
	if errors.As(err, &unknown) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown account"})
		return
	}
	log.Printf("api error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
