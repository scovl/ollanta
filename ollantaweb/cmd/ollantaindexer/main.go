package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
	"github.com/scovl/ollanta/ollantaweb/config"
	"github.com/scovl/ollanta/ollantaweb/ingest"
)

func main() {
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("ollantaindexer: connect postgres: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("ollantaindexer: migrate: %v", err)
	}

	indexJobRepo := postgres.NewIndexJobRepository(db)
	issueRepo := postgres.NewIssueRepository(db)

	zincCfg := search.ZincConfig{
		Host:     cfg.ZincSearchURL,
		User:     cfg.ZincSearchUser,
		Password: cfg.ZincSearchPassword,
	}
	_, indexer, err := search.NewBackend(cfg.SearchBackend, zincCfg, db.Pool)
	if err != nil {
		log.Fatalf("ollantaindexer: create search backend: %v", err)
	}
	if err := indexer.ConfigureIndexes(ctx); err != nil {
		log.Printf("ollantaindexer: search configure: %v (continuing)", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "ollantaindexer"
	}
	workerID := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	worker := ingest.NewWorker(indexer, issueRepo, indexJobRepo, workerID)
	log.Printf("ollantaindexer: started as %s", workerID)
	worker.Start(ctx)
	log.Println("ollantaindexer: stopped")
}
