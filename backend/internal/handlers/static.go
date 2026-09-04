package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// withStaticSPA serves a built frontend from staticDir with SPA fallback.
func withStaticSPA(api http.Handler, staticDir string) http.Handler {
	fs := http.FileServer(http.Dir(staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/" {
			candidate := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
			if rel, err := filepath.Rel(staticDir, candidate); err == nil && !strings.HasPrefix(rel, "..") {
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					fs.ServeHTTP(w, r)
					return
				}
			}
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
}
