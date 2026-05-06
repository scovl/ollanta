// Package ingest implements the scan ingestion use case.
// It receives a decoded report, persists it via repository ports,
// runs issue tracking and quality gate evaluation, then triggers async search indexing.
package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/scovl/ollanta/application/analysis"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	"github.com/scovl/ollanta/domain/service"
)

// IngestMetadata mirrors the Metadata field of report.Report for JSON decoding.
type IngestMetadata struct {
	ProjectKey      string `json:"project_key"`
	AnalysisDate    string `json:"analysis_date"` // RFC 3339
	Version         string `json:"version"`
	ElapsedMs       int64  `json:"elapsed_ms"`
	ScopeType       string `json:"scope_type,omitempty"`
	Branch          string `json:"branch,omitempty"`
	CommitSHA       string `json:"commit_sha,omitempty"`
	PullRequestKey  string `json:"pull_request_key,omitempty"`
	PullRequestBase string `json:"pull_request_base,omitempty"`
}

// IngestMeasures mirrors the Measures field of report.Report for JSON decoding.
type IngestMeasures struct {
	Files                  int            `json:"files"`
	Lines                  int            `json:"lines"`
	Ncloc                  int            `json:"ncloc"`
	Comments               int            `json:"comments"`
	Bugs                   int            `json:"bugs"`
	CodeSmells             int            `json:"code_smells"`
	Vulnerabilities        int            `json:"vulnerabilities"`
	Coverage               *float64       `json:"coverage,omitempty"`
	Tests                  int            `json:"tests,omitempty"`
	TestFailures           int            `json:"test_failures,omitempty"`
	TestErrors             int            `json:"test_errors,omitempty"`
	TestSkipped            int            `json:"test_skipped,omitempty"`
	TestDurationMs         int64          `json:"test_duration_ms,omitempty"`
	MutationScore          *float64       `json:"mutation_score,omitempty"`
	MutantsTotal           int            `json:"mutants_total,omitempty"`
	MutantsKilled          int            `json:"mutants_killed,omitempty"`
	MutantsSurvived        int            `json:"mutants_survived,omitempty"`
	MutantsTimeout         int            `json:"mutants_timeout,omitempty"`
	MutantsSkipped         int            `json:"mutants_skipped,omitempty"`
	MutantsError           int            `json:"mutants_error,omitempty"`
	ChangedMutationScore   *float64       `json:"changed_mutation_score,omitempty"`
	ChangedMutantsTotal    int            `json:"changed_mutants_total,omitempty"`
	ChangedMutantsKilled   int            `json:"changed_mutants_killed,omitempty"`
	ChangedMutantsSurvived int            `json:"changed_mutants_survived,omitempty"`
	ByLang                 map[string]int `json:"by_language"`
}

// IngestRequest is the payload accepted by POST /api/v1/scans.
// Its JSON shape is identical to the report.json produced by ollantascanner.
type IngestRequest struct {
	Metadata        IngestMetadata          `json:"metadata"`
	ScannerOptions  json.RawMessage         `json:"scanner_options,omitempty"`
	Measures        IngestMeasures          `json:"measures"`
	Issues          []*model.Issue          `json:"issues"`
	QualityProfiles []model.ProfileSnapshot `json:"quality_profiles,omitempty"`
	CodeSnapshot    *model.CodeSnapshot     `json:"code_snapshot,omitempty"`
	TestSignals     json.RawMessage         `json:"test_signals,omitempty"`
}

// IngestResult is the response returned after a successful ingest.
type IngestResult struct {
	ScanID       int64                   `json:"scan_id"`
	ProjectKey   string                  `json:"project_key"`
	GateStatus   string                  `json:"gate_status"`
	GateResult   *model.GateResult       `json:"gate_result,omitempty"`
	TotalIssues  int                     `json:"total_issues"`
	NewIssues    int                     `json:"new_issues"`
	ClosedIssues int                     `json:"closed_issues"`
	Tracking     *service.TrackingResult `json:"tracking"`
}

// ISearchEnqueuer is an optional outbound port for async search indexing.
// Implementations live in the outer layer chosen by the active runtime.
type ISearchEnqueuer interface {
	// Enqueue submits an index job without blocking. Dropping is acceptable.
	Enqueue(ctx context.Context, scanID, projectID int64, projectKey string)
}

// IWebhookDispatcher is an optional outbound port for firing webhooks.
type IWebhookDispatcher interface {
	// Dispatch sends webhook notifications for the given scan event.
	Dispatch(ctx context.Context, projectID, scanID int64, event string) error
}

// IngestUseCase orchestrates the full ingest workflow using domain port interfaces.
type IngestUseCase struct {
	projects      port.IProjectRepo
	scans         port.IScanRepo
	issues        port.IIssueRepo
	measures      port.IMeasureRepo
	snapshots     port.ICodeSnapshotRepo
	profiles      port.IProfileSnapshotRepo
	tags          port.ITagCatalogRepo
	gateEvaluator *analysis.EvaluateGateUseCase
	indexer       ISearchEnqueuer    // optional — nil disables search indexing
	webhooks      IWebhookDispatcher // optional — nil disables webhook dispatch
}

// NewIngestUseCase creates an IngestUseCase with all required dependencies.
// indexer and webhooks may be nil to disable optional features.
func NewIngestUseCase(
	projects port.IProjectRepo,
	scans port.IScanRepo,
	issues port.IIssueRepo,
	measures port.IMeasureRepo,
	snapshots port.ICodeSnapshotRepo,
	indexer ISearchEnqueuer,
	webhooks IWebhookDispatcher,
) *IngestUseCase {
	return &IngestUseCase{
		projects:  projects,
		scans:     scans,
		issues:    issues,
		measures:  measures,
		snapshots: snapshots,
		indexer:   indexer,
		webhooks:  webhooks,
	}
}

// SetProfileSnapshotRepo enables persistence for scan-time quality profile snapshots.
func (uc *IngestUseCase) SetProfileSnapshotRepo(repo port.IProfileSnapshotRepo) {
	uc.profiles = repo
}

// SetTagCatalogRepo enables server-side tag discovery during scan ingestion.
func (uc *IngestUseCase) SetTagCatalogRepo(repo port.ITagCatalogRepo) {
	uc.tags = repo
}

// SetGateEvaluator enables database-backed quality gate evaluation during ingest.
// When nil, falls back to built-in DefaultConditions().
func (uc *IngestUseCase) SetGateEvaluator(evaluator *analysis.EvaluateGateUseCase) {
	uc.gateEvaluator = evaluator
}

func resolveRequestScope(meta IngestMetadata) (model.AnalysisScope, error) {
	scope := model.AnalysisScope{
		Type:            meta.ScopeType,
		Branch:          meta.Branch,
		PullRequestKey:  meta.PullRequestKey,
		PullRequestBase: meta.PullRequestBase,
	}.Normalize()
	if scope.Type == model.ScopeTypePullRequest {
		missing := make([]string, 0, 3)
		if scope.PullRequestKey == "" {
			missing = append(missing, "pull_request_key")
		}
		if scope.Branch == "" {
			missing = append(missing, "branch")
		}
		if scope.PullRequestBase == "" {
			missing = append(missing, "pull_request_base")
		}
		if len(missing) > 0 {
			return model.AnalysisScope{}, fmt.Errorf("pull request scope missing %s", strings.Join(missing, ", "))
		}
	}
	return scope, nil
}

// parseAnalysisDate parses an RFC 3339 string, falling back to UTC now.
func parseAnalysisDate(s string) time.Time {
	if s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return time.Now().UTC()
}

// fetchPrevIssues retrieves issues from the previous scan for tracking purposes.
// Returns nil if prevScan is nil or the query fails.
func (uc *IngestUseCase) fetchPrevIssues(ctx context.Context, projectID int64, prevScan *model.Scan) []*model.Issue {
	if prevScan == nil {
		return nil
	}
	sid := prevScan.ID
	rows, _, err := uc.issues.Query(ctx, model.IssueFilter{
		ProjectID: &projectID,
		ScanID:    &sid,
		Limit:     10000,
	})
	if err != nil {
		return nil
	}
	out := make([]*model.Issue, len(rows))
	for i, r := range rows {
		out[i] = issueRowToDomain(r)
	}
	return out
}

// Ingest persists a scan report and returns a summary of the results.
func (uc *IngestUseCase) Ingest(ctx context.Context, req *IngestRequest) (*IngestResult, error) {
	if req.Metadata.ProjectKey == "" {
		return nil, fmt.Errorf("project_key is required")
	}
	scope, err := resolveRequestScope(req.Metadata)
	if err != nil {
		return nil, err
	}

	analysisDate := parseAnalysisDate(req.Metadata.AnalysisDate)

	// ── 1. Upsert project ────────────────────────────────────────────────────
	project := &model.Project{
		Key:  req.Metadata.ProjectKey,
		Name: req.Metadata.ProjectKey,
	}
	if err := pipelineSteps.upsertProject.run(ctx, func(ctx context.Context) error {
		return uc.projects.Upsert(ctx, project)
	}); err != nil {
		return nil, fmt.Errorf("upsert project: %w", err)
	}

	// ── 2. Fetch previous scan for tracking ──────────────────────────────────
	prevScan := uc.fetchPreviousScan(ctx, project, scope)

	// ── 3. Issue tracking ────────────────────────────────────────────────────
	prevIssues := uc.fetchPrevIssues(ctx, project.ID, prevScan)
	trackResult := service.Track(req.Issues, prevIssues)
	trackingStates := buildTrackingStateMap(trackResult)

	// ── 4. Quality gate evaluation ───────────────────────────────────────────
	measures := map[string]float64{
		model.MetricBugs:            float64(req.Measures.Bugs),
		model.MetricVulnerabilities: float64(req.Measures.Vulnerabilities),
		model.MetricCodeSmells:      float64(req.Measures.CodeSmells),
		model.MetricFiles:           float64(req.Measures.Files),
		model.MetricLines:           float64(req.Measures.Lines),
		model.MetricNcloc:           float64(req.Measures.Ncloc),
	}
	evidence := detectTestSignalEvidence(req.TestSignals)
	addOptionalTestMeasures(measures, req.Measures, evidence)

	newMeasures := computeNewCodeMeasures(measures, prevScan)
	changedLines := computeChangedLines(req.Measures, prevScan)

	var gateStatus *service.GateStatus
	var gateResult *model.GateResult
	if uc.gateEvaluator != nil {
		gateStatus, _ = uc.gateEvaluator.EvaluateForProject(
			ctx, project.ID,
			measures, newMeasures,
			changedLines, 0, // smallChangesetLines comes from the gate
		)
		if gateStatus != nil {
			gateResult = serviceGateToGateResult(gateStatus)
		}
	}
	if gateStatus == nil {
		gateStatus = service.Evaluate(service.DefaultConditions(), measures)
	}
	gateStr := string(gateStatus.Status)

	// ── 5. Insert scan ───────────────────────────────────────────────────────
	scan := &model.Scan{
		ProjectID:            project.ID,
		Version:              req.Metadata.Version,
		ScopeType:            scope.Type,
		Branch:               scope.Branch,
		CommitSHA:            req.Metadata.CommitSHA,
		PullRequestKey:       scope.PullRequestKey,
		PullRequestBase:      scope.PullRequestBase,
		Status:               "completed",
		ElapsedMs:            req.Metadata.ElapsedMs,
		GateStatus:           gateStr,
		GateResult:           gateResult,
		AnalysisDate:         analysisDate,
		TotalFiles:           req.Measures.Files,
		TotalLines:           req.Measures.Lines,
		TotalNcloc:           req.Measures.Ncloc,
		TotalComments:        req.Measures.Comments,
		TotalIssues:          len(req.Issues),
		TotalBugs:            req.Measures.Bugs,
		TotalCodeSmells:      req.Measures.CodeSmells,
		TotalVulnerabilities: req.Measures.Vulnerabilities,
		NewIssues:            trackResult.NewCount(),
		ClosedIssues:         trackResult.ClosedCount(),
	}
	if err := pipelineSteps.insertScan.run(ctx, func(ctx context.Context) error {
		return uc.scans.Create(ctx, scan)
	}); err != nil {
		return nil, fmt.Errorf("create scan: %w", err)
	}

	// ── 6. Bulk insert issues ────────────────────────────────────────────────
	issueRows := make([]model.IssueRow, len(req.Issues))
	for i, iss := range req.Issues {
		issueRows[i] = domainToIssueRow(iss, scan.ID, project.ID, trackingStates[iss])
	}
	if err := pipelineSteps.bulkInsIssues.run(ctx, func(ctx context.Context) error {
		return uc.issues.BulkInsert(ctx, issueRows)
	}); err != nil {
		return nil, fmt.Errorf("bulk insert issues: %w", err)
	}
	if err := uc.discoverIssueTags(ctx, req.Issues); err != nil {
		return nil, fmt.Errorf("discover tags: %w", err)
	}

	// ── 7. Bulk insert measures ──────────────────────────────────────────────
	measureRows := buildMeasureRows(req.Measures, req.TestSignals, scan.ID, project.ID)
	if err := pipelineSteps.bulkInsMeasures.run(ctx, func(ctx context.Context) error {
		return uc.measures.BulkInsert(ctx, measureRows)
	}); err != nil {
		return nil, fmt.Errorf("bulk insert measures: %w", err)
	}

	// ── 7.5. Upsert live measures for fast current-value queries ──────────
	uc.upsertLiveMeasures(ctx, project.ID, scan.ID, measures)

	// ── 7.6. Upsert daily rollup for trend queries ────────────────────────
	uc.upsertDailyRollup(ctx, project.ID, measures)

	// ── 8. Persist latest code/profile snapshots for the scope ──────────────
	if err := uc.persistScanArtifacts(ctx, project.ID, scan.ID, scope, req); err != nil {
		return nil, err
	}

	// ── 9. Async: enqueue search indexing ────────────────────────────────────
	uc.enqueueSearchIndex(ctx, scan.ID, project.ID, project.Key)

	// ── 10. Fire webhooks ────────────────────────────────────────────────────
	uc.dispatchScanWebhook(ctx, project.ID, scan.ID)
	uc.dispatchGateChanged(ctx, project.ID, scan.ID, prevScan, gateStr)

	return &IngestResult{
		ScanID:       scan.ID,
		ProjectKey:   project.Key,
		GateStatus:   gateStr,
		GateResult:   gateResult,
		TotalIssues:  len(req.Issues),
		NewIssues:    trackResult.NewCount(),
		ClosedIssues: trackResult.ClosedCount(),
		Tracking:     trackResult,
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (uc *IngestUseCase) fetchPreviousScan(ctx context.Context, project *model.Project, scope model.AnalysisScope) *model.Scan {
	var prevScan *model.Scan
	_ = pipelineSteps.fetchPrevScan.run(ctx, func(ctx context.Context) error {
		defaultBranch, _, err := uc.scans.ResolveDefaultBranch(ctx, project.ID, project.MainBranch)
		if err != nil {
			return err
		}
		prevScan, err = uc.scans.GetLatestInScope(ctx, project.ID, scope, defaultBranch)
		return err
	})
	return prevScan
}

func (uc *IngestUseCase) discoverIssueTags(ctx context.Context, issues []*model.Issue) error {
	if uc.tags == nil || len(issues) == 0 {
		return nil
	}
	var tags []string
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		tags = append(tags, issue.Tags...)
		for _, category := range model.SecurityCategories(issue.Tags) {
			tags = append(tags, category)
		}
	}
	return uc.tags.DiscoverTags(ctx, tags, model.TagSourceScan)
}

func (uc *IngestUseCase) persistScanArtifacts(ctx context.Context, projectID, scanID int64, scope model.AnalysisScope, req *IngestRequest) error {
	if uc.snapshots != nil && req.CodeSnapshot != nil {
		if err := uc.snapshots.Replace(ctx, &model.CodeSnapshotState{
			ProjectID: projectID,
			ScanID:    scanID,
			Scope:     scope,
			Snapshot:  *req.CodeSnapshot,
		}); err != nil {
			return fmt.Errorf("persist code snapshot: %w", err)
		}
	}
	if uc.profiles == nil {
		return nil
	}
	if err := uc.profiles.Replace(ctx, projectID, scanID, scope, normalizeProfileSnapshots(req.QualityProfiles)); err != nil {
		return fmt.Errorf("persist quality profile snapshots: %w", err)
	}
	return nil
}

func (uc *IngestUseCase) enqueueSearchIndex(ctx context.Context, scanID, projectID int64, projectKey string) {
	if uc.indexer == nil {
		return
	}
	_ = pipelineSteps.indexSearch.run(ctx, func(ctx context.Context) error {
		uc.indexer.Enqueue(ctx, scanID, projectID, projectKey)
		return nil
	})
}

func (uc *IngestUseCase) dispatchScanWebhook(ctx context.Context, projectID, scanID int64) {
	if uc.webhooks == nil {
		return
	}
	_ = pipelineSteps.fireWebhooks.run(ctx, func(ctx context.Context) error {
		return uc.webhooks.Dispatch(ctx, projectID, scanID, "scan.completed")
	})
}

func (uc *IngestUseCase) upsertLiveMeasures(ctx context.Context, projectID, scanID int64, measures map[string]float64) {
	_ = uc.measures.UpsertLiveBatch(ctx, projectID, scanID, measures)
}

func (uc *IngestUseCase) upsertDailyRollup(ctx context.Context, projectID int64, measures map[string]float64) {
	today := time.Now().UTC().Format("2006-01-02")
	_ = uc.measures.UpsertDailyAggregateBatch(ctx, projectID, today, measures)
}

func issueRowToDomain(r *model.IssueRow) *model.Issue {
	return &model.Issue{
		RuleKey:       r.RuleKey,
		ComponentPath: r.ComponentPath,
		Line:          r.Line,
		Column:        r.Column,
		EndLine:       r.EndLine,
		EndColumn:     r.EndColumn,
		Message:       r.Message,
		Type:          model.IssueType(r.Type),
		Severity:      model.Severity(r.Severity),
		QualityDomain: model.IssueQualityDomain(r.QualityDomain),
		Language:      r.Language,
		Status:        model.Status(r.Status),
		Resolution:    r.Resolution,
		EffortMinutes: r.EffortMinutes,
		LineHash:      r.LineHash,
		Tags:          r.Tags,
	}
}

func buildTrackingStateMap(result *service.TrackingResult) map[*model.Issue]string {
	tracking := make(map[*model.Issue]string)
	if result == nil {
		return tracking
	}
	for _, issue := range result.New {
		tracking[issue] = string(model.IssueTrackingStateNew)
	}
	for _, pair := range result.Unchanged {
		tracking[pair.Current] = string(model.IssueTrackingStateUnchanged)
	}
	for _, pair := range result.Reopened {
		tracking[pair.Current] = string(model.IssueTrackingStateReopened)
	}
	return tracking
}

func normalizeProfileSnapshots(snapshots []model.ProfileSnapshot) []model.ProfileSnapshot {
	if len(snapshots) == 0 {
		return nil
	}
	out := make([]model.ProfileSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.Language == "" {
			continue
		}
		snapshot.MetadataAvailable = true
		out = append(out, snapshot)
	}
	return out
}

func domainToIssueRow(iss *model.Issue, scanID, projectID int64, trackingState string) model.IssueRow {
	tags := iss.Tags
	if tags == nil {
		tags = []string{}
	}
	if trackingState == "" {
		trackingState = string(model.IssueTrackingStateUnknown)
	}
	return model.IssueRow{
		ScanID:        scanID,
		ProjectID:     projectID,
		RuleKey:       iss.RuleKey,
		ComponentPath: iss.ComponentPath,
		Line:          iss.Line,
		Column:        iss.Column,
		EndLine:       iss.EndLine,
		EndColumn:     iss.EndColumn,
		Message:       iss.Message,
		Type:          string(iss.Type),
		Severity:      string(iss.Severity),
		QualityDomain: string(issueQualityDomain(iss)),
		Language:      issueLanguage(iss),
		Status:        string(iss.Status),
		Resolution:    iss.Resolution,
		TrackingState: trackingState,
		EffortMinutes: iss.EffortMinutes,
		LineHash:      iss.LineHash,
		Tags:          tags,
	}
}

func buildMeasureRows(m IngestMeasures, testSignals json.RawMessage, scanID, projectID int64) []model.MeasureRow {
	evidence := detectTestSignalEvidence(testSignals)
	rows := []model.MeasureRow{
		{ScanID: scanID, ProjectID: projectID, MetricKey: "files", Value: float64(m.Files)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "lines", Value: float64(m.Lines)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "ncloc", Value: float64(m.Ncloc)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "comments", Value: float64(m.Comments)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "bugs", Value: float64(m.Bugs)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "code_smells", Value: float64(m.CodeSmells)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "vulnerabilities", Value: float64(m.Vulnerabilities)},
	}
	if m.Coverage != nil {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricCoverage, Value: *m.Coverage})
	}
	for _, measure := range optionalTestMeasureValues(m, evidence) {
		if measure.Include {
			rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: measure.Key, Value: measure.Value})
		}
	}
	for lang, count := range m.ByLang {
		rows = append(rows, model.MeasureRow{
			ScanID:        scanID,
			ProjectID:     projectID,
			MetricKey:     "files_by_lang_" + lang,
			ComponentPath: lang,
			Value:         float64(count),
		})
	}
	rows = append(rows, buildCoverageFileMeasureRows(testSignals, scanID, projectID)...)
	return rows
}

type ingestTestSignalReport struct {
	Modules []ingestTestModuleSignal `json:"modules"`
}

type ingestTestModuleSignal struct {
	Files []ingestTestFileCoverage `json:"files"`
}

type ingestTestFileCoverage struct {
	Path           string `json:"path"`
	LinesToCover   int    `json:"lines_to_cover"`
	CoveredLines   int    `json:"covered_lines"`
	UncoveredLines []int  `json:"uncovered_lines"`
}

func buildCoverageFileMeasureRows(testSignals json.RawMessage, scanID, projectID int64) []model.MeasureRow {
	if len(testSignals) == 0 {
		return nil
	}
	var report ingestTestSignalReport
	if err := json.Unmarshal(testSignals, &report); err != nil {
		return nil
	}
	var rows []model.MeasureRow
	for _, module := range report.Modules {
		for _, file := range module.Files {
			if file.Path == "" || file.LinesToCover <= 0 {
				continue
			}
			coverage := float64(file.CoveredLines) * 100 / float64(file.LinesToCover)
			rows = append(rows,
				model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricCoverage, ComponentPath: file.Path, Value: coverage},
				model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricLinesToCover, ComponentPath: file.Path, Value: float64(file.LinesToCover)},
				model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricCoveredLines, ComponentPath: file.Path, Value: float64(file.CoveredLines)},
				model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricUncoveredLines, ComponentPath: file.Path, Value: float64(len(file.UncoveredLines))},
			)
		}
	}
	return rows
}

func issueQualityDomain(issue *model.Issue) model.IssueQualityDomain {
	if issue.QualityDomain != "" {
		return issue.QualityDomain
	}
	return model.DeriveIssueQualityDomain(issue.Type, issue.Tags)
}

func issueLanguage(issue *model.Issue) string {
	if issue.Language != "" {
		return issue.Language
	}
	return model.LanguageFromPath(issue.ComponentPath)
}

func addOptionalTestMeasures(measures map[string]float64, m IngestMeasures, evidence testSignalEvidence) {
	if m.Coverage != nil {
		measures[model.MetricCoverage] = *m.Coverage
	}
	for _, measure := range optionalTestMeasureValues(m, evidence) {
		if measure.Include {
			measures[measure.Key] = measure.Value
		}
	}
}

type optionalMeasureValue struct {
	Key     string
	Value   float64
	Include bool
}

func optionalTestMeasureValues(m IngestMeasures, evidence testSignalEvidence) []optionalMeasureValue {
	values := []optionalMeasureValue{
		{Key: model.MetricTests, Value: float64(m.Tests), Include: metricEvidenceAvailable(evidence.Tests, float64(m.Tests))},
		{Key: model.MetricTestFailures, Value: float64(m.TestFailures), Include: metricEvidenceAvailable(evidence.Tests, float64(m.TestFailures))},
		{Key: model.MetricTestErrors, Value: float64(m.TestErrors), Include: metricEvidenceAvailable(evidence.Tests, float64(m.TestErrors))},
		{Key: model.MetricTestSkipped, Value: float64(m.TestSkipped), Include: metricEvidenceAvailable(evidence.Tests, float64(m.TestSkipped))},
		{Key: model.MetricTestDurationMs, Value: float64(m.TestDurationMs), Include: metricEvidenceAvailable(evidence.Tests, float64(m.TestDurationMs))},
		{Key: model.MetricMutantsTotal, Value: float64(m.MutantsTotal), Include: metricEvidenceAvailable(evidence.Mutation, float64(m.MutantsTotal))},
		{Key: model.MetricMutantsKilled, Value: float64(m.MutantsKilled), Include: metricEvidenceAvailable(evidence.Mutation, float64(m.MutantsKilled))},
		{Key: model.MetricMutantsSurvived, Value: float64(m.MutantsSurvived), Include: metricEvidenceAvailable(evidence.Mutation, float64(m.MutantsSurvived))},
		{Key: model.MetricMutantsTimeout, Value: float64(m.MutantsTimeout), Include: metricEvidenceAvailable(evidence.Mutation, float64(m.MutantsTimeout))},
		{Key: model.MetricMutantsSkipped, Value: float64(m.MutantsSkipped), Include: metricEvidenceAvailable(evidence.Mutation, float64(m.MutantsSkipped))},
		{Key: model.MetricMutantsError, Value: float64(m.MutantsError), Include: metricEvidenceAvailable(evidence.Mutation, float64(m.MutantsError))},
		{Key: model.MetricChangedMutantsTotal, Value: float64(m.ChangedMutantsTotal), Include: metricEvidenceAvailable(evidence.ChangedMutation, float64(m.ChangedMutantsTotal))},
		{Key: model.MetricChangedMutantsKilled, Value: float64(m.ChangedMutantsKilled), Include: metricEvidenceAvailable(evidence.ChangedMutation, float64(m.ChangedMutantsKilled))},
		{Key: model.MetricChangedMutantsSurvived, Value: float64(m.ChangedMutantsSurvived), Include: metricEvidenceAvailable(evidence.ChangedMutation, float64(m.ChangedMutantsSurvived))},
	}
	if m.MutationScore != nil {
		values = append(values, optionalMeasureValue{Key: model.MetricMutationScore, Value: *m.MutationScore, Include: true})
	}
	if m.ChangedMutationScore != nil {
		values = append(values, optionalMeasureValue{Key: model.MetricChangedMutationScore, Value: *m.ChangedMutationScore, Include: true})
	}
	return values
}

func metricEvidenceAvailable(hasEvidence bool, value float64) bool {
	return hasEvidence || value > 0
}

type testSignalEvidence struct {
	Tests           bool
	Mutation        bool
	ChangedMutation bool
}

func detectTestSignalEvidence(raw json.RawMessage) testSignalEvidence {
	if len(raw) == 0 {
		return testSignalEvidence{}
	}
	var payload struct {
		Summary struct {
			Tests                  int      `json:"tests"`
			MutationScore          *float64 `json:"mutation_score"`
			MutantsTotal           int      `json:"mutants_total"`
			ChangedMutationScore   *float64 `json:"changed_mutation_score"`
			ChangedMutantsTotal    int      `json:"changed_mutants_total"`
			ChangedMutantsKilled   int      `json:"changed_mutants_killed"`
			ChangedMutantsSurvived int      `json:"changed_mutants_survived"`
		} `json:"summary"`
		Modules []struct {
			Suites   []json.RawMessage `json:"suites"`
			Mutation *json.RawMessage  `json:"mutation"`
			Reports  []struct {
				Kind string `json:"kind"`
			} `json:"reports"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return testSignalEvidence{}
	}
	evidence := testSignalEvidence{
		Tests:           payload.Summary.Tests > 0,
		Mutation:        payload.Summary.MutationScore != nil || payload.Summary.MutantsTotal > 0,
		ChangedMutation: payload.Summary.ChangedMutationScore != nil || payload.Summary.ChangedMutantsTotal > 0 || payload.Summary.ChangedMutantsKilled > 0 || payload.Summary.ChangedMutantsSurvived > 0,
	}
	for _, module := range payload.Modules {
		if len(module.Suites) > 0 {
			evidence.Tests = true
		}
		if module.Mutation != nil {
			evidence.Mutation = true
		}
		for _, report := range module.Reports {
			switch report.Kind {
			case "test", "native":
				evidence.Tests = true
			case "mutation", "native_mutation":
				evidence.Mutation = true
			}
		}
	}
	return evidence
}

// ── gate helpers ─────────────────────────────────────────────────────────────

// computeNewCodeMeasures computes new-code metrics by subtracting the previous
// scan baseline from the current scan measures. Cumulative metrics (bugs,
// vulnerabilities, code_smells) use (current - previous). Relative metrics
// (coverage, mutation_score) use the current value as-is. If prevScan is nil,
// all current values are treated as new.
func computeNewCodeMeasures(current map[string]float64, prevScan *model.Scan) map[string]float64 {
	newM := make(map[string]float64, len(current))
	for k, v := range current {
		newM[k] = v
	}
	if prevScan == nil {
		return newM
	}
	prevMeasures := analysis.MeasuresFromScan(prevScan)
	cumulativeKeys := map[string]bool{
		model.MetricBugs:            true,
		model.MetricVulnerabilities: true,
		model.MetricCodeSmells:      true,
	}
	for k := range current {
		if cumulativeKeys[k] {
			if prev, ok := prevMeasures[k]; ok {
				diff := current[k] - prev
				if diff < 0 {
					diff = 0
				}
				newM[k] = diff
			}
		}
	}
	return newM
}

// computeChangedLines returns the number of changed lines compared to the
// previous scan, for small-changeset detection. If prevScan is nil, returns
// the current total lines.
func computeChangedLines(m IngestMeasures, prevScan *model.Scan) int {
	if prevScan == nil {
		return m.Lines
	}
	changed := m.Lines - prevScan.TotalLines
	if changed < 0 {
		changed = -changed
	}
	return changed
}

func serviceGateToGateResult(gs *service.GateStatus) *model.GateResult {
	if gs == nil {
		return nil
	}
	conditions := make([]model.GateConditionEval, len(gs.Conditions))
	for i, c := range gs.Conditions {
		conditions[i] = model.GateConditionEval{
			Metric:    c.Condition.MetricKey,
			Operator:  string(c.Condition.Operator),
			Threshold: c.Condition.ErrorThreshold,
			Actual:    c.ActualValue,
			HasValue:  c.HasValue,
			Status:    string(c.Status),
		}
	}
	return &model.GateResult{
		Status:      string(gs.Status),
		Conditions:  conditions,
		EvaluatedAt: time.Now().UTC(),
	}
}

func (uc *IngestUseCase) dispatchGateChanged(ctx context.Context, projectID, scanID int64, prevScan *model.Scan, gateStr string) {
	if uc.webhooks == nil {
		return
	}
	prevStatus := ""
	if prevScan != nil {
		prevStatus = prevScan.GateStatus
	}
	if prevStatus != gateStr {
		_ = pipelineSteps.fireWebhooks.run(ctx, func(ctx context.Context) error {
			return uc.webhooks.Dispatch(ctx, projectID, scanID, "gate.changed")
		})
	}
}
