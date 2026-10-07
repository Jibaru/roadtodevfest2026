package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jibaru/s1ngo/internal/config"
	"github.com/jibaru/s1ngo/internal/handlers"
	"github.com/jibaru/s1ngo/internal/logger"
	"github.com/jibaru/s1ngo/internal/realtime"
	"github.com/jibaru/s1ngo/internal/server"
	"github.com/jibaru/s1ngo/internal/video/domain"
	"github.com/jibaru/s1ngo/internal/video/infra/agents"
	"github.com/jibaru/s1ngo/internal/video/infra/fake"
	"github.com/jibaru/s1ngo/internal/video/infra/persistence/memory"
	"github.com/jibaru/s1ngo/internal/video/infra/persistence/postgres"
	"github.com/jibaru/s1ngo/internal/video/infra/youtube"
	"github.com/jibaru/s1ngo/internal/video/service"
	"github.com/jibaru/s1ngo/web"
)

func main() {
	log := logger.New()

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()

	// Persistence: Postgres in production, memory for rehearsals.
	// Swapping storage is exactly this one decision.
	var repo domain.VideoRepository = memory.NewVideoRepository()
	if cfg.DatabaseURL != "" {
		if repo, err = postgres.NewVideoRepository(ctx, cfg.DatabaseURL); err != nil {
			log.Error("postgres", "error", err)
			os.Exit(1)
		}
		log.Info("using postgres repository")
	}

	// Infra adapters: real yt-dlp + OpenAI agents, or fakes for
	// offline rehearsal.
	var (
		lyricsAgents service.LyricsAgents
		subtitles    service.SubtitleSource
	)
	if cfg.FakeAgents {
		log.Info("FAKE_AGENTS=1: offline processing with fake subtitle source + agents")
		lyricsAgents, subtitles = &fake.Agents{Delay: time.Second}, fake.SubtitleSource{}
	} else {
		openaiAgents, err := agents.NewOpenAIAgents(ctx, cfg.OpenAIModel, cfg.OpenAIAPIKey)
		if err != nil {
			log.Error("agents", "error", err)
			os.Exit(1)
		}
		log.Info("lyrics agents ready", "model", cfg.OpenAIModel)
		cookiesFile := ""
		if cfg.YtdlpCookies != "" {
			f, err := os.CreateTemp("", "s1ngo-cookies-*.txt")
			if err == nil {
				if _, err = f.WriteString(cfg.YtdlpCookies); err == nil {
					cookiesFile = f.Name()
					log.Info("yt-dlp cookies configured")
				}
				_ = f.Close()
			}
		}
		lyricsAgents = openaiAgents
		subtitles = &youtube.Fetcher{Bin: cfg.YtdlpPath, CookiesFile: cookiesFile, ExtraArgs: cfg.YtdlpExtraArgs}
	}

	hub := realtime.NewHub(log)
	processor := service.NewProcessingService(repo, subtitles, lyricsAgents, hub, cfg.Workers, log)

	h := handlers.New(processor, repo, hub, log)
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: server.New(h, web.FS, log),
	}

	go func() {
		log.Info("listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("bye")
}
