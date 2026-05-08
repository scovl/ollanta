package api

import (
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
)

// ── Common wrappers ─────────────────────────────────────────────────────

type paginatedItemsResponse struct {
	Items  interface{} `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type paginatedItemsWithScopeResponse struct {
	Items  interface{}  `json:"items"`
	Total  int          `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
	Scope  *scopeResponse `json:"scope,omitempty"`
}

type itemsOnlyResponse struct {
	Items interface{} `json:"items"`
}

type idStatusResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

// ── Auth ────────────────────────────────────────────────────────────────

// refreshResponse is returned by POST /api/v1/auth/refresh.
type refreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// ── Users ───────────────────────────────────────────────────────────────

type userListResponse struct {
	Users []userView `json:"users"`
	Total int        `json:"total"`
}

// ── Groups ──────────────────────────────────────────────────────────────

type groupListResponse struct {
	Groups []groupView `json:"groups"`
}

type groupMembersResponse struct {
	Members []userView `json:"members"`
}

// ── Tokens ──────────────────────────────────────────────────────────────

type tokenListResponse struct {
	Tokens []tokenView `json:"tokens"`
}

type tokenCreateResponse struct {
	Token string    `json:"token"`
	Meta  tokenView `json:"meta"`
}

// ── Permissions ─────────────────────────────────────────────────────────

type permListResponse struct {
	Permissions interface{} `json:"permissions"`
}

// ── Projects ────────────────────────────────────────────────────────────

type projectListResponse struct {
	Items  []*postgres.Project `json:"items"`
	Total  int                 `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// ── Scans ───────────────────────────────────────────────────────────────

type scanListResponse struct {
	Items  []*postgres.Scan `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
	Scope  *scopeResponse   `json:"scope,omitempty"`
}

type survivedMutantsResponse struct {
	ScanID  int64                `json:"scan_id"`
	Mutants []survivedMutantItem `json:"mutants"`
	Total   int                  `json:"total"`
}

// ── Issues ──────────────────────────────────────────────────────────────

type issueListResponse struct {
	Items  []*postgres.IssueRow `json:"items"`
	Total  int                  `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

type issueChangelogResponse struct {
	Items []postgres.ChangelogEntry `json:"items"`
}

// ── Rules ───────────────────────────────────────────────────────────────

type ruleListResponse []*ruleDetail

// ── Tags ────────────────────────────────────────────────────────────────

type tagListResponse struct {
	Items  interface{} `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type savedFilterListResponse struct {
	Items interface{} `json:"items"`
	Total int         `json:"total"`
}

// ── Custom rules ────────────────────────────────────────────────────────

type customRuleListResponse struct {
	Items interface{} `json:"items"`
}

type customRulePreviewResponse model.CustomRulePreviewResult

// ── Custom rule AI ──────────────────────────────────────────────────────

type customRuleAIProvidersResponse struct {
	Providers []customRuleAIProviderOption `json:"providers"`
}

type customRuleAISuggestResponse struct {
	Suggestion customRuleAISuggestion `json:"suggestion"`
}

// ── Quality gates ───────────────────────────────────────────────────────

type gateDetailResponse struct {
	Gate       *postgres.QualityGate      `json:"gate"`
	Conditions []*postgres.GateCondition `json:"conditions"`
}

// ── Quality profiles ────────────────────────────────────────────────────

type profileImportResponse struct {
	ImportedRules int `json:"imported_rules"`
}

type profileChangelogResponse struct {
	Items  interface{} `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// ── New code periods ────────────────────────────────────────────────────

type newCodePeriodInheritedResponse struct {
	Strategy string `json:"strategy"`
	Value    string `json:"value"`
	Scope    string `json:"scope"`
}

// ── Measures ────────────────────────────────────────────────────────────

type measureTrendResponse struct {
	Project   string             `json:"project"`
	Metric    string             `json:"metric"`
	From      string             `json:"from"`
	To        string             `json:"to"`
	Points    []postgres.TrendPoint `json:"points"`
	Component string             `json:"component,omitempty"`
}

// ── Search ──────────────────────────────────────────────────────────────

// searchResponse wraps search.SearchResult for swagger typing.
type searchResponse search.SearchResult

// ── System ──────────────────────────────────────────────────────────────

type systemInfoResponse struct {
	Version        string         `json:"version"`
	GoVersion      string         `json:"go_version"`
	OS             string         `json:"os"`
	Arch           string         `json:"arch"`
	NumGoroutines  int            `json:"num_goroutines"`
	SearchBackend  string         `json:"search_backend"`
	Stats          systemStats    `json:"stats"`
}

type systemStats struct {
	Users    int64 `json:"users"`
	Projects int64 `json:"projects"`
}

type uiSettingsResponse struct {
	ObservabilityLinks []uiObservabilityLink `json:"observability_links"`
}

type uiObservabilityLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ── Health ──────────────────────────────────────────────────────────────

type readinessResponse struct {
	Status string                 `json:"status"`
	Checks map[string]checkResult `json:"checks"`
}

// ── Background tasks ────────────────────────────────────────────────────

type backgroundTaskListResponse struct {
	Items  []*backgroundTaskDTO `json:"items"`
	Total  int                  `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

// ── Outbox ──────────────────────────────────────────────────────────────

type outboxJobListResponse struct {
	Items  interface{} `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// ── Webhooks ────────────────────────────────────────────────────────────

type webhookTestResponse struct {
	Status string `json:"status"`
}

// ── Project scope ───────────────────────────────────────────────────────

type branchesResponse struct {
	DefaultBranch string                    `json:"default_branch"`
	Items         []*postgres.BranchSummary `json:"items"`
}

type pullRequestsResponse struct {
	Items []*postgres.PullRequestSummary `json:"items"`
}

type projectInformationResponse struct {
	Project      *postgres.Project          `json:"project"`
	Scope        *scopeResponse             `json:"scope"`
	LatestScan   *postgres.Scan             `json:"latest_scan"`
	CodeSnapshot *postgres.CodeSnapshotScope `json:"code_snapshot"`
	Measures     map[string]interface{}     `json:"measures"`
}

type codeTreeResponse struct {
	Scope        *scopeResponse             `json:"scope"`
	CodeSnapshot *postgres.CodeSnapshotScope `json:"code_snapshot"`
	Items        []*postgres.CodeSnapshotFile `json:"items"`
}

type codeFileResponse struct {
	Scope  *scopeResponse       `json:"scope"`
	File   *postgres.CodeSnapshotFile `json:"file"`
	Issues []*postgres.IssueRow `json:"issues"`
}

// ── Activity ────────────────────────────────────────────────────────────

type activityResponse struct {
	Items  []activityEntry `json:"items"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
	Scope  *scopeResponse  `json:"scope,omitempty"`
}

// ── Overview ────────────────────────────────────────────────────────────

// overviewResponse is already defined in overview.go; listed here for reference.

// ── Issue tracking backfill ─────────────────────────────────────────────

// issueTrackingBackfillResult is already defined in issue_tracking_backfill.go.

// ── Badges ──────────────────────────────────────────────────────────────
// Badges return SVG, not JSON; no DTO needed.

// ── Admin reindex ───────────────────────────────────────────────────────

type reindexResponse struct {
	Status string `json:"status"`
}

// ── Engines ─────────────────────────────────────────────────────────────

type enginesResponse struct {
	Engines interface{} `json:"engines"`
}

// ── Saved filters ───────────────────────────────────────────────────────

type savedFilterCriteriaResponse struct {
	Criteria interface{} `json:"criteria"`
	Filter   interface{} `json:"filter"`
}
