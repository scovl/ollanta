package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/service"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

type issueTrackingBackfillProjectStore interface {
	GetByKey(ctx context.Context, key string) (*postgres.Project, error)
}

type issueTrackingBackfillScanStore interface {
	ResolveDefaultBranch(ctx context.Context, projectID int64, configured string) (string, bool, error)
	ListBranches(ctx context.Context, projectID int64, defaultBranch string) ([]*postgres.BranchSummary, error)
	ListPullRequests(ctx context.Context, projectID int64) ([]*postgres.PullRequestSummary, error)
	ListByProjectInScope(ctx context.Context, projectID int64, scope model.AnalysisScope, defaultBranch string) ([]*postgres.Scan, error)
}

type issueTrackingBackfillIssueStore interface {
	Query(ctx context.Context, filter postgres.IssueFilter) ([]*postgres.IssueRow, int, error)
	UpdateTrackingStates(ctx context.Context, states map[int64]string) (int64, error)
}

type issueTrackingBackfillService struct {
	projects issueTrackingBackfillProjectStore
	scans    issueTrackingBackfillScanStore
	issues   issueTrackingBackfillIssueStore
}

type IssueTrackingBackfillHandler struct {
	service *issueTrackingBackfillService
}

type issueTrackingBackfillResult struct {
	ProjectKey      string `json:"project_key"`
	ScopesProcessed int    `json:"scopes_processed"`
	ScansProcessed  int    `json:"scans_processed"`
	UnknownIssues   int    `json:"unknown_issues"`
	IssuesUpdated   int64  `json:"issues_updated"`
}

func (s *issueTrackingBackfillService) BackfillProject(ctx context.Context, projectKey string) (*issueTrackingBackfillResult, error) {
	project, err := s.projects.GetByKey(ctx, projectKey)
	if err != nil {
		return nil, err
	}

	defaultBranch, _, err := s.scans.ResolveDefaultBranch(ctx, project.ID, project.MainBranch)
	if err != nil {
		return nil, err
	}

	scopes, err := s.listProjectScopes(ctx, project.ID, defaultBranch)
	if err != nil {
		return nil, err
	}

	result := &issueTrackingBackfillResult{ProjectKey: project.Key}
	for _, scope := range scopes {
		scansProcessed, unknownIssues, issuesUpdated, err := s.backfillScope(ctx, project.ID, scope, defaultBranch)
		if err != nil {
			return nil, err
		}
		result.ScopesProcessed++
		result.ScansProcessed += scansProcessed
		result.UnknownIssues += unknownIssues
		result.IssuesUpdated += issuesUpdated
	}

	return result, nil
}

func (h *IssueTrackingBackfillHandler) BackfillProject(w http.ResponseWriter, r *http.Request) {
	projectKey := chi.URLParam(r, "key")
	if projectKey == "" {
		jsonError(w, http.StatusBadRequest, "project key is required")
		return
	}

	result, err := h.service.BackfillProject(r.Context(), projectKey)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, projectNotFoundMessage)
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, http.StatusOK, result)
}

func (s *issueTrackingBackfillService) listProjectScopes(ctx context.Context, projectID int64, defaultBranch string) ([]model.AnalysisScope, error) {
	branches, err := s.scans.ListBranches(ctx, projectID, defaultBranch)
	if err != nil {
		return nil, err
	}
	pullRequests, err := s.scans.ListPullRequests(ctx, projectID)
	if err != nil {
		return nil, err
	}

	scopes := make([]model.AnalysisScope, 0, len(branches)+len(pullRequests))
	for _, branch := range branches {
		scopes = append(scopes, model.AnalysisScope{Type: model.ScopeTypeBranch, Branch: branch.Name}.Normalize())
	}
	for _, pr := range pullRequests {
		scopes = append(scopes, model.AnalysisScope{
			Type:            model.ScopeTypePullRequest,
			Branch:          pr.Branch,
			PullRequestKey:  pr.Key,
			PullRequestBase: pr.BaseBranch,
		}.Normalize())
	}
	return scopes, nil
}

func (s *issueTrackingBackfillService) backfillScope(ctx context.Context, projectID int64, scope model.AnalysisScope, defaultBranch string) (int, int, int64, error) {
	scans, err := s.scans.ListByProjectInScope(ctx, projectID, scope, defaultBranch)
	if err != nil {
		return 0, 0, 0, err
	}

	var (
		scansProcessed int
		unknownIssues  int
		issuesUpdated  int64
		previousIssues []*model.Issue
	)

	for index := len(scans) - 1; index >= 0; index-- {
		scanID := scans[index].ID
		rows, _, err := s.issues.Query(ctx, postgres.IssueFilter{
			ProjectID: &projectID,
			ScanID:    &scanID,
			Limit:     10000,
		})
		if err != nil {
			return scansProcessed, unknownIssues, issuesUpdated, err
		}

		scansProcessed++
		updates, currentUnknown := buildIssueTrackingBackfillUpdates(rows, previousIssues)
		unknownIssues += currentUnknown
		if len(updates) > 0 {
			updated, err := s.issues.UpdateTrackingStates(ctx, updates)
			if err != nil {
				return scansProcessed, unknownIssues, issuesUpdated, err
			}
			issuesUpdated += updated
		}
		previousIssues = issueRowsToDomain(rows)
	}

	return scansProcessed, unknownIssues, issuesUpdated, nil
}

func buildIssueTrackingBackfillUpdates(rows []*postgres.IssueRow, previousIssues []*model.Issue) (map[int64]string, int) {
	updates := make(map[int64]string)
	currentIssues := make([]*model.Issue, 0, len(rows))
	unknownByIssue := make(map[*model.Issue]*postgres.IssueRow)
	unknownCount := 0

	for _, row := range rows {
		if row == nil {
			continue
		}
		issue := issueRowToTrackingIssue(row)
		currentIssues = append(currentIssues, issue)
		if row.TrackingState == string(model.IssueTrackingStateUnknown) {
			unknownByIssue[issue] = row
			unknownCount++
		}
	}
	if len(unknownByIssue) == 0 {
		return updates, 0
	}

	tracking := service.Track(currentIssues, previousIssues)
	for _, issue := range tracking.New {
		if row := unknownByIssue[issue]; row != nil {
			updates[row.ID] = string(model.IssueTrackingStateNew)
		}
	}
	for _, pair := range tracking.Unchanged {
		if row := unknownByIssue[pair.Current]; row != nil {
			updates[row.ID] = string(model.IssueTrackingStateUnchanged)
		}
	}
	for _, pair := range tracking.Reopened {
		if row := unknownByIssue[pair.Current]; row != nil {
			updates[row.ID] = string(model.IssueTrackingStateReopened)
		}
	}

	return updates, unknownCount
}

func issueRowsToDomain(rows []*postgres.IssueRow) []*model.Issue {
	issues := make([]*model.Issue, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		issues = append(issues, issueRowToTrackingIssue(row))
	}
	return issues
}

func issueRowToTrackingIssue(row *postgres.IssueRow) *model.Issue {
	return &model.Issue{
		RuleKey:       row.RuleKey,
		ComponentPath: row.ComponentPath,
		Line:          row.Line,
		Column:        row.Column,
		EndLine:       row.EndLine,
		EndColumn:     row.EndColumn,
		Message:       row.Message,
		Type:          model.IssueType(row.Type),
		Severity:      model.Severity(row.Severity),
		Status:        model.Status(row.Status),
		Resolution:    row.Resolution,
		EffortMinutes: row.EffortMinutes,
		LineHash:      row.LineHash,
		Tags:          row.Tags,
	}
}
