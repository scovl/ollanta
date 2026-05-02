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
	Files           int            `json:"files"`
	Lines           int            `json:"lines"`
	Ncloc           int            `json:"ncloc"`
	Comments        int            `json:"comments"`
	Bugs            int            `json:"bugs"`
	CodeSmells      int            `json:"code_smells"`
	Vulnerabilities int            `json:"vulnerabilities"`
	Coverage        *float64       `json:"coverage,omitempty"`
	Tests           int            `json:"tests,omitempty"`
	TestFailures    int            `json:"test_failures,omitempty"`
	TestErrors      int            `json:"test_errors,omitempty"`
	TestSkipped     int            `json:"test_skipped,omitempty"`
	TestDurationMs  int64          `json:"test_duration_ms,omitempty"`
	MutationScore   *float64       `json:"mutation_score,omitempty"`
	MutantsTotal    int            `json:"mutants_total,omitempty"`
	MutantsKilled   int            `json:"mutants_killed,omitempty"`
	MutantsSurvived int            `json:"mutants_survived,omitempty"`
	MutantsTimeout  int            `json:"mutants_timeout,omitempty"`
	MutantsError    int            `json:"mutants_error,omitempty"`
	ByLang          map[string]int `json:"by_language"`
}

// IngestRequest is the payload accepted by POST /api/v1/scans.
// Its JSON shape is identical to the report.json produced by ollantascanner.
type IngestRequest struct {
	Metadata       IngestMetadata      `json:"metadata"`
	ScannerOptions json.RawMessage     `json:"scanner_options,omitempty"`
	Measures       IngestMeasures      `json:"measures"`
	Issues         []*model.Issue      `json:"issues"`
	CodeSnapshot   *model.CodeSnapshot `json:"code_snapshot,omitempty"`
	TestSignals    json.RawMessage     `json:"test_signals,omitempty"`
}

// IngestResult is the response returned after a successful ingest.
type IngestResult struct {
	ScanID       int64                   `json:"scan_id"`
	ProjectKey   string                  `json:"project_key"`
	GateStatus   string                  `json:"gate_status"`
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
	projects  port.IProjectRepo
	scans     port.IScanRepo
	issues    port.IIssueRepo
	measures  port.IMeasureRepo
	snapshots port.ICodeSnapshotRepo
	indexer   ISearchEnqueuer    // optional — nil disables search indexing
	webhooks  IWebhookDispatcher // optional — nil disables webhook dispatch
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
	var prevScan *model.Scan
	_ = pipelineSteps.fetchPrevScan.run(ctx, func(ctx context.Context) error {
		defaultBranch, _, err := uc.scans.ResolveDefaultBranch(ctx, project.ID, project.MainBranch)
		if err != nil {
			return err
		}
		prevScan, err = uc.scans.GetLatestInScope(ctx, project.ID, scope, defaultBranch)
		return err
	})

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
	addOptionalTestMeasures(measures, req.Measures)
	gateStatus := service.Evaluate(service.DefaultConditions(), measures)
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

	// ── 7. Bulk insert measures ──────────────────────────────────────────────
	measureRows := buildMeasureRows(req.Measures, req.TestSignals, scan.ID, project.ID)
	if err := pipelineSteps.bulkInsMeasures.run(ctx, func(ctx context.Context) error {
		return uc.measures.BulkInsert(ctx, measureRows)
	}); err != nil {
		return nil, fmt.Errorf("bulk insert measures: %w", err)
	}

	// ── 8. Persist latest code snapshot for the scope ───────────────────────
	if uc.snapshots != nil && req.CodeSnapshot != nil {
		if err := uc.snapshots.Replace(ctx, &model.CodeSnapshotState{
			ProjectID: project.ID,
			ScanID:    scan.ID,
			Scope:     scope,
			Snapshot:  *req.CodeSnapshot,
		}); err != nil {
			return nil, fmt.Errorf("persist code snapshot: %w", err)
		}
	}

	// ── 9. Async: enqueue search indexing ────────────────────────────────────
	if uc.indexer != nil {
		_ = pipelineSteps.indexSearch.run(ctx, func(ctx context.Context) error {
			uc.indexer.Enqueue(ctx, scan.ID, project.ID, project.Key)
			return nil
		})
	}

	// ── 10. Fire webhooks ────────────────────────────────────────────────────
	if uc.webhooks != nil {
		_ = pipelineSteps.fireWebhooks.run(ctx, func(ctx context.Context) error {
			return uc.webhooks.Dispatch(ctx, project.ID, scan.ID, "scan.completed")
		})
	}

	return &IngestResult{
		ScanID:       scan.ID,
		ProjectKey:   project.Key,
		GateStatus:   gateStr,
		TotalIssues:  len(req.Issues),
		NewIssues:    trackResult.NewCount(),
		ClosedIssues: trackResult.ClosedCount(),
		Tracking:     trackResult,
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

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
	if m.Tests > 0 {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricTests, Value: float64(m.Tests)})
	}
	if m.TestFailures > 0 {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricTestFailures, Value: float64(m.TestFailures)})
	}
	if m.TestErrors > 0 {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricTestErrors, Value: float64(m.TestErrors)})
	}
	if m.TestSkipped > 0 {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricTestSkipped, Value: float64(m.TestSkipped)})
	}
	if m.TestDurationMs > 0 {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricTestDurationMs, Value: float64(m.TestDurationMs)})
	}
	if m.MutationScore != nil {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricMutationScore, Value: *m.MutationScore})
	}
	if m.MutantsTotal > 0 {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricMutantsTotal, Value: float64(m.MutantsTotal)})
	}
	if m.MutantsKilled > 0 {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricMutantsKilled, Value: float64(m.MutantsKilled)})
	}
	if m.MutantsSurvived > 0 {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricMutantsSurvived, Value: float64(m.MutantsSurvived)})
	}
	if m.MutantsTimeout > 0 {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricMutantsTimeout, Value: float64(m.MutantsTimeout)})
	}
	if m.MutantsError > 0 {
		rows = append(rows, model.MeasureRow{ScanID: scanID, ProjectID: projectID, MetricKey: model.MetricMutantsError, Value: float64(m.MutantsError)})
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

func addOptionalTestMeasures(measures map[string]float64, m IngestMeasures) {
	if m.Coverage != nil {
		measures[model.MetricCoverage] = *m.Coverage
	}
	if m.Tests > 0 {
		measures[model.MetricTests] = float64(m.Tests)
	}
	if m.TestFailures > 0 {
		measures[model.MetricTestFailures] = float64(m.TestFailures)
	}
	if m.TestErrors > 0 {
		measures[model.MetricTestErrors] = float64(m.TestErrors)
	}
	if m.TestSkipped > 0 {
		measures[model.MetricTestSkipped] = float64(m.TestSkipped)
	}
	if m.TestDurationMs > 0 {
		measures[model.MetricTestDurationMs] = float64(m.TestDurationMs)
	}
	if m.MutationScore != nil {
		measures[model.MetricMutationScore] = *m.MutationScore
	}
	if m.MutantsTotal > 0 {
		measures[model.MetricMutantsTotal] = float64(m.MutantsTotal)
	}
	if m.MutantsKilled > 0 {
		measures[model.MetricMutantsKilled] = float64(m.MutantsKilled)
	}
	if m.MutantsSurvived > 0 {
		measures[model.MetricMutantsSurvived] = float64(m.MutantsSurvived)
	}
	if m.MutantsTimeout > 0 {
		measures[model.MetricMutantsTimeout] = float64(m.MutantsTimeout)
	}
	if m.MutantsError > 0 {
		measures[model.MetricMutantsError] = float64(m.MutantsError)
	}
}
