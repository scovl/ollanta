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
	scanJobRepo := postgres.NewScanJobRepository(db)
	indexJobRepo := postgres.NewIndexJobRepository(db)
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
	webhookJobRepo := postgres.NewWebhookJobRepository(db)
	changelogRepo := postgres.NewChangelogRepository(db)

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
	api.SetHealthDeps(db, indexer, nil)

	// ── Telemetry ──────────────────────────────────────────────────────────
	metricsReg := telemetry.NewRegistry()
	appMetrics := telemetry.NewMetrics(metricsReg)
	_ = appMetrics // wired into pipeline below
	webhookDispatcher := webhook.NewDispatcher(webhookRepo, webhookJobRepo, "ollantaweb")

	// ── HTTP server ────────────────────────────────────────────────────────
	router := api.NewRouter(&api.RouterDeps{
		Config:      cfg,
		Projects:    projectRepo,
		Scans:       scanRepo,
		ScanJobs:    scanJobRepo,
		IndexJobs:   indexJobRepo,
		Issues:      issueRepo,
		Measures:    measureRepo,
		Users:       userRepo,
		Groups:      groupRepo,
		Tokens:      tokenRepo,
		Sessions:    sessionRepo,
		Perms:       permRepo,
		Searcher:    searcher,
		Indexer:     indexer,
		Profiles:    profileRepo,
		Gates:       gateRepo,
		Periods:     periodRepo,
		Webhooks:    webhookRepo,
		WebhookJobs: webhookJobRepo,
		Dispatcher:  webhookDispatcher,
		MetricsReg:  metricsReg,
		Changelog:   changelogRepo,
	})

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

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("ollantaweb: graceful shutdown: %v", err)
	}
	log.Println("ollantaweb: stopped")
}
