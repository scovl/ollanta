// Package ingest implements the scan ingestion use case.
// It receives a decoded report, persists it via repository ports,
// runs issue tracking and quality gate evaluation, then triggers async search indexing.
package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	"github.com/scovl/ollanta/domain/service"
)

// IngestMetadata mirrors the Metadata field of report.Report for JSON decoding.
type IngestMetadata struct {
	ProjectKey   string `json:"project_key"`
	AnalysisDate string `json:"analysis_date"` // RFC 3339
	Version      string `json:"version"`
	ElapsedMs    int64  `json:"elapsed_ms"`
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
	ByLang          map[string]int `json:"by_language"`
}

// IngestRequest is the payload accepted by POST /api/v1/scans.
// Its JSON shape is identical to the report.json produced by ollantascanner.
type IngestRequest struct {
	Metadata IngestMetadata `json:"metadata"`
	Measures IngestMeasures `json:"measures"`
	Issues   []*model.Issue `json:"issues"`
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
	Enqueue(scanID, projectID int64, projectKey string)
}

// IWebhookDispatcher is an optional outbound port for firing webhooks.
type IWebhookDispatcher interface {
	// Dispatch sends webhook notifications for the given scan event.
	Dispatch(ctx context.Context, projectID, scanID int64, event string) error
}

// IngestUseCase orchestrates the full ingest workflow using domain port interfaces.
type IngestUseCase struct {
	projects port.IProjectRepo
	scans    port.IScanRepo
	issues   port.IIssueRepo
	measures port.IMeasureRepo
	indexer  ISearchEnqueuer    // optional — nil disables search indexing
	webhooks IWebhookDispatcher // optional — nil disables webhook dispatch
}

// NewIngestUseCase creates an IngestUseCase with all required dependencies.
// indexer and webhooks may be nil to disable optional features.
func NewIngestUseCase(
	projects port.IProjectRepo,
	scans port.IScanRepo,
	issues port.IIssueRepo,
	measures port.IMeasureRepo,
	indexer ISearchEnqueuer,
	webhooks IWebhookDispatcher,
) *IngestUseCase {
	return &IngestUseCase{
		projects: projects,
		scans:    scans,
		issues:   issues,
		measures: measures,
		indexer:  indexer,
		webhooks: webhooks,
	}
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
		var err error
		prevScan, err = uc.scans.GetLatest(ctx, project.ID)
		return err
	})

	// ── 3. Issue tracking ────────────────────────────────────────────────────
	prevIssues := uc.fetchPrevIssues(ctx, project.ID, prevScan)
	trackResult := service.Track(req.Issues, prevIssues)

	// ── 4. Quality gate evaluation ───────────────────────────────────────────
	measures := map[string]float64{
		"bugs":            float64(req.Measures.Bugs),
		"vulnerabilities": float64(req.Measures.Vulnerabilities),
		"code_smells":     float64(req.Measures.CodeSmells),
		"files":           float64(req.Measures.Files),
		"lines":           float64(req.Measures.Lines),
		"ncloc":           float64(req.Measures.Ncloc),
	}
	gateStatus := service.Evaluate(service.DefaultConditions(), measures)
	gateStr := string(gateStatus.Status)

	// ── 5. Insert scan ───────────────────────────────────────────────────────
	scan := &model.Scan{
		ProjectID:            project.ID,
		Version:              req.Metadata.Version,
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
		issueRows[i] = domainToIssueRow(iss, scan.ID, project.ID)
	}
	if err := pipelineSteps.bulkInsIssues.run(ctx, func(ctx context.Context) error {
		return uc.issues.BulkInsert(ctx, issueRows)
	}); err != nil {
		return nil, fmt.Errorf("bulk insert issues: %w", err)
	}

	// ── 7. Bulk insert measures ──────────────────────────────────────────────
	measureRows := buildMeasureRows(req.Measures, scan.ID, project.ID)
	if err := pipelineSteps.bulkInsMeasures.run(ctx, func(ctx context.Context) error {
		return uc.measures.BulkInsert(ctx, measureRows)
	}); err != nil {
		return nil, fmt.Errorf("bulk insert measures: %w", err)
	}

	// ── 8. Async: enqueue search indexing ────────────────────────────────────
	if uc.indexer != nil {
		_ = pipelineSteps.indexSearch.run(ctx, func(_ context.Context) error {
			uc.indexer.Enqueue(scan.ID, project.ID, project.Key)
			return nil
		})
	}

	// ── 9. Fire webhooks ─────────────────────────────────────────────────────
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
		Status:        model.Status(r.Status),
		Resolution:    r.Resolution,
		EffortMinutes: r.EffortMinutes,
		LineHash:      r.LineHash,
		Tags:          r.Tags,
	}
}

func domainToIssueRow(iss *model.Issue, scanID, projectID int64) model.IssueRow {
	tags := iss.Tags
	if tags == nil {
		tags = []string{}
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
		Status:        string(iss.Status),
		Resolution:    iss.Resolution,
		EffortMinutes: iss.EffortMinutes,
		LineHash:      iss.LineHash,
		Tags:          tags,
	}
}

func buildMeasureRows(m IngestMeasures, scanID, projectID int64) []model.MeasureRow {
	rows := []model.MeasureRow{
		{ScanID: scanID, ProjectID: projectID, MetricKey: "files", Value: float64(m.Files)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "lines", Value: float64(m.Lines)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "ncloc", Value: float64(m.Ncloc)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "comments", Value: float64(m.Comments)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "bugs", Value: float64(m.Bugs)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "code_smells", Value: float64(m.CodeSmells)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "vulnerabilities", Value: float64(m.Vulnerabilities)},
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
	return rows
}
