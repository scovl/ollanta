package ingest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/scovl/ollanta/domain/model"
)

const (
	ingestErrorMessage  = "Ingest() error = %v"
	featureCartBranch   = "feature/cart"
	cartPath            = "cart.go"
	defaultMainBranch   = "main"
	releaseBranch       = "release"
	branchSnapshotScope = "branch:release"
)

func TestIngestUsesBranchScopedPreviousScan(t *testing.T) {
	t.Parallel()

	projectRepo := &scopeAwareProjectRepo{
		project: &model.Project{ID: 1, Key: "shop", MainBranch: defaultMainBranch},
	}
	scanRepo := &observingScanRepo{
		defaultBranch: defaultMainBranch,
		globalLatest:  &model.Scan{ID: 10, ProjectID: 1, ScopeType: model.ScopeTypeBranch, Branch: defaultMainBranch},
	}
	issueRepo := &queryIssueRepo{
		rowsByScanID: map[int64][]*model.IssueRow{
			10: {toIssueRow(newTestIssue("go:branch-scope", cartPath), 10, 1)},
		},
	}

	uc := NewIngestUseCase(projectRepo, scanRepo, issueRepo, &fakeMeasureRepo{}, nil, nil, nil)
	result, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{
			ProjectKey:   "shop",
			ScopeType:    model.ScopeTypeBranch,
			Branch:       featureCartBranch,
			AnalysisDate: time.Now().UTC().Format(time.RFC3339),
		},
		Measures: IngestMeasures{Files: 1, Lines: 8, Ncloc: 8},
		Issues:   []*model.Issue{newTestIssue("go:branch-scope", cartPath)},
	})
	if err != nil {
		t.Fatalf(ingestErrorMessage, err)
	}
	if result.NewIssues != 1 {
		t.Fatalf("NewIssues = %d, want 1", result.NewIssues)
	}
	if scanRepo.getLatestCalls != 0 {
		t.Fatalf("GetLatest() calls = %d, want 0", scanRepo.getLatestCalls)
	}
	if scanRepo.getLatestInScopeCalls != 1 {
		t.Fatalf("GetLatestInScope() calls = %d, want 1", scanRepo.getLatestInScopeCalls)
	}
	if scanRepo.lastDefaultBranch != defaultMainBranch {
		t.Fatalf("default branch = %q, want main", scanRepo.lastDefaultBranch)
	}
	if scanRepo.lastScope.Type != model.ScopeTypeBranch || scanRepo.lastScope.Branch != featureCartBranch {
		t.Fatalf("scope = %+v, want branch feature/cart", scanRepo.lastScope)
	}
	if len(scanRepo.created) != 1 || scanRepo.created[0].Branch != featureCartBranch {
		t.Fatalf("created scan branch = %q, want feature/cart", firstCreatedBranch(scanRepo.created))
	}
	if len(issueRepo.lastQueryScanIDs) != 0 {
		t.Fatalf("previous issue query scan IDs = %v, want none", issueRepo.lastQueryScanIDs)
	}
}

func TestIngestAcceptsOptionalTestSignals(t *testing.T) {
	t.Parallel()

	projectRepo := &scopeAwareProjectRepo{project: &model.Project{ID: 1, Key: "shop", MainBranch: defaultMainBranch}}
	scanRepo := &observingScanRepo{defaultBranch: defaultMainBranch}
	uc := NewIngestUseCase(projectRepo, scanRepo, &queryIssueRepo{}, &fakeMeasureRepo{}, nil, nil, nil)

	result, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{
			ProjectKey:   "shop",
			AnalysisDate: time.Now().UTC().Format(time.RFC3339),
		},
		Measures:    IngestMeasures{Files: 1, Lines: 8, Ncloc: 8},
		Issues:      []*model.Issue{},
		TestSignals: json.RawMessage(`{"summary":{"enabled":true,"modules":1},"modules":[{"name":"domain","root":"domain"}]}`),
	})
	if err != nil {
		t.Fatalf(ingestErrorMessage, err)
	}
	if result.ScanID == 0 {
		t.Fatal("ScanID = 0, want persisted scan")
	}
	if len(scanRepo.created) != 1 {
		t.Fatalf("created scans = %d, want 1", len(scanRepo.created))
	}
}

func TestIngestPersistsCoverageFileMeasures(t *testing.T) {
	t.Parallel()

	projectRepo := &scopeAwareProjectRepo{project: &model.Project{ID: 1, Key: "shop", MainBranch: defaultMainBranch}}
	scanRepo := &observingScanRepo{defaultBranch: defaultMainBranch}
	measureRepo := &fakeMeasureRepo{}
	uc := NewIngestUseCase(projectRepo, scanRepo, &queryIssueRepo{}, measureRepo, nil, nil, nil)

	_, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{
			ProjectKey:   "shop",
			AnalysisDate: time.Now().UTC().Format(time.RFC3339),
		},
		Measures:    IngestMeasures{Files: 1, Lines: 8, Ncloc: 8},
		Issues:      []*model.Issue{},
		TestSignals: json.RawMessage(`{"modules":[{"name":"scan","files":[{"path":"application/scan/usecase.go","lines_to_cover":10,"covered_lines":7,"uncovered_lines":[12,18,24]}]}]}`),
	})
	if err != nil {
		t.Fatalf(ingestErrorMessage, err)
	}
	coverage := findInsertedMeasure(measureRepo.inserted, model.MetricCoverage, "application/scan/usecase.go")
	if coverage == nil || coverage.Value != 70 {
		t.Fatalf("coverage measure = %+v, want 70", coverage)
	}
	uncovered := findInsertedMeasure(measureRepo.inserted, model.MetricUncoveredLines, "application/scan/usecase.go")
	if uncovered == nil || uncovered.Value != 3 {
		t.Fatalf("uncovered measure = %+v, want 3", uncovered)
	}
}

func findInsertedMeasure(rows []model.MeasureRow, metricKey, componentPath string) *model.MeasureRow {
	for i := range rows {
		if rows[i].MetricKey == metricKey && rows[i].ComponentPath == componentPath {
			return &rows[i]
		}
	}
	return nil
}

func TestIngestUsesPullRequestScopedPreviousScan(t *testing.T) {
	t.Parallel()

	projectRepo := &scopeAwareProjectRepo{
		project: &model.Project{ID: 1, Key: "shop", MainBranch: defaultMainBranch},
	}
	previousPR := &model.Scan{
		ID:             29,
		ProjectID:      1,
		ScopeType:      model.ScopeTypePullRequest,
		Branch:         featureCartBranch,
		PullRequestKey: "129",
	}
	scanRepo := &observingScanRepo{
		defaultBranch: defaultMainBranch,
		globalLatest:  previousPR,
		previous: map[string]*model.Scan{
			"pr:129": previousPR,
		},
	}
	issueRepo := &queryIssueRepo{
		rowsByScanID: map[int64][]*model.IssueRow{
			29: {toIssueRow(newTestIssue("go:pr-scope", cartPath), 29, 1)},
		},
	}

	uc := NewIngestUseCase(projectRepo, scanRepo, issueRepo, &fakeMeasureRepo{}, nil, nil, nil)
	result, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{
			ProjectKey:      "shop",
			ScopeType:       model.ScopeTypePullRequest,
			Branch:          "feature/login",
			PullRequestKey:  "128",
			PullRequestBase: defaultMainBranch,
			AnalysisDate:    time.Now().UTC().Format(time.RFC3339),
		},
		Measures: IngestMeasures{Files: 1, Lines: 8, Ncloc: 8},
		Issues:   []*model.Issue{newTestIssue("go:pr-scope", cartPath)},
	})
	if err != nil {
		t.Fatalf(ingestErrorMessage, err)
	}
	if result.NewIssues != 1 {
		t.Fatalf("NewIssues = %d, want 1", result.NewIssues)
	}
	if scanRepo.getLatestCalls != 0 {
		t.Fatalf("GetLatest() calls = %d, want 0", scanRepo.getLatestCalls)
	}
	if scanRepo.lastScope.Type != model.ScopeTypePullRequest || scanRepo.lastScope.PullRequestKey != "128" {
		t.Fatalf("scope = %+v, want pull request 128", scanRepo.lastScope)
	}
	created := scanRepo.created[len(scanRepo.created)-1]
	if created.ScopeType != model.ScopeTypePullRequest || created.PullRequestKey != "128" || created.PullRequestBase != defaultMainBranch {
		t.Fatalf("created scan = %+v, want pull request metadata", created)
	}
	if len(issueRepo.lastQueryScanIDs) != 0 {
		t.Fatalf("previous issue query scan IDs = %v, want none", issueRepo.lastQueryScanIDs)
	}
}

func TestIngestUsesResolvedDefaultBranchForLegacyHistoricalScans(t *testing.T) {
	t.Parallel()

	legacyScan := &model.Scan{ID: 14, ProjectID: 1, ScopeType: model.ScopeTypeBranch, Branch: ""}
	projectRepo := &scopeAwareProjectRepo{project: &model.Project{ID: 1, Key: "shop"}}
	scanRepo := &observingScanRepo{
		defaultBranch: defaultMainBranch,
		previous: map[string]*model.Scan{
			"legacy:main": legacyScan,
		},
	}
	issueRepo := &queryIssueRepo{
		rowsByScanID: map[int64][]*model.IssueRow{
			14: {toIssueRow(newTestIssue("go:legacy", "legacy.go"), 14, 1)},
		},
	}

	uc := NewIngestUseCase(projectRepo, scanRepo, issueRepo, &fakeMeasureRepo{}, nil, nil, nil)
	result, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{
			ProjectKey:   "shop",
			AnalysisDate: time.Now().UTC().Format(time.RFC3339),
		},
		Measures: IngestMeasures{Files: 1, Lines: 6, Ncloc: 6},
		Issues:   []*model.Issue{newTestIssue("go:legacy", "legacy.go")},
	})
	if err != nil {
		t.Fatalf(ingestErrorMessage, err)
	}
	if result.NewIssues != 0 {
		t.Fatalf("NewIssues = %d, want 0", result.NewIssues)
	}
	if scanRepo.lastDefaultBranch != defaultMainBranch {
		t.Fatalf("default branch = %q, want main", scanRepo.lastDefaultBranch)
	}
	if scanRepo.lastScope.Type != model.ScopeTypeBranch || scanRepo.lastScope.Branch != "" {
		t.Fatalf("scope = %+v, want blank branch branch-scope request", scanRepo.lastScope)
	}
	if len(issueRepo.lastQueryScanIDs) != 1 || issueRepo.lastQueryScanIDs[0] != 14 {
		t.Fatalf("previous issue query scan IDs = %v, want [14]", issueRepo.lastQueryScanIDs)
	}
}

func TestIngestReplacesLatestCodeSnapshotForSameScope(t *testing.T) {
	t.Parallel()

	projectRepo := &scopeAwareProjectRepo{
		project: &model.Project{ID: 1, Key: "shop", MainBranch: defaultMainBranch},
	}
	scanRepo := &observingScanRepo{defaultBranch: defaultMainBranch}
	snapshotRepo := &replacingSnapshotRepo{}
	uc := NewIngestUseCase(projectRepo, scanRepo, &queryIssueRepo{}, &fakeMeasureRepo{}, snapshotRepo, nil, nil)

	firstContent := "package shop\nconst version = 1\n"
	first, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{
			ProjectKey:   "shop",
			ScopeType:    model.ScopeTypeBranch,
			Branch:       releaseBranch,
			AnalysisDate: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		},
		Measures: IngestMeasures{Files: 1, Lines: 2, Ncloc: 2},
		CodeSnapshot: &model.CodeSnapshot{
			Files:         []model.CodeSnapshotFile{{Path: "src/app.go", Content: firstContent, LineCount: 2, SizeBytes: len(firstContent)}},
			TotalFiles:    1,
			StoredFiles:   1,
			StoredBytes:   len(firstContent),
			MaxFileBytes:  128,
			MaxTotalBytes: 1024,
		},
	})
	if err != nil {
		t.Fatalf("first "+ingestErrorMessage, err)
	}

	secondContent := "package shop\nconst version = 2\n"
	second, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{
			ProjectKey:   "shop",
			ScopeType:    model.ScopeTypeBranch,
			Branch:       releaseBranch,
			AnalysisDate: time.Now().UTC().Format(time.RFC3339),
		},
		Measures: IngestMeasures{Files: 1, Lines: 2, Ncloc: 2},
		CodeSnapshot: &model.CodeSnapshot{
			Files:         []model.CodeSnapshotFile{{Path: "src/app.go", Content: secondContent, LineCount: 2, SizeBytes: len(secondContent)}},
			TotalFiles:    1,
			StoredFiles:   1,
			StoredBytes:   len(secondContent),
			MaxFileBytes:  128,
			MaxTotalBytes: 1024,
		},
	})
	if err != nil {
		t.Fatalf("second "+ingestErrorMessage, err)
	}

	stored, ok := snapshotRepo.states[branchSnapshotScope]
	if !ok {
		t.Fatal("expected latest snapshot for branch:release")
	}
	if stored.ScanID != second.ScanID {
		t.Fatalf("stored ScanID = %d, want %d", stored.ScanID, second.ScanID)
	}
	if stored.ScanID == first.ScanID {
		t.Fatalf("stored ScanID = %d, want replacement after first scan %d", stored.ScanID, first.ScanID)
	}
	if got := stored.Snapshot.Files[0].Content; got != secondContent {
		t.Fatalf("stored snapshot content = %q, want %q", got, secondContent)
	}
	if snapshotRepo.calls != 2 {
		t.Fatalf("Replace() calls = %d, want 2", snapshotRepo.calls)
	}
}

func TestIngestPersistsTrackingStateForCurrentIssues(t *testing.T) {
	t.Parallel()

	projectRepo := &scopeAwareProjectRepo{
		project: &model.Project{ID: 1, Key: "shop", MainBranch: defaultMainBranch},
	}
	previous := &model.Scan{ID: 44, ProjectID: 1, ScopeType: model.ScopeTypeBranch, Branch: defaultMainBranch}
	scanRepo := &observingScanRepo{
		defaultBranch: defaultMainBranch,
		previous: map[string]*model.Scan{
			"legacy:main": previous,
		},
	}

	unchangedPrev := newTestIssue("go:unchanged", "same.go")
	unchangedPrev.Status = model.StatusOpen
	reopenedPrev := newTestIssue("go:reopened", "reopened.go")
	reopenedPrev.Status = model.StatusClosed

	issueRepo := &queryIssueRepo{
		rowsByScanID: map[int64][]*model.IssueRow{
			44: {
				toIssueRow(unchangedPrev, 44, 1),
				toIssueRow(reopenedPrev, 44, 1),
			},
		},
	}

	currentUnchanged := newTestIssue("go:unchanged", "same.go")
	currentReopened := newTestIssue("go:reopened", "reopened.go")
	currentNew := newTestIssue("go:new", "new.go")

	uc := NewIngestUseCase(projectRepo, scanRepo, issueRepo, &fakeMeasureRepo{}, nil, nil, nil)
	_, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{
			ProjectKey:   "shop",
			AnalysisDate: time.Now().UTC().Format(time.RFC3339),
		},
		Measures: IngestMeasures{Files: 3, Lines: 18, Ncloc: 18},
		Issues:   []*model.Issue{currentUnchanged, currentReopened, currentNew},
	})
	if err != nil {
		t.Fatalf(ingestErrorMessage, err)
	}
	if len(issueRepo.inserted) != 3 {
		t.Fatalf("inserted issues = %d, want 3", len(issueRepo.inserted))
	}

	trackingByRule := map[string]string{}
	for _, issue := range issueRepo.inserted {
		trackingByRule[issue.RuleKey] = issue.TrackingState
	}
	if trackingByRule["go:unchanged"] != string(model.IssueTrackingStateUnchanged) {
		t.Fatalf("tracking_state for unchanged = %q, want unchanged", trackingByRule["go:unchanged"])
	}
	if trackingByRule["go:reopened"] != string(model.IssueTrackingStateReopened) {
		t.Fatalf("tracking_state for reopened = %q, want reopened", trackingByRule["go:reopened"])
	}
	if trackingByRule["go:new"] != string(model.IssueTrackingStateNew) {
		t.Fatalf("tracking_state for new = %q, want new", trackingByRule["go:new"])
	}
}

func TestIngestPersistsTestAndMutationMetrics(t *testing.T) {
	t.Parallel()

	measureRepo := &fakeMeasureRepo{}
	uc := NewIngestUseCase(
		&scopeAwareProjectRepo{project: &model.Project{ID: 1, Key: "shop", MainBranch: defaultMainBranch}},
		&observingScanRepo{defaultBranch: defaultMainBranch},
		&queryIssueRepo{},
		measureRepo,
		nil,
		nil,
		nil,
	)
	mutationScore := 83.5
	coverage := 91.2
	_, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{ProjectKey: "shop", AnalysisDate: time.Now().UTC().Format(time.RFC3339)},
		Measures: IngestMeasures{
			Files:           1,
			Lines:           8,
			Ncloc:           8,
			Coverage:        &coverage,
			Tests:           12,
			TestFailures:    1,
			MutationScore:   &mutationScore,
			MutantsTotal:    10,
			MutantsKilled:   8,
			MutantsSurvived: 2,
		},
	})
	if err != nil {
		t.Fatalf(ingestErrorMessage, err)
	}

	values := map[string]float64{}
	for _, measure := range measureRepo.inserted {
		values[measure.MetricKey] = measure.Value
	}
	if values[model.MetricTests] != 12 {
		t.Fatalf("tests measure = %v, want 12", values[model.MetricTests])
	}
	if values[model.MetricCoverage] != coverage {
		t.Fatalf("coverage measure = %v, want %v", values[model.MetricCoverage], coverage)
	}
	if values[model.MetricTestFailures] != 1 {
		t.Fatalf("test_failures measure = %v, want 1", values[model.MetricTestFailures])
	}
	if values[model.MetricMutationScore] != mutationScore {
		t.Fatalf("mutation_score measure = %v, want %v", values[model.MetricMutationScore], mutationScore)
	}
	if values[model.MetricMutantsSurvived] != 2 {
		t.Fatalf("mutants_survived measure = %v, want 2", values[model.MetricMutantsSurvived])
	}
}

func TestIngestDerivesLanguageAndTestabilityForActionableFindings(t *testing.T) {
	t.Parallel()

	issueRepo := &queryIssueRepo{}
	uc := NewIngestUseCase(
		&scopeAwareProjectRepo{project: &model.Project{ID: 1, Key: "shop", MainBranch: defaultMainBranch}},
		&observingScanRepo{defaultBranch: defaultMainBranch},
		issueRepo,
		&fakeMeasureRepo{},
		nil,
		nil,
		nil,
	)
	issue := newTestIssue("test:survived-mutant", "internal/cart_test.go")
	issue.Tags = []string{"mutation", "survived-mutant"}
	_, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{ProjectKey: "shop", AnalysisDate: time.Now().UTC().Format(time.RFC3339)},
		Measures: IngestMeasures{Files: 1, Lines: 8, Ncloc: 8},
		Issues:   []*model.Issue{issue},
	})
	if err != nil {
		t.Fatalf(ingestErrorMessage, err)
	}
	if len(issueRepo.inserted) != 1 {
		t.Fatalf("inserted issues = %d, want 1", len(issueRepo.inserted))
	}
	inserted := issueRepo.inserted[0]
	if inserted.QualityDomain != string(model.QualityTestability) {
		t.Fatalf("quality_domain = %q, want testability", inserted.QualityDomain)
	}
	if inserted.Language != model.LangGo {
		t.Fatalf("language = %q, want go", inserted.Language)
	}
}

type scopeAwareProjectRepo struct {
	project *model.Project
}

func (r *scopeAwareProjectRepo) Create(_ context.Context, p *model.Project) error {
	return r.Upsert(context.Background(), p)
}

func (r *scopeAwareProjectRepo) Upsert(_ context.Context, p *model.Project) error {
	if r.project == nil {
		r.project = &model.Project{ID: 1, Key: p.Key}
	}
	p.ID = r.project.ID
	p.Key = r.project.Key
	if p.Name == "" {
		p.Name = r.project.Key
	}
	p.MainBranch = r.project.MainBranch
	return nil
}

func (r *scopeAwareProjectRepo) GetByKey(_ context.Context, _ string) (*model.Project, error) {
	if r.project == nil {
		return nil, model.ErrNotFound
	}
	clone := *r.project
	return &clone, nil
}

func (r *scopeAwareProjectRepo) GetByID(_ context.Context, _ int64) (*model.Project, error) {
	if r.project == nil {
		return nil, model.ErrNotFound
	}
	clone := *r.project
	return &clone, nil
}

func (r *scopeAwareProjectRepo) List(_ context.Context) ([]*model.Project, error) { return nil, nil }

func (r *scopeAwareProjectRepo) Delete(_ context.Context, _ int64) error { return nil }

type observingScanRepo struct {
	defaultBranch         string
	globalLatest          *model.Scan
	previous              map[string]*model.Scan
	created               []*model.Scan
	nextID                int64
	getLatestCalls        int
	getLatestInScopeCalls int
	lastScope             model.AnalysisScope
	lastDefaultBranch     string
}

func (r *observingScanRepo) Create(_ context.Context, s *model.Scan) error {
	r.nextID++
	s.ID = r.nextID
	clone := *s
	r.created = append(r.created, &clone)
	return nil
}

func (r *observingScanRepo) Update(_ context.Context, _ *model.Scan) error { return nil }

func (r *observingScanRepo) GetByID(_ context.Context, _ int64) (*model.Scan, error) {
	return nil, model.ErrNotFound
}

func (r *observingScanRepo) GetLatest(_ context.Context, _ int64) (*model.Scan, error) {
	r.getLatestCalls++
	if r.globalLatest == nil {
		return nil, model.ErrNotFound
	}
	clone := *r.globalLatest
	return &clone, nil
}

func (r *observingScanRepo) GetLatestInScope(_ context.Context, _ int64, scope model.AnalysisScope, defaultBranch string) (*model.Scan, error) {
	r.getLatestInScopeCalls++
	r.lastScope = scope.Normalize()
	r.lastDefaultBranch = defaultBranch
	if r.previous == nil {
		return nil, model.ErrNotFound
	}
	lookupKey := latestScopeKey(r.lastScope, defaultBranch)
	if scan, ok := r.previous[lookupKey]; ok {
		clone := *scan
		return &clone, nil
	}
	return nil, model.ErrNotFound
}

func (r *observingScanRepo) ListByProject(_ context.Context, _ int64) ([]*model.Scan, error) {
	return nil, nil
}

func (r *observingScanRepo) ListByProjectInScope(_ context.Context, _ int64, _ model.AnalysisScope, _ string) ([]*model.Scan, error) {
	return nil, nil
}

func (r *observingScanRepo) ResolveDefaultBranch(_ context.Context, _ int64, configured string) (string, bool, error) {
	if configured != "" {
		return configured, false, nil
	}
	return r.defaultBranch, true, nil
}

type queryIssueRepo struct {
	rowsByScanID     map[int64][]*model.IssueRow
	inserted         []model.IssueRow
	lastQueryScanIDs []int64
}

func (r *queryIssueRepo) BulkInsert(_ context.Context, issues []model.IssueRow) error {
	r.inserted = append([]model.IssueRow(nil), issues...)
	return nil
}

func (r *queryIssueRepo) Query(_ context.Context, filter model.IssueFilter) ([]*model.IssueRow, int, error) {
	if filter.ScanID == nil {
		return nil, 0, nil
	}
	r.lastQueryScanIDs = append(r.lastQueryScanIDs, *filter.ScanID)
	rows := r.rowsByScanID[*filter.ScanID]
	clone := make([]*model.IssueRow, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		copyRow := *row
		clone = append(clone, &copyRow)
	}
	return clone, len(clone), nil
}

func (r *queryIssueRepo) Facets(_ context.Context, _, _ int64) (*model.IssueFacets, error) {
	return nil, nil
}

func (r *queryIssueRepo) CountByProject(_ context.Context, _ int64) (int, error) { return 0, nil }

func (r *queryIssueRepo) GetByID(_ context.Context, _ int64) (*model.IssueRow, error) {
	return nil, model.ErrNotFound
}

func (r *queryIssueRepo) Transition(_ context.Context, _, _ int64, _, _, _ string) error {
	return nil
}

type replacingSnapshotRepo struct {
	states map[string]*model.CodeSnapshotState
	calls  int
}

func (r *replacingSnapshotRepo) Replace(_ context.Context, state *model.CodeSnapshotState) error {
	if r.states == nil {
		r.states = map[string]*model.CodeSnapshotState{}
	}
	r.calls++
	r.states[storedScopeKey(state.Scope)] = cloneSnapshotState(state)
	return nil
}

func newTestIssue(ruleKey, path string) *model.Issue {
	issue := model.NewIssue(ruleKey, path, 1)
	issue.Message = "test issue"
	issue.Type = model.TypeCodeSmell
	issue.Severity = model.SeverityMajor
	return issue
}

func toIssueRow(issue *model.Issue, scanID, projectID int64) *model.IssueRow {
	row := domainToIssueRow(issue, scanID, projectID, string(model.IssueTrackingStateUnknown))
	return &row
}

func latestScopeKey(scope model.AnalysisScope, defaultBranch string) string {
	scope = scope.Normalize()
	if scope.Type == model.ScopeTypePullRequest {
		return "pr:" + scope.PullRequestKey
	}
	branch := scope.Branch
	if branch == "" {
		branch = defaultBranch
	}
	if branch == defaultBranch {
		return "legacy:" + defaultBranch
	}
	return "branch:" + branch
}

func storedScopeKey(scope model.AnalysisScope) string {
	scope = scope.Normalize()
	if scope.Type == model.ScopeTypePullRequest {
		return "pull_request:" + scope.PullRequestKey
	}
	return "branch:" + scope.Branch
}

func cloneSnapshotState(state *model.CodeSnapshotState) *model.CodeSnapshotState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.Snapshot.Files = append([]model.CodeSnapshotFile(nil), state.Snapshot.Files...)
	return &clone
}

func firstCreatedBranch(scans []*model.Scan) string {
	if len(scans) == 0 || scans[0] == nil {
		return ""
	}
	return scans[0].Branch
}
