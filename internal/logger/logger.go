package logger

import (
	"log/slog"
	"os"
)

// New returns a structured JSON logger suitable for Cloud Run log ingestion.
func New() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
