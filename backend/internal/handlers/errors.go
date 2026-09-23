package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

func writeAPIError(w http.ResponseWriter, err error) {
	var invalid *service.InvalidRequestError
	if errors.As(err, &invalid) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": invalid.Message})
		return
	}
	var rateLimited *service.RateLimitedError
	if errors.As(err, &rateLimited) {
		secs := int(rateLimited.RetryAfter.Seconds())
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": rateLimited.Error()})
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
