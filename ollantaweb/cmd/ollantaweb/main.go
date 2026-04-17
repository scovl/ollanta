// Command ollantaweb is the centralized scan-collection server.
// It exposes a REST API for receiving scan reports from ollantascanner,
// persisting them to PostgreSQL, and indexing them in Meilisearch.
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

	// ── Meilisearch ────────────────────────────────────────────────────────
	indexerCfg := search.IndexerConfig{
		Host:   cfg.MeilisearchURL,
		APIKey: cfg.MeilisearchAPIKey,
	}
	indexer, err := search.NewMeilisearchIndexer(indexerCfg)
	if err != nil {
		log.Fatalf("ollantaweb: create indexer: %v", err)
	}
	if err := indexer.ConfigureIndexes(ctx); err != nil {
		log.Printf("ollantaweb: meilisearch configure: %v (continuing)", err)
	}
	searcher, err := search.NewMeilisearchSearcher(indexerCfg)
	if err != nil {
		log.Fatalf("ollantaweb: create searcher: %v", err)
	}

	// ── Health deps ────────────────────────────────────────────────────────
	api.SetHealthDeps(db, indexer)

	// ── Background worker ─────────────────────────────────────────────────
	worker := ingest.NewWorker(indexer, issueRepo, 256)
	go worker.Start(ctx)

	// ── Ingest pipeline ────────────────────────────────────────────────────
	pipeline := ingest.NewPipeline(db, projectRepo, scanRepo, issueRepo, measureRepo, indexer, worker)

	// ── HTTP server ────────────────────────────────────────────────────────
	router := api.NewRouter(
		projectRepo, scanRepo, issueRepo, measureRepo,
		searcher, indexer, projectRepo, pipeline,
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

	worker.Stop()

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("ollantaweb: graceful shutdown: %v", err)
	}
	log.Println("ollantaweb: stopped")
}
