package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
)

// healthDeps are satisfied by the dependencies available in main.
type healthDeps struct {
	db      *postgres.DB
	indexer *search.MeilisearchIndexer
}

var deps *healthDeps

// SetHealthDeps wires the dependencies used by the health handlers.
func SetHealthDeps(db *postgres.DB, indexer *search.MeilisearchIndexer) {
	deps = &healthDeps{db: db, indexer: indexer}
}

// Liveness handles GET /healthz — always 200 while the process is alive.
func Liveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// Readiness handles GET /readyz — 200 when PG + Meilisearch are reachable, 503 otherwise.
func Readiness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	checks := map[string]string{}
	status := http.StatusOK

	if deps != nil {
		if err := deps.db.Health(ctx); err != nil {
			checks["postgres"] = err.Error()
			status = http.StatusServiceUnavailable
		} else {
			checks["postgres"] = "ok"
		}

		if err := deps.indexer.Health(context.Background()); err != nil {
			checks["meilisearch"] = err.Error()
			status = http.StatusServiceUnavailable
		} else {
			checks["meilisearch"] = "ok"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"checks": checks})
}
