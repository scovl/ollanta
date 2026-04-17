package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
	"github.com/scovl/ollanta/ollantaweb/ingest"
)

// NewRouter builds and returns the complete chi router for the ollantaweb server.
func NewRouter(
	projects *postgres.ProjectRepository,
	scans *postgres.ScanRepository,
	issues *postgres.IssueRepository,
	measures *postgres.MeasureRepository,
	searcher *search.MeilisearchSearcher,
	indexer *search.MeilisearchIndexer,
	projectRepo *postgres.ProjectRepository,
	pipeline *ingest.Pipeline,
) http.Handler {
	r := chi.NewRouter()

	// ── Global middleware ──────────────────────────────────────────────────
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(RequestLogger)
	r.Use(CORS)
	r.Use(MaxBody(10 << 20)) // 10 MB

	// ── Health ────────────────────────────────────────────────────────────
	r.Get("/healthz", Liveness)
	r.Get("/readyz", Readiness)

	// ── API v1 ────────────────────────────────────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {
		// Projects
		ph := &ProjectsHandler{repo: projects}
		r.Post("/projects", ph.Create)
		r.Get("/projects", ph.List)
		r.Get("/projects/{key}", ph.Get)

		// Scans
		sh := &ScansHandler{
			scans:    scans,
			projects: projects,
			pipeline: pipeline,
		}
		r.Post("/scans", sh.Ingest)
		r.Get("/scans/{id}", sh.Get)
		r.Get("/projects/{key}/scans", sh.ListByProject)
		r.Get("/projects/{key}/scans/latest", sh.Latest)

		// Issues
		ih := &IssuesHandler{issues: issues, projects: projects}
		r.Get("/issues", ih.List)
		r.Get("/issues/facets", ih.Facets)

		// Measures
		mh := &MeasuresHandler{measures: measures, projects: projects}
		r.Get("/projects/{key}/measures/trend", mh.Trend)

		// Search
		srh := &SearchHandler{searcher: searcher}
		r.Get("/search", srh.Search)
	})

	// ── Admin ─────────────────────────────────────────────────────────────
	r.Post("/admin/reindex", func(w http.ResponseWriter, r *http.Request) {
		go func() {
			ctx := r.Context()
			if err := indexer.ReindexAll(ctx, issues, projectRepo); err != nil {
				// Non-fatal; logged by the indexer
				_ = err
			}
		}()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"reindex started"}`))
	})

	return r
}
