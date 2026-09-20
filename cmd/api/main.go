package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jibaru/s1ngo/internal/agents"
	"github.com/jibaru/s1ngo/internal/agents/fake"
	"github.com/jibaru/s1ngo/internal/config"
	"github.com/jibaru/s1ngo/internal/handlers"
	"github.com/jibaru/s1ngo/internal/logger"
	"github.com/jibaru/s1ngo/internal/pipeline"
	"github.com/jibaru/s1ngo/internal/realtime"
	"github.com/jibaru/s1ngo/internal/server"
	"github.com/jibaru/s1ngo/internal/video/domain"
	"github.com/jibaru/s1ngo/internal/video/infra/persistence/memory"
	"github.com/jibaru/s1ngo/internal/video/infra/persistence/postgres"
	"github.com/jibaru/s1ngo/internal/ytdlp"
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

	// The cast: real yt-dlp + Gemini agents, or fakes for offline rehearsal.
	var (
		crew    pipeline.Agents
		fetcher pipeline.Fetcher
	)
	if cfg.FakeAgents {
		log.Info("FAKE_AGENTS=1: offline pipeline with fake fetcher + agents")
		crew, fetcher = &fake.Crew{Delay: time.Second}, fake.Fetcher{}
	} else {
		realCrew, err := agents.NewCrew(ctx, cfg.GeminiAPIKey)
		if err != nil {
			log.Error("agents", "error", err)
			os.Exit(1)
		}
		crew, fetcher = realCrew, &ytdlp.Fetcher{Bin: cfg.YtdlpPath}
	}

	hub := realtime.NewHub(log)
	pipe := pipeline.New(repo, fetcher, crew, hub, cfg.Workers, log)

	h := handlers.New(pipe, repo, hub, log)
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
