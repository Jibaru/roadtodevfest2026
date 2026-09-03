package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	Port           string
	GeminiAPIKey   string
	PresenterToken string
	// SnapshotDir enables the file snapshot repository when non-empty (local rehearsals).
	SnapshotDir string
	// FakeAgents runs the show with canned verses instead of calling Gemini.
	FakeAgents bool
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		GeminiAPIKey:   os.Getenv("GEMINI_API_KEY"),
		PresenterToken: os.Getenv("PRESENTER_TOKEN"),
		SnapshotDir:    os.Getenv("SNAPSHOT_DIR"),
		FakeAgents:     os.Getenv("FAKE_AGENTS") == "1",
	}
	if cfg.PresenterToken == "" {
		return nil, fmt.Errorf("PRESENTER_TOKEN is required (stage access control)")
	}
	if cfg.GeminiAPIKey == "" && !cfg.FakeAgents {
		return nil, fmt.Errorf("GEMINI_API_KEY is required unless FAKE_AGENTS=1")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
