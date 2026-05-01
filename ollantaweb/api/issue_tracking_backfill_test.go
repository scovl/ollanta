package api

import (
	"context"
	"testing"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

func TestIssueTrackingBackfillServiceBackfillProject(t *testing.T) {
	t.Parallel()

	service := &issueTrackingBackfillService{
		projects: &fakeIssueTrackingBackfillProjects{
			project: &postgres.Project{ID: 7, Key: "demo", MainBranch: "main"},
		},
		scans: &fakeIssueTrackingBackfillScans{
			defaultBranch: "main",
			branches:      []*postgres.BranchSummary{{Name: "main", IsDefault: true}},
			scansByScope: map[string][]*postgres.Scan{
				"branch:main": {
					{ID: 22, ProjectID: 7, ScopeType: model.ScopeTypeBranch, Branch: "main"},
					{ID: 11, ProjectID: 7, ScopeType: model.ScopeTypeBranch, Branch: "main"},
				},
			},
		},
		issues: &fakeIssueTrackingBackfillIssues{
			rowsByScanID: map[int64][]*postgres.IssueRow{
				11: {
					{
						ID:            101,
						ProjectID:     7,
						ScanID:        11,
						RuleKey:       "go:legacy-bug",
						ComponentPath: "main.go",
						Line:          10,
						LineHash:      "hash-a",
						Status:        string(model.StatusOpen),
						Type:          string(model.TypeBug),
						Severity:      string(model.SeverityMajor),
						TrackingState: string(model.IssueTrackingStateUnknown),
					},
				},
				22: {
					{
						ID:            201,
						ProjectID:     7,
						ScanID:        22,
						RuleKey:       "go:legacy-bug",
						ComponentPath: "main.go",
						Line:          12,
						LineHash:      "hash-a",
						Status:        string(model.StatusOpen),
						Type:          string(model.TypeBug),
						Severity:      string(model.SeverityMajor),
						TrackingState: string(model.IssueTrackingStateUnknown),
					},
					{
						ID:            202,
						ProjectID:     7,
						ScanID:        22,
						RuleKey:       "go:new-smell",
						ComponentPath: "main.go",
						Line:          30,
						LineHash:      "hash-b",
						Status:        string(model.StatusOpen),
						Type:          string(model.TypeCodeSmell),
						Severity:      string(model.SeverityMinor),
						TrackingState: string(model.IssueTrackingStateUnknown),
					},
					{
						ID:            203,
						ProjectID:     7,
						ScanID:        22,
						RuleKey:       "go:keep-known",
						ComponentPath: "main.go",
						Line:          42,
						LineHash:      "hash-c",
						Status:        string(model.StatusOpen),
						Type:          string(model.TypeBug),
						Severity:      string(model.SeverityCritical),
						TrackingState: string(model.IssueTrackingStateReopened),
					},
				},
			},
		},
	}

	result, err := service.BackfillProject(context.Background(), "demo")
	if err != nil {
		t.Fatalf("BackfillProject() error = %v", err)
	}

	if result.ScopesProcessed != 1 {
		t.Fatalf("ScopesProcessed = %d, want 1", result.ScopesProcessed)
	}
	if result.ScansProcessed != 2 {
		t.Fatalf("ScansProcessed = %d, want 2", result.ScansProcessed)
	}
	if result.UnknownIssues != 3 {
		t.Fatalf("UnknownIssues = %d, want 3", result.UnknownIssues)
	}
	if result.IssuesUpdated != 3 {
		t.Fatalf("IssuesUpdated = %d, want 3", result.IssuesUpdated)
	}

	updated := service.issues.(*fakeIssueTrackingBackfillIssues).updatedStates
	if got := updated[101]; got != string(model.IssueTrackingStateNew) {
		t.Fatalf("scan 11 tracking_state = %q, want new", got)
	}
	if got := updated[201]; got != string(model.IssueTrackingStateUnchanged) {
		t.Fatalf("scan 22 repeated issue tracking_state = %q, want unchanged", got)
	}
	if got := updated[202]; got != string(model.IssueTrackingStateNew) {
		t.Fatalf("scan 22 fresh issue tracking_state = %q, want new", got)
	}
	if _, ok := updated[203]; ok {
		t.Fatalf("known tracking_state should not be rewritten")
	}
}

type fakeIssueTrackingBackfillProjects struct {
	project *postgres.Project
}

func (f *fakeIssueTrackingBackfillProjects) GetByKey(_ context.Context, key string) (*postgres.Project, error) {
	if f.project == nil || f.project.Key != key {
		return nil, postgres.ErrNotFound
	}
	clone := *f.project
	return &clone, nil
}

type fakeIssueTrackingBackfillScans struct {
	defaultBranch string
	branches      []*postgres.BranchSummary
	pullRequests  []*postgres.PullRequestSummary
	scansByScope  map[string][]*postgres.Scan
}

func (f *fakeIssueTrackingBackfillScans) ResolveDefaultBranch(_ context.Context, _ int64, configured string) (string, bool, error) {
	if configured != "" {
		return configured, false, nil
	}
	return f.defaultBranch, true, nil
}

func (f *fakeIssueTrackingBackfillScans) ListBranches(_ context.Context, _ int64, _ string) ([]*postgres.BranchSummary, error) {
	items := make([]*postgres.BranchSummary, 0, len(f.branches))
	for _, item := range f.branches {
		clone := *item
		items = append(items, &clone)
	}
	return items, nil
}

func (f *fakeIssueTrackingBackfillScans) ListPullRequests(_ context.Context, _ int64) ([]*postgres.PullRequestSummary, error) {
	items := make([]*postgres.PullRequestSummary, 0, len(f.pullRequests))
	for _, item := range f.pullRequests {
		clone := *item
		items = append(items, &clone)
	}
	return items, nil
}

func (f *fakeIssueTrackingBackfillScans) ListByProjectInScope(_ context.Context, _ int64, scope model.AnalysisScope, _ string) ([]*postgres.Scan, error) {
	lookup := storedScopeKey(scope)
	items := f.scansByScope[lookup]
	clones := make([]*postgres.Scan, 0, len(items))
	for _, item := range items {
		clone := *item
		clones = append(clones, &clone)
	}
	return clones, nil
}

func storedScopeKey(scope model.AnalysisScope) string {
	scope = scope.Normalize()
	if scope.Type == model.ScopeTypePullRequest {
		return "pr:" + scope.PullRequestKey
	}
	return "branch:" + scope.Branch
}

type fakeIssueTrackingBackfillIssues struct {
	rowsByScanID    map[int64][]*postgres.IssueRow
	updatedStates   map[int64]string
	updatedIssueIDs []int64
}

func (f *fakeIssueTrackingBackfillIssues) Query(_ context.Context, filter postgres.IssueFilter) ([]*postgres.IssueRow, int, error) {
	if filter.ScanID == nil {
		return nil, 0, nil
	}
	rows := f.rowsByScanID[*filter.ScanID]
	items := make([]*postgres.IssueRow, 0, len(rows))
	for _, row := range rows {
		clone := *row
		items = append(items, &clone)
	}
	return items, len(items), nil
}

func (f *fakeIssueTrackingBackfillIssues) UpdateTrackingStates(_ context.Context, states map[int64]string) (int64, error) {
	if f.updatedStates == nil {
		f.updatedStates = make(map[int64]string)
	}
	var updated int64
	for id, state := range states {
		f.updatedStates[id] = state
		f.updatedIssueIDs = append(f.updatedIssueIDs, id)
		updated++
	}
	return updated, nil
}
