package server

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/jibaru/rapbattle/internal/handlers"
)

// New builds the router with all routes and middleware.
func New(h *handlers.Handlers, webFS fs.FS, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /api/battles", h.StartBattle)
	mux.HandleFunc("POST /api/battles/current/advance", h.Advance)
	mux.HandleFunc("POST /api/battles/current/reset", h.Reset)
	mux.HandleFunc("GET /api/battles/current", h.CurrentBattle)

	// Realtime
	mux.HandleFunc("GET /ws", h.AudienceWS)
	mux.HandleFunc("GET /ws/stage", h.StageWS)

	// Embedded UI
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(webFS)))
	mux.HandleFunc("GET /{$}", servePage(webFS, "audience.html"))
	mux.HandleFunc("GET /stage", servePage(webFS, "stage.html"))

	return chain(mux, recovery(log), requestID(), logging(log))
}

func servePage(webFS fs.FS, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(webFS, name)
		if err != nil {
			http.Error(w, "page not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}
}
