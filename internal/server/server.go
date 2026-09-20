package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jibaru/s1ngo/internal/handlers"
)

// New builds the router: JSON API + WebSocket + the embedded React SPA
// (any unknown GET path falls back to index.html for client routing).
func New(h *handlers.Handlers, webFS fs.FS, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /api/videos", h.ListVideos)
	mux.HandleFunc("POST /api/videos", h.CreateVideo)
	mux.HandleFunc("GET /api/videos/{id}", h.GetVideo)
	mux.HandleFunc("DELETE /api/videos/{id}", h.DeleteVideo)

	// Realtime
	mux.HandleFunc("GET /ws", h.WS)

	// Embedded SPA with client-side routing fallback.
	fileServer := http.FileServerFS(webFS)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := webFS.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		data, err := fs.ReadFile(webFS, "index.html")
		if err != nil {
			http.Error(w, "UI not built", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

	return chain(mux, recovery(log), requestID(), logging(log))
}
