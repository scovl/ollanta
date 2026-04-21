package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
	"github.com/scovl/ollanta/ollantaweb/config"
	"github.com/scovl/ollanta/ollantaweb/ingest"
	"github.com/scovl/ollanta/ollantaweb/telemetry"
	"github.com/scovl/ollanta/ollantaweb/webhook"
)

const (
	projectByKeyPath   = "/projects/{key}"
	projectNewCodePath = "/projects/{key}/new-code-period"
)

// RouterDeps groups all dependencies needed to build the HTTP router.
type RouterDeps struct {
	Config      *config.Config
	Projects    *postgres.ProjectRepository
	Scans       *postgres.ScanRepository
	ScanJobs    *postgres.ScanJobRepository
	IndexJobs   *postgres.IndexJobRepository
	Issues      *postgres.IssueRepository
	Measures    *postgres.MeasureRepository
	Users       *postgres.UserRepository
	Groups      *postgres.GroupRepository
	Tokens      *postgres.TokenRepository
	Sessions    *postgres.SessionRepository
	Perms       *postgres.PermissionRepository
	Searcher    search.ISearcher
	Indexer     search.IIndexer
	Profiles    *postgres.ProfileRepository
	Gates       *postgres.GateRepository
	Periods     *postgres.NewCodePeriodRepository
	Webhooks    *postgres.WebhookRepository
	WebhookJobs *postgres.WebhookJobRepository
	Dispatcher  *webhook.Dispatcher
	MetricsReg  *telemetry.Registry
	Changelog   *postgres.ChangelogRepository
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
	r.Get("/metrics", d.MetricsReg.Handler())

	// ── Public badges (embeddable in READMEs, no auth) ─────────────────
	bh := &BadgesHandler{projects: d.Projects, scans: d.Scans, measures: d.Measures}
	r.Get("/api/v1/projects/{key}/badge", bh.QualityGate)

	// ── Auth middleware ────────────────────────────────────────────────────
	authMW := NewAuthMiddleware(d.Users, d.Tokens, d.Sessions, []byte(d.Config.JWTSecret), d.Config.ScannerToken)

	// ── Handlers ──────────────────────────────────────────────────────────
	authH := NewAuthHandler(d.Config, d.Users, d.Groups, d.Sessions)
	usersH := NewUsersHandler(d.Users, d.Tokens)
	groupsH := NewGroupsHandler(d.Groups)
	tokensH := NewTokensHandler(d.Tokens, d.Projects, d.Perms)
	permsH := NewPermsHandler(d.Perms, d.Projects)
	profilesH := NewProfilesHandler(d.Profiles, d.Projects)
	gatesH := NewGatesHandler(d.Gates, d.Projects)
	periodsH := NewNewCodePeriodHandler(d.Periods, d.Projects)
	webhooksH := NewWebhooksHandler(d.Webhooks, d.Projects, d.Dispatcher)
	sysH := &SystemHandler{users: d.Users, projects: d.Projects, config: d.Config}
	rulesH := NewRulesHandler()

	ph := &ProjectsHandler{repo: d.Projects}
	jobService := ingest.NewScanJobService(d.ScanJobs)
	sh := &ScansHandler{scans: d.Scans, projects: d.Projects, jobs: jobService}
	sjh := &ScanJobsHandler{jobs: jobService}
	ih := &IssuesHandler{issues: d.Issues, projects: d.Projects, changelog: d.Changelog}
	mh := &MeasuresHandler{measures: d.Measures, projects: d.Projects}
	srh := &SearchHandler{searcher: d.Searcher}
	oh := &OverviewHandler{projects: d.Projects, scans: d.Scans, issues: d.Issues, measures: d.Measures, gates: d.Gates}
	ah := &ActivityHandler{scans: d.Scans, projects: d.Projects}
	outboxH := &OutboxJobsHandler{indexJobs: d.IndexJobs, webhookJobs: d.WebhookJobs}

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
			r.Put("/users/me/password", usersH.ChangePassword)

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
				r.Post("/{id}/reactivate", usersH.Reactivate)
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

			// Projects (read is open, write requires admin)
			r.Get("/projects", ph.List)
			r.Get(projectByKeyPath, ph.Get)
			r.Group(func(r chi.Router) {
				r.Use(RequirePermission(d.Perms, "admin"))
				r.Post("/projects", ph.Create)
				r.Put(projectByKeyPath, ph.Update)
				r.Delete(projectByKeyPath, ph.Delete)
			})

			// Project-level permissions (requires admin)
			r.Get("/projects/{key}/permissions", permsH.ListProject)
			r.Group(func(r chi.Router) {
				r.Use(RequirePermission(d.Perms, "admin"))
				r.Post("/projects/{key}/permissions/grant", permsH.GrantProject)
				r.Post("/projects/{key}/permissions/revoke", permsH.RevokeProject)
			})

			// Project-scoped profile and gate assignments (requires admin)
			r.Group(func(r chi.Router) {
				r.Use(RequirePermission(d.Perms, "admin"))
				r.Post("/projects/{key}/profiles", profilesH.AssignToProject)
				r.Post("/projects/{key}/quality-gate", gatesH.AssignToProject)
			})

			// Project-scoped new code period (read is open, write requires admin)
			r.Get(projectNewCodePath, periodsH.GetForProject)
			r.Group(func(r chi.Router) {
				r.Use(RequirePermission(d.Perms, "admin"))
				r.Put(projectNewCodePath, periodsH.SetForProject)
				r.Delete(projectNewCodePath, periodsH.DeleteForProject)
			})

			// Scans
			r.Post("/scans", sh.Ingest)
			r.Get("/scans/{id}", sh.Get)
			r.Get("/scan-jobs/{id}", sjh.Get)
			r.Get("/projects/{key}/scans", sh.ListByProject)
			r.Get("/projects/{key}/scans/latest", sh.Latest)

			// Issues
			r.Get("/issues", ih.List)
			r.Get("/issues/facets", ih.Facets)
			r.Post("/issues/{id}/transition", ih.Transition)
			r.Get("/issues/{id}/changelog", ih.Changelog)

			// Rules (read-only, returns rule metadata including descriptions and examples)
			r.Get("/rules", rulesH.List)
			r.Get("/rules/*", rulesH.Get)

			// Project overview & activity (SonarQube-inspired dashboard)
			r.Get("/projects/{key}/overview", oh.Overview)
			r.Get("/projects/{key}/activity", ah.Activity)

			// Quality profiles (read is open, write requires admin)
			r.Route("/profiles", func(r chi.Router) {
				r.Get("/", profilesH.List)
				r.Get("/{id}", profilesH.Get)
				r.Get("/{id}/effective-rules", profilesH.EffectiveRules)
				r.Group(func(r chi.Router) {
					r.Use(RequirePermission(d.Perms, "admin"))
					r.Post("/", profilesH.Create)
					r.Put("/{id}", profilesH.Update)
					r.Delete("/{id}", profilesH.Delete)
					r.Post("/{id}/rules", profilesH.ActivateRule)
					r.Delete("/{id}/rules/{rule}", profilesH.DeactivateRule)
					r.Post("/{id}/copy", profilesH.Copy)
					r.Post("/{id}/set-default", profilesH.SetDefault)
				})
			})

			// Quality gates (read is open, write requires admin)
			r.Route("/quality-gates", func(r chi.Router) {
				r.Get("/", gatesH.List)
				r.Get("/{id}", gatesH.Get)
				r.Group(func(r chi.Router) {
					r.Use(RequirePermission(d.Perms, "admin"))
					r.Post("/", gatesH.Create)
					r.Put("/{id}", gatesH.Update)
					r.Delete("/{id}", gatesH.Delete)
					r.Post("/{id}/conditions", gatesH.AddCondition)
					r.Put("/{id}/conditions/{cid}", gatesH.UpdateCondition)
					r.Delete("/{id}/conditions/{cid}", gatesH.RemoveCondition)
					r.Post("/{id}/copy", gatesH.Copy)
					r.Post("/{id}/set-default", gatesH.SetDefault)
				})
			})

			// New code periods — global (read is open, write requires admin)
			r.Get("/new-code-periods/global", periodsH.GetGlobal)
			r.Group(func(r chi.Router) {
				r.Use(RequirePermission(d.Perms, "admin"))
				r.Put("/new-code-periods/global", periodsH.SetGlobal)
			})

			// Webhooks (requires admin)
			r.Route("/webhooks", func(r chi.Router) {
				r.Use(RequirePermission(d.Perms, "admin"))
				r.Get("/", webhooksH.List)
				r.Post("/", webhooksH.Create)
				r.Put("/{id}", webhooksH.Update)
				r.Delete("/{id}", webhooksH.Delete)
				r.Get("/{id}/deliveries", webhooksH.Deliveries)
				r.Post("/{id}/test", webhooksH.Test)
			})

			// Measures
			r.Get("/projects/{key}/measures/trend", mh.Trend)

			// Search
			r.Get("/search", srh.Search)

			// System info (requires admin)
			r.Group(func(r chi.Router) {
				r.Use(RequirePermission(d.Perms, "admin"))
				r.Get("/system/info", sysH.Info)
				r.Get("/admin/index-jobs", outboxH.ListIndexJobs)
				r.Post("/admin/index-jobs/{id}/retry", outboxH.RetryIndexJob)
				r.Get("/admin/webhook-jobs", outboxH.ListWebhookJobs)
				r.Post("/admin/webhook-jobs/{id}/retry", outboxH.RetryWebhookJob)
			})
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
