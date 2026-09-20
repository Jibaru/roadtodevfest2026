package config

import (
	"encoding/base64"
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
	// YtdlpCookies is the raw contents of a Netscape cookies.txt; when
	// set it is written to a temp file and passed to yt-dlp — the
	// escape hatch for YouTube's datacenter-IP bot checks.
	YtdlpCookies string
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
		YtdlpCookies: loadCookies(),
		FakeAgents:   os.Getenv("FAKE_AGENTS") == "1",
	}
	if cfg.GeminiAPIKey == "" && !cfg.FakeAgents {
		return nil, fmt.Errorf("GEMINI_API_KEY is required unless FAKE_AGENTS=1")
	}
	return cfg, nil
}

// loadCookies accepts the cookies.txt either raw (YTDLP_COOKIES) or
// base64-encoded (YTDLP_COOKIES_B64). Base64 is the practical option:
// cookies.txt is multiline with tabs, which dotenv-style env blocks
// (like Dokploy's) cannot carry.
func loadCookies() string {
	if b64 := os.Getenv("YTDLP_COOKIES_B64"); b64 != "" {
		if raw, err := base64.StdEncoding.DecodeString(b64); err == nil {
			return string(raw)
		}
	}
	return os.Getenv("YTDLP_COOKIES")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
