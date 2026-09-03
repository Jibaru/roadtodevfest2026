package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jibaru/rapbattle/internal/agents"
	"github.com/jibaru/rapbattle/internal/agents/fake"
	"github.com/jibaru/rapbattle/internal/battle/domain"
	filerepo "github.com/jibaru/rapbattle/internal/battle/infra/persistence/file"
	"github.com/jibaru/rapbattle/internal/battle/infra/persistence/embedded"
	"github.com/jibaru/rapbattle/internal/battle/infra/persistence/memory"
	"github.com/jibaru/rapbattle/internal/battle/service"
	"github.com/jibaru/rapbattle/internal/config"
	"github.com/jibaru/rapbattle/internal/handlers"
	"github.com/jibaru/rapbattle/internal/logger"
	"github.com/jibaru/rapbattle/internal/realtime"
	"github.com/jibaru/rapbattle/internal/server"
	"github.com/jibaru/rapbattle/internal/tts"
	"github.com/jibaru/rapbattle/web"
)

func main() {
	log := logger.New()

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()

	// Persistence: memory for the show, file snapshots for rehearsals.
	// Swapping storage is exactly this one decision.
	var repo domain.BattleRepository = memory.NewBattleRepository()
	if cfg.SnapshotDir != "" {
		if repo, err = filerepo.NewBattleRepository(cfg.SnapshotDir); err != nil {
			log.Error("file repo", "error", err)
			os.Exit(1)
		}
		log.Info("using file snapshot repository", "dir", cfg.SnapshotDir)
	}

	cache, err := embedded.NewVerseCache()
	if err != nil {
		log.Error("verse cache", "error", err)
		os.Exit(1)
	}

	// The cast: real Gemini-powered crew, or fakes for offline rehearsal.
	var (
		writer    service.VerseWriter
		judge     service.Judge
		performer service.Performer
	)
	if cfg.FakeAgents {
		log.Info("FAKE_AGENTS=1: running the show offline with canned verses")
		crew := &fake.Crew{Cache: cache, Delay: 3 * time.Second}
		writer, judge, performer = crew, crew, fake.SilentPerformer{}
	} else {
		crew, err := agents.NewCrew(ctx, cfg.GeminiAPIKey)
		if err != nil {
			log.Error("agents", "error", err)
			os.Exit(1)
		}
		crew.CrowdWords = func(n int) []string {
			b, err := repo.Current(context.Background())
			if err != nil {
				return nil
			}
			return b.RecentCrowdWords(n)
		}
		ttsClient, err := tts.NewClient(ctx, cfg.GeminiAPIKey)
		if err != nil {
			log.Error("tts", "error", err)
			os.Exit(1)
		}
		writer, judge, performer = crew, crew, ttsClient
	}

	hub := realtime.NewHub(log)
	svc := service.NewBattleService(repo, cache, writer, judge, performer, hub, log)
	hub.SetGame(svc)

	h := handlers.New(svc, hub, cfg.PresenterToken, log)
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
