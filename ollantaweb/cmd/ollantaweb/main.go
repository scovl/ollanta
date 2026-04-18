// Command ollantaweb is the centralized scan-collection server.
// It exposes a REST API for receiving scan reports from ollantascanner,
// persisting them to PostgreSQL, and indexing them in ZincSearch.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
	"github.com/scovl/ollanta/ollantaweb/api"
	"github.com/scovl/ollanta/ollantaweb/config"
	"github.com/scovl/ollanta/ollantaweb/ingest"
	"github.com/scovl/ollanta/ollantaweb/pgnotify"
	"github.com/scovl/ollanta/ollantaweb/telemetry"
	"github.com/scovl/ollanta/ollantaweb/webhook"
)

func main() {
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── PostgreSQL ─────────────────────────────────────────────────────────
	db, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("ollantaweb: connect postgres: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("ollantaweb: migrate: %v", err)
	}
	log.Println("ollantaweb: database migrations applied")

	// ── Repositories ───────────────────────────────────────────────────────
	projectRepo := postgres.NewProjectRepository(db)
	scanRepo := postgres.NewScanRepository(db)
	issueRepo := postgres.NewIssueRepository(db)
	measureRepo := postgres.NewMeasureRepository(db)
	userRepo := postgres.NewUserRepository(db)
	groupRepo := postgres.NewGroupRepository(db)
	tokenRepo := postgres.NewTokenRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	permRepo := postgres.NewPermissionRepository(db)
	profileRepo := postgres.NewProfileRepository(db)
	gateRepo := postgres.NewGateRepository(db)
	periodRepo := postgres.NewNewCodePeriodRepository(db)
	webhookRepo := postgres.NewWebhookRepository(db)

	// ── Search backend ─────────────────────────────────────────────────────
	zincCfg := search.ZincConfig{
		Host:     cfg.ZincSearchURL,
		User:     cfg.ZincSearchUser,
		Password: cfg.ZincSearchPassword,
	}
	searcher, indexer, err := search.NewBackend(cfg.SearchBackend, zincCfg, db.Pool)
	if err != nil {
		log.Fatalf("ollantaweb: create search backend: %v", err)
	}
	if err := indexer.ConfigureIndexes(ctx); err != nil {
		log.Printf("ollantaweb: search configure: %v (continuing)", err)
	}

	// ── Health deps ────────────────────────────────────────────────────────
	ingestQueue := ingest.NewIngestQueue(100)
	api.SetHealthDeps(db, indexer, ingestQueue)

	// ── Telemetry ──────────────────────────────────────────────────────────
	metricsReg := telemetry.NewRegistry()
	appMetrics := telemetry.NewMetrics(metricsReg)
	_ = appMetrics // wired into pipeline below

	// ── Webhook dispatcher ─────────────────────────────────────────────────
	wdispatcher := webhook.NewDispatcher(webhookRepo, 256)
	go wdispatcher.Start(ctx)

	// ── Index coordinator ──────────────────────────────────────────────────
	var enqueuer ingest.IndexEnqueuer
	switch cfg.IndexCoordinator {
	case "pgnotify":
		coord := pgnotify.NewCoordinator(db.Pool, indexer, issueRepo)
		if err := coord.EnsureTable(ctx); err != nil {
			log.Fatalf("ollantaweb: pgnotify ensure table: %v", err)
		}
		go coord.Start(ctx)
		enqueuer = coord
	default: // "memory"
		worker := ingest.NewWorker(indexer, issueRepo, 256)
		go worker.Start(ctx)
		enqueuer = worker
	}

	// ── Ingest pipeline ────────────────────────────────────────────────────
	pipeline := ingest.NewPipeline(db, projectRepo, scanRepo, issueRepo, measureRepo, indexer, enqueuer)

	// ── HTTP server ────────────────────────────────────────────────────────
	router := api.NewRouter(
		cfg,
		projectRepo, scanRepo, issueRepo, measureRepo,
		userRepo, groupRepo, tokenRepo, sessionRepo, permRepo,
		searcher, indexer, pipeline,
		profileRepo, gateRepo, periodRepo, webhookRepo, wdispatcher,
		metricsReg,
	)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("ollantaweb: listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("ollantaweb: listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("ollantaweb: shutting down...")

	if stopper, ok := enqueuer.(interface{ Stop() }); ok {
		stopper.Stop()
	}
	wdispatcher.Stop()

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("ollantaweb: graceful shutdown: %v", err)
	}
	log.Println("ollantaweb: stopped")
}
