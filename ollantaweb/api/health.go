package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
	"github.com/scovl/ollanta/ollantaweb/ingest"
)

// healthDeps are satisfied by the dependencies available in main.
type healthDeps struct {
	db      *postgres.DB
	indexer *search.MeilisearchIndexer
	queue   *ingest.IngestQueue
}

var deps *healthDeps

// SetHealthDeps wires the dependencies used by the health handlers.
func SetHealthDeps(db *postgres.DB, indexer *search.MeilisearchIndexer, queue *ingest.IngestQueue) {
	deps = &healthDeps{db: db, indexer: indexer, queue: queue}
}

// Liveness handles GET /healthz — always 200 while the process is alive.
func Liveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// checkResult holds the outcome of a single component check.
type checkResult struct {
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Readiness handles GET /readyz.
// Returns 200 "ready" when all components are up.
// Returns 200 "degraded" when postgres is up but meilisearch is down.
// Returns 503 "not_ready" when postgres is down.
func Readiness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	checks := map[string]checkResult{}
	pgOK := false
	msOK := false

	if deps != nil {
		// ── postgres ──────────────────────────────────────────────────────
		pgStart := time.Now()
		if err := deps.db.Health(ctx); err != nil {
			checks["postgres"] = checkResult{Status: "error", Error: err.Error()}
		} else {
			pgOK = true
			checks["postgres"] = checkResult{Status: "ok", Latency: time.Since(pgStart).String()}
		}

		// ── meilisearch ───────────────────────────────────────────────────
		msStart := time.Now()
		msCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := deps.indexer.Health(msCtx); err != nil {
			checks["meilisearch"] = checkResult{Status: "error", Error: err.Error()}
		} else {
			msOK = true
			checks["meilisearch"] = checkResult{Status: "ok", Latency: time.Since(msStart).String()}
		}

		// ── ingest queue ──────────────────────────────────────────────────
		if deps.queue != nil {
			checks["ingest_queue"] = checkResult{
				Status:  "ok",
				Latency: "0s",
			}
		}
	}

	// Determine overall status and HTTP status code.
	overallStatus := "ready"
	httpStatus := http.StatusOK
	if !pgOK {
		overallStatus = "not_ready"
		httpStatus = http.StatusServiceUnavailable
	} else if !msOK {
		overallStatus = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": overallStatus,
		"checks": checks,
	})
}
