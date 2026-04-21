package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/config"
	"github.com/scovl/ollanta/ollantaweb/ingest"
	"github.com/scovl/ollanta/ollantaweb/webhook"
)

func main() {
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("ollantaworker: connect postgres: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("ollantaworker: migrate: %v", err)
	}

	projectRepo := postgres.NewProjectRepository(db)
	scanRepo := postgres.NewScanRepository(db)
	scanJobRepo := postgres.NewScanJobRepository(db)
	indexJobRepo := postgres.NewIndexJobRepository(db)
	issueRepo := postgres.NewIssueRepository(db)
	measureRepo := postgres.NewMeasureRepository(db)
	webhookRepo := postgres.NewWebhookRepository(db)
	webhookJobRepo := postgres.NewWebhookJobRepository(db)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "ollantaworker"
	}
	workerID := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	indexEnqueuer := ingest.NewIndexJobEnqueuer(indexJobRepo)
	webhookDispatcher := webhook.NewDispatcher(webhookRepo, webhookJobRepo, workerID)

	processor := ingest.NewScanJobProcessor(
		workerID,
		scanJobRepo,
		projectRepo,
		scanRepo,
		issueRepo,
		measureRepo,
		indexEnqueuer,
		webhookDispatcher,
	)
	jobWorker := ingest.NewScanJobWorker(processor, time.Second)

	log.Printf("ollantaworker: started as %s", workerID)
	jobWorker.Start(ctx)

	webhookDispatcher.Stop()
	log.Println("ollantaworker: stopped")
}
