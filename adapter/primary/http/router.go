package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/scovl/ollanta/adapter/secondary/postgres"
	"github.com/scovl/ollanta/adapter/secondary/search"
	"github.com/scovl/ollanta/adapter/config"
	"github.com/scovl/ollanta/application/ingest"
	"github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/adapter/secondary/webhook"
)

// RouterDeps groups all dependencies required by NewRouter.
type RouterDeps struct {
	Cfg        *config.Config
	Projects   *postgres.ProjectRepository
	Scans      *postgres.ScanRepository
	Issues     *postgres.IssueRepository
	Measures   *postgres.MeasureRepository
	Users      *postgres.UserRepository
	Groups     *postgres.GroupRepository
	Tokens     *postgres.TokenRepository
	Sessions   *postgres.SessionRepository
	Perms      *postgres.PermissionRepository
	Searcher   *search.MeilisearchSearcher
	Indexer    *search.MeilisearchIndexer
	Pipeline   *ingest.IngestUseCase
	Profiles   *postgres.ProfileRepository
	Gates      *postgres.GateRepository
	Periods    *postgres.NewCodePeriodRepository
	Webhooks   *postgres.WebhookRepository
	Dispatcher *webhook.Dispatcher
	Metrics    *telemetry.Registry
}

// NewRouter builds and returns the complete chi router for the ollantaweb server.
func NewRouter(d *RouterDeps) http.Handler {
	r := chi.NewRouter()

	// ── Global middleware ──────────────────────────────────────────────────
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(telemetry.TraceIDMiddleware)
	r.Use(RequestLogger)
	r.Use(CORS)
	r.Use(MaxBody(10 << 20)) // 10 MB

	// ── Health (always public) ─────────────────────────────────────────────
	r.Get("/healthz", Liveness)
	r.Get("/readyz", Readiness)
	r.Get("/metrics", d.Metrics.Handler())

	// ── Auth middleware ────────────────────────────────────────────────────
	authMW := NewAuthMiddleware(d.Users, d.Tokens, d.Sessions, []byte(d.Cfg.JWTSecret))

	// ── Handlers ──────────────────────────────────────────────────────────
	authH := NewAuthHandler(d.Cfg, d.Users, d.Groups, d.Sessions)
	usersH := NewUsersHandler(d.Users, d.Tokens)
	groupsH := NewGroupsHandler(d.Groups)
	tokensH := NewTokensHandler(d.Tokens, d.Projects, d.Perms)
	permsH := NewPermsHandler(d.Perms, d.Projects)
	profilesH := NewProfilesHandler(d.Profiles, d.Projects)
	gatesH := NewGatesHandler(d.Gates, d.Projects)
	periodsH := NewNewCodePeriodHandler(d.Periods, d.Projects)
	webhooksH := NewWebhooksHandler(d.Webhooks, d.Projects, d.Dispatcher)

	ph := &ProjectsHandler{repo: d.Projects}
	sh := &ScansHandler{scans: d.Scans, projects: d.Projects, pipeline: d.Pipeline}
	ih := &IssuesHandler{issues: d.Issues, projects: d.Projects}
	mh := &MeasuresHandler{measures: d.Measures, projects: d.Projects}
	srh := &SearchHandler{searcher: d.Searcher}

	// ── API v1 ────────────────────────────────────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {
		// Public: auth endpoints
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)
		r.Get("/auth/github", authH.GitHubRedirect)
		r.Get("/auth/github/callback", authH.GitHubCallback)
		r.Get("/auth/gitlab", authH.GitLabRedirect)
		r.Get("/auth/gitlab/callback", authH.GitLabCallback)
		r.Get("/auth/google", authH.GoogleRedirect)
		r.Get("/auth/google/callback", authH.GoogleCallback)

		// Protected: all other routes require a valid JWT or API token
		r.Group(func(r chi.Router) {
			r.Use(authMW.Authenticate)

			// Auth management
			r.Post("/auth/logout", authH.Logout)

			// Current user
			r.Get("/users/me", usersH.Me)

			// Own tokens (self-service)
			r.Get("/users/me/tokens", tokensH.List)
			r.Post("/users/me/tokens", tokensH.Create)
			r.Delete("/users/me/tokens/{id}", tokensH.Delete)

			// User management (requires manage_users)
			r.Route("/users", func(r chi.Router) {
				r.Use(RequirePermission(d.Perms, "manage_users"))
				r.Get("/", usersH.List)
				r.Post("/", usersH.Create)
				r.Get("/{id}", usersH.Get)
				r.Put("/{id}", usersH.Update)
				r.Delete("/{id}", usersH.Deactivate)
				r.Get("/{id}/tokens", usersH.ListTokens)
				r.Delete("/{id}/tokens/{tid}", usersH.DeleteToken)
			})

			// Group management (requires manage_groups)
			r.Route("/groups", func(r chi.Router) {
				r.Use(RequirePermission(d.Perms, "manage_groups"))
				r.Get("/", groupsH.List)
				r.Post("/", groupsH.Create)
				r.Put("/{id}", groupsH.Update)
				r.Delete("/{id}", groupsH.Delete)
				r.Get("/{id}/members", groupsH.ListMembers)
				r.Post("/{id}/members", groupsH.AddMember)
				r.Delete("/{id}/members/{uid}", groupsH.RemoveMember)
			})

			// Global permission management (requires admin)
			r.Route("/permissions", func(r chi.Router) {
				r.Use(RequirePermission(d.Perms, "admin"))
				r.Get("/global", permsH.ListGlobal)
				r.Post("/global/grant", permsH.GrantGlobal)
				r.Post("/global/revoke", permsH.RevokeGlobal)
			})

			// Projects
			r.Post("/projects", ph.Create)
			r.Get("/projects", ph.List)
			r.Get("/projects/{key}", ph.Get)

			// Project-level permissions (requires admin global or project admin)
			r.Get("/projects/{key}/permissions", permsH.ListProject)
			r.Post("/projects/{key}/permissions/grant", permsH.GrantProject)
			r.Post("/projects/{key}/permissions/revoke", permsH.RevokeProject)

			// Project-scoped profile and gate assignments
			r.Post("/projects/{key}/profiles", profilesH.AssignToProject)
			r.Post("/projects/{key}/quality-gate", gatesH.AssignToProject)

			// Project-scoped new code period
			r.Route("/projects/{key}/new-code-period", func(r chi.Router) {
				r.Get("/", periodsH.GetForProject)
				r.Put("/", periodsH.SetForProject)
				r.Delete("/", periodsH.DeleteForProject)
			})

			// Scans
			r.Post("/scans", sh.Ingest)
			r.Get("/scans/{id}", sh.Get)
			r.Get("/projects/{key}/scans", sh.ListByProject)
			r.Get("/projects/{key}/scans/latest", sh.Latest)

			// Issues
			r.Get("/issues", ih.List)
			r.Get("/issues/facets", ih.Facets)
			r.Post("/issues/{id}/transition", ih.Transition)

			// Quality profiles
			r.Get("/profiles", profilesH.List)
			r.Post("/profiles", profilesH.Create)
			r.Route("/profiles/{id}", func(r chi.Router) {
				r.Get("/", profilesH.Get)
				r.Put("/", profilesH.Update)
				r.Delete("/", profilesH.Delete)
				r.Post("/rules", profilesH.ActivateRule)
				r.Delete("/rules/{rule}", profilesH.DeactivateRule)
				r.Get("/effective-rules", profilesH.EffectiveRules)
			})

			// Quality gates
			r.Get("/quality-gates", gatesH.List)
			r.Post("/quality-gates", gatesH.Create)
			r.Route("/quality-gates/{id}", func(r chi.Router) {
				r.Get("/", gatesH.Get)
				r.Put("/", gatesH.Update)
				r.Delete("/", gatesH.Delete)
				r.Post("/conditions", gatesH.AddCondition)
				r.Delete("/conditions/{cid}", gatesH.RemoveCondition)
			})

			// New code periods (global)
			r.Get("/new-code-periods/global", periodsH.GetGlobal)
			r.Put("/new-code-periods/global", periodsH.SetGlobal)

			// Webhooks
			r.Get("/webhooks", webhooksH.List)
			r.Post("/webhooks", webhooksH.Create)
			r.Put("/webhooks/{id}", webhooksH.Update)
			r.Delete("/webhooks/{id}", webhooksH.Delete)
			r.Get("/webhooks/{id}/deliveries", webhooksH.Deliveries)
			r.Post("/webhooks/{id}/test", webhooksH.Test)

			// Measures
			r.Get("/projects/{key}/measures/trend", mh.Trend)

			// Search
			r.Get("/search", srh.Search)
		})
	})

	// ── Admin (requires admin permission) ─────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(authMW.Authenticate)
		r.Use(RequirePermission(d.Perms, "admin"))
		r.Post("/admin/reindex", func(w http.ResponseWriter, r *http.Request) {
			go func() {
				ctx := r.Context()
				if err := d.Indexer.ReindexAll(ctx, d.Issues, d.Projects); err != nil {
					_ = err
				}
			}()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"reindex started"}`))
		})
	})

	// ── Frontend (SPA fallback) ────────────────────────────────────────────
	r.Handle("/*", staticHandler())

	return r
}
