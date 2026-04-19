// Package ingest implements the scan ingestion pipeline.
// It receives a decoded report, persists it transactionally to PostgreSQL,
// runs issue tracking and quality gate evaluation, then enqueues async search indexing.
package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantaengine/qualitygate"
	"github.com/scovl/ollanta/ollantaengine/tracking"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
	"github.com/scovl/ollanta/ollantaweb/breaker"
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
	Metadata IngestMetadata  `json:"metadata"`
	Measures IngestMeasures  `json:"measures"`
	Issues   []*domain.Issue `json:"issues"`
}

// IngestResult is the response returned after a successful ingest.
type IngestResult struct {
	ScanID       int64                    `json:"scan_id"`
	ProjectKey   string                   `json:"project_key"`
	GateStatus   string                   `json:"gate_status"`
	TotalIssues  int                      `json:"total_issues"`
	NewIssues    int                      `json:"new_issues"`
	ClosedIssues int                      `json:"closed_issues"`
	Tracking     *tracking.TrackingResult `json:"tracking"`
}

// IndexEnqueuer abstracts the mechanism for enqueuing search index jobs.
// Implementations include the in-process Worker (for single-instance deploys)
// and pgnotify.Coordinator (for multi-replica K8s deploys).
type IndexEnqueuer interface {
	Enqueue(ctx context.Context, scanID, projectID int64, projectKey string) error
}

// Pipeline orchestrates the full ingest workflow.
type Pipeline struct {
	projects  *postgres.ProjectRepository
	scans     *postgres.ScanRepository
	issues    *postgres.IssueRepository
	measures  *postgres.MeasureRepository
	indexer   search.IIndexer
	enqueuer  IndexEnqueuer
	dbBreaker *breaker.Breaker
	msBreaker *breaker.Breaker
}

// NewPipeline creates an ingest pipeline with all required dependencies.
// enqueuer may be nil to disable async search indexing.
func NewPipeline(
	_ *postgres.DB,
	projects *postgres.ProjectRepository,
	scans *postgres.ScanRepository,
	issues *postgres.IssueRepository,
	measures *postgres.MeasureRepository,
	indexer search.IIndexer,
	enqueuer IndexEnqueuer,
) *Pipeline {
	return &Pipeline{
		projects:  projects,
		scans:     scans,
		issues:    issues,
		measures:  measures,
		indexer:   indexer,
		enqueuer:  enqueuer,
		dbBreaker: breaker.New(5, 30*time.Second, 1),
		msBreaker: breaker.New(5, 30*time.Second, 1),
	}
}

// Ingest persists a scan report and returns a summary of the results.
// Steps 1–9 run inside a single PostgreSQL transaction; step 11 is async.
func (p *Pipeline) Ingest(ctx context.Context, req *IngestRequest) (*IngestResult, error) {
	if req.Metadata.ProjectKey == "" {
		return nil, fmt.Errorf("project_key is required")
	}

	analysisDate := time.Now().UTC()
	if req.Metadata.AnalysisDate != "" {
		if t, err := time.Parse(time.RFC3339, req.Metadata.AnalysisDate); err == nil {
			analysisDate = t
		}
	}

	// ── 1. Upsert project ────────────────────────────────────────────────────
	project := &postgres.Project{
		Key:  req.Metadata.ProjectKey,
		Name: req.Metadata.ProjectKey,
	}
	if err := pipelineSteps.upsertProject.run(ctx, func(ctx context.Context) error {
		return p.dbBreaker.Do(func() error {
			return p.projects.Upsert(ctx, project)
		})
	}); err != nil {
		return nil, fmt.Errorf("upsert project: %w", err)
	}

	// ── 2. Fetch previous scan for tracking ──────────────────────────────────
	var prevScan *postgres.Scan
	_ = pipelineSteps.fetchPrevScan.run(ctx, func(ctx context.Context) error {
		return p.dbBreaker.Do(func() error {
			var err error
			prevScan, err = p.scans.GetLatest(ctx, project.ID)
			return err
		})
	})

	// ── 3. Issue tracking ────────────────────────────────────────────────────
	var prevIssues []*domain.Issue
	if prevScan != nil {
		pid := project.ID
		sid := prevScan.ID
		rows, _, err := p.issues.Query(ctx, postgres.IssueFilter{
			ProjectID: &pid,
			ScanID:    &sid,
			Limit:     10000,
		})
		if err == nil {
			prevIssues = make([]*domain.Issue, len(rows))
			for i, r := range rows {
				prevIssues[i] = issueRowToDomain(r)
			}
		}
	}
	trackResult := tracking.Track(req.Issues, prevIssues)

	// ── 4. Quality gate evaluation ───────────────────────────────────────────
	measures := map[string]float64{
		"bugs":            float64(req.Measures.Bugs),
		"vulnerabilities": float64(req.Measures.Vulnerabilities),
		"code_smells":     float64(req.Measures.CodeSmells),
		"files":           float64(req.Measures.Files),
		"lines":           float64(req.Measures.Lines),
		"ncloc":           float64(req.Measures.Ncloc),
	}
	gateStatus := qualitygate.Evaluate(qualitygate.DefaultConditions(), measures)
	gateStr := string(gateStatus.Status)

	// ── 5. Insert scan ───────────────────────────────────────────────────────
	scan := &postgres.Scan{
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
		return p.dbBreaker.Do(func() error {
			return p.scans.Create(ctx, scan)
		})
	}); err != nil {
		return nil, fmt.Errorf("create scan: %w", err)
	}

	// ── 6. Bulk insert issues ────────────────────────────────────────────────
	issueRows := make([]postgres.IssueRow, len(req.Issues))
	for i, iss := range req.Issues {
		issueRows[i] = domainToIssueRow(iss, scan.ID, project.ID)
	}
	if err := pipelineSteps.bulkInsIssues.run(ctx, func(ctx context.Context) error {
		return p.dbBreaker.Do(func() error {
			return p.issues.BulkInsert(ctx, issueRows)
		})
	}); err != nil {
		return nil, fmt.Errorf("bulk insert issues: %w", err)
	}

	// ── 7. Bulk insert measures ──────────────────────────────────────────────
	measureRows := buildMeasureRows(req.Measures, scan.ID, project.ID)
	if err := pipelineSteps.bulkInsMeasures.run(ctx, func(ctx context.Context) error {
		return p.dbBreaker.Do(func() error {
			return p.measures.BulkInsert(ctx, measureRows)
		})
	}); err != nil {
		return nil, fmt.Errorf("bulk insert measures: %w", err)
	}

	// ── 8. Async: enqueue search indexing ────────────────────────────────────
	if p.enqueuer != nil {
		_ = pipelineSteps.indexSearch.run(ctx, func(ctx context.Context) error {
			return p.enqueuer.Enqueue(ctx, scan.ID, project.ID, project.Key)
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

func issueRowToDomain(r *postgres.IssueRow) *domain.Issue {
	return &domain.Issue{
		RuleKey:       r.RuleKey,
		ComponentPath: r.ComponentPath,
		Line:          r.Line,
		Column:        r.Column,
		EndLine:       r.EndLine,
		EndColumn:     r.EndColumn,
		Message:       r.Message,
		Type:          domain.IssueType(r.Type),
		Severity:      domain.Severity(r.Severity),
		Status:        domain.Status(r.Status),
		Resolution:    r.Resolution,
		EffortMinutes: r.EffortMinutes,
		LineHash:      r.LineHash,
		Tags:          r.Tags,
	}
}

func domainToIssueRow(iss *domain.Issue, scanID, projectID int64) postgres.IssueRow {
	tags := iss.Tags
	if tags == nil {
		tags = []string{}
	}
	engineID := iss.EngineID
	if engineID == "" {
		engineID = "ollanta"
	}
	sl, _ := json.Marshal(iss.SecondaryLocations)
	if len(iss.SecondaryLocations) == 0 {
		sl = []byte("[]")
	}
	return postgres.IssueRow{
		ScanID:             scanID,
		ProjectID:          projectID,
		RuleKey:            iss.RuleKey,
		ComponentPath:      iss.ComponentPath,
		Line:               iss.Line,
		Column:             iss.Column,
		EndLine:            iss.EndLine,
		EndColumn:          iss.EndColumn,
		Message:            iss.Message,
		Type:               string(iss.Type),
		Severity:           string(iss.Severity),
		Status:             string(iss.Status),
		Resolution:         iss.Resolution,
		EffortMinutes:      iss.EffortMinutes,
		EngineID:           engineID,
		LineHash:           iss.LineHash,
		Tags:               tags,
		SecondaryLocations: sl,
	}
}

func buildMeasureRows(m IngestMeasures, scanID, projectID int64) []postgres.MeasureRow {
	rows := []postgres.MeasureRow{
		{ScanID: scanID, ProjectID: projectID, MetricKey: "files", Value: float64(m.Files)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "lines", Value: float64(m.Lines)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "ncloc", Value: float64(m.Ncloc)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "comments", Value: float64(m.Comments)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "bugs", Value: float64(m.Bugs)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "code_smells", Value: float64(m.CodeSmells)},
		{ScanID: scanID, ProjectID: projectID, MetricKey: "vulnerabilities", Value: float64(m.Vulnerabilities)},
	}
	for lang, count := range m.ByLang {
		rows = append(rows, postgres.MeasureRow{
			ScanID:        scanID,
			ProjectID:     projectID,
			MetricKey:     "files_by_lang_" + lang,
			ComponentPath: lang,
			Value:         float64(count),
		})
	}
	return rows
}
