package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	Port         string
	GeminiAPIKey string
	// DatabaseURL enables the Postgres repository when non-empty;
	// otherwise videos live in memory.
	DatabaseURL string
	// Workers is the size of the processing pool.
	Workers int
	// YtdlpPath overrides the yt-dlp binary location (default: PATH).
	YtdlpPath string
	// FakeAgents runs the whole pipeline offline: fake fetcher + fake agents.
	FakeAgents bool
}

func Load() (*Config, error) {
	workers, _ := strconv.Atoi(getEnv("WORKERS", "3"))
	cfg := &Config{
		Port:         getEnv("PORT", "8080"),
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		Workers:      workers,
		YtdlpPath:    os.Getenv("YTDLP_PATH"),
		FakeAgents:   os.Getenv("FAKE_AGENTS") == "1",
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
