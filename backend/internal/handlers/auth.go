package handlers

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

// APIClient is a named bearer token (one per integration).
type APIClient struct {
	Name  string
	Token string
}

func isPublicPath(path string) bool {
	for _, prefix := range []string{APIVersionPrefix, LegacyPrefix} {
		switch strings.TrimPrefix(path, prefix) {
		case "/health", "/ready", "/meta":
			if strings.HasPrefix(path, prefix+"/") {
				return true
			}
		}
	}
	return false
}

// withBearerAuth protects the API with a single token (client "default").
func withBearerAuth(token string, next http.Handler) http.Handler {
	if token == "" {
		return withClientAuth(nil, next)
	}
	return withClientAuth([]APIClient{{Name: "default", Token: token}}, next)
}

// matchClient returns the client whose token equals got. Every token is
// compared in constant time so timing doesn't reveal which one matched.
func matchClient(clients []APIClient, got string) (string, bool) {
	name, ok := "", false
	for _, c := range clients {
		if subtle.ConstantTimeCompare([]byte(got), []byte(c.Token)) == 1 {
			name, ok = c.Name, true
		}
	}
	return name, ok
}

// withClientAuth authenticates requests against named client tokens (when
// any are configured), tags the request context with the client name, and
// writes one access-log line per API request so spend lookups can be
// attributed to the integration that made them.
func withClientAuth(clients []APIClient, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := service.AnonymousClient
		if len(clients) > 0 && r.Method != http.MethodOptions && !isPublicPath(r.URL.Path) {
			auth := strings.TrimSpace(r.Header.Get("Authorization"))
			const prefix = "Bearer "
			name, ok := "", false
			if strings.HasPrefix(auth, prefix) {
				name, ok = matchClient(clients, strings.TrimSpace(auth[len(prefix):]))
			}
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			client = name
		}

		ctx, info := service.WithRefreshInfo(service.WithClient(r.Context(), client))
		rw := &statusWriter{ResponseWriter: w, info: info}
		start := time.Now()
		next.ServeHTTP(rw, r.WithContext(ctx))
		if r.Method != http.MethodOptions && !isPublicPath(r.URL.Path) {
			log.Printf("api_request client=%s method=%s path=%s status=%d duration_ms=%d refresh_coalesced=%v",
				client, r.Method, r.URL.Path, rw.status(), time.Since(start).Milliseconds(), info.Coalesced)
		}
	})
}

// statusWriter records the status code and adds X-Refresh-Coalesced when a
// refresh=1 request was served data another client had just refreshed.
type statusWriter struct {
	http.ResponseWriter
	info *service.RefreshInfo
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
		if w.info != nil && w.info.Coalesced {
			w.Header().Set("X-Refresh-Coalesced", "true")
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) status() int {
	if w.code == 0 {
		return http.StatusOK
	}
	return w.code
}
