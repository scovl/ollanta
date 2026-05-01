package ingest

import (
	"context"
	"log/slog"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

const repoPageSize = 1000

var _ port.IProjectRepo = (*projectRepoAdapter)(nil)
var _ port.IScanRepo = (*scanRepoAdapter)(nil)
var _ port.IScanJobRepo = (*scanJobRepoAdapter)(nil)
var _ port.IIssueRepo = (*issueRepoAdapter)(nil)
var _ port.IMeasureRepo = (*measureRepoAdapter)(nil)
var _ port.ICodeSnapshotRepo = (*codeSnapshotRepoAdapter)(nil)

type projectRepoAdapter struct {
	inner *postgres.ProjectRepository
}

func (a *projectRepoAdapter) Create(ctx context.Context, p *model.Project) error {
	store := toStoreProject(p)
	if err := a.inner.Create(ctx, store); err != nil {
		return mapStoreErr(err)
	}
	copyProjectFromStore(p, store)
	return nil
}

func (a *projectRepoAdapter) Upsert(ctx context.Context, p *model.Project) error {
	store := toStoreProject(p)
	if err := a.inner.Upsert(ctx, store); err != nil {
		return mapStoreErr(err)
	}
	copyProjectFromStore(p, store)
	return nil
}

func (a *projectRepoAdapter) GetByKey(ctx context.Context, key string) (*model.Project, error) {
	project, err := a.inner.GetByKey(ctx, key)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return toDomainProject(project), nil
}

func (a *projectRepoAdapter) GetByID(ctx context.Context, id int64) (*model.Project, error) {
	project, err := a.inner.GetByID(ctx, id)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return toDomainProject(project), nil
}

func (a *projectRepoAdapter) List(ctx context.Context) ([]*model.Project, error) {
	var out []*model.Project
	total := 1
	for offset := 0; offset < total; offset += repoPageSize {
		items, count, err := a.inner.List(ctx, repoPageSize, offset)
		if err != nil {
			return nil, mapStoreErr(err)
		}
		total = count
		for _, item := range items {
			out = append(out, toDomainProject(item))
		}
		if len(items) == 0 {
			break
		}
	}
	if out == nil {
		out = []*model.Project{}
	}
	return out, nil
}

func (a *projectRepoAdapter) Delete(ctx context.Context, id int64) error {
	return mapStoreErr(a.inner.Delete(ctx, id))
}

type scanRepoAdapter struct {
	inner *postgres.ScanRepository
}

func (a *scanRepoAdapter) Create(ctx context.Context, s *model.Scan) error {
	store := toStoreScan(s)
	if err := a.inner.Create(ctx, store); err != nil {
		return mapStoreErr(err)
	}
	copyScanFromStore(s, store)
	return nil
}

func (a *scanRepoAdapter) Update(ctx context.Context, s *model.Scan) error {
	return mapStoreErr(a.inner.Update(ctx, toStoreScan(s)))
}

func (a *scanRepoAdapter) GetByID(ctx context.Context, id int64) (*model.Scan, error) {
	scan, err := a.inner.GetByID(ctx, id)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return toDomainScan(scan), nil
}

func (a *scanRepoAdapter) GetLatest(ctx context.Context, projectID int64) (*model.Scan, error) {
	scan, err := a.inner.GetLatest(ctx, projectID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return toDomainScan(scan), nil
}

func (a *scanRepoAdapter) GetLatestInScope(ctx context.Context, projectID int64, scope model.AnalysisScope, defaultBranch string) (*model.Scan, error) {
	scan, err := a.inner.GetLatestInScope(ctx, projectID, scope, defaultBranch)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return toDomainScan(scan), nil
}

func (a *scanRepoAdapter) ListByProject(ctx context.Context, projectID int64) ([]*model.Scan, error) {
	var out []*model.Scan
	total := 1
	for offset := 0; offset < total; offset += repoPageSize {
		items, count, err := a.inner.ListByProject(ctx, projectID, repoPageSize, offset)
		if err != nil {
			return nil, mapStoreErr(err)
		}
		total = count
		for _, item := range items {
			out = append(out, toDomainScan(item))
		}
		if len(items) == 0 {
			break
		}
	}
	if out == nil {
		out = []*model.Scan{}
	}
	return out, nil
}

func (a *scanRepoAdapter) ListByProjectInScope(ctx context.Context, projectID int64, scope model.AnalysisScope, defaultBranch string) ([]*model.Scan, error) {
	items, err := a.inner.ListByProjectInScope(ctx, projectID, scope, defaultBranch)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	out := make([]*model.Scan, 0, len(items))
	for _, item := range items {
		out = append(out, toDomainScan(item))
	}
	if out == nil {
		out = []*model.Scan{}
	}
	return out, nil
}

func (a *scanRepoAdapter) ResolveDefaultBranch(ctx context.Context, projectID int64, configured string) (string, bool, error) {
	branch, inferred, err := a.inner.ResolveDefaultBranch(ctx, projectID, configured)
	return branch, inferred, mapStoreErr(err)
}

type scanJobRepoAdapter struct {
	inner *postgres.ScanJobRepository
}

func (a *scanJobRepoAdapter) Create(ctx context.Context, job *model.ScanJob) error {
	store := toStoreScanJob(job)
	if err := a.inner.Create(ctx, store); err != nil {
		return mapStoreErr(err)
	}
	copyScanJobFromStore(job, store)
	return nil
}

func (a *scanJobRepoAdapter) GetByID(ctx context.Context, id int64) (*model.ScanJob, error) {
	job, err := a.inner.GetByID(ctx, id)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return toDomainScanJob(job), nil
}

func (a *scanJobRepoAdapter) ClaimNext(ctx context.Context, workerID string) (*model.ScanJob, error) {
	job, err := a.inner.ClaimNext(ctx, workerID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return toDomainScanJob(job), nil
}

func (a *scanJobRepoAdapter) MarkCompleted(ctx context.Context, id, scanID int64) error {
	return mapStoreErr(a.inner.MarkCompleted(ctx, id, scanID))
}

func (a *scanJobRepoAdapter) MarkFailed(ctx context.Context, id int64, lastError string) error {
	return mapStoreErr(a.inner.MarkFailed(ctx, id, lastError))
}

type issueRepoAdapter struct {
	inner *postgres.IssueRepository
}

func (a *issueRepoAdapter) BulkInsert(ctx context.Context, issues []model.IssueRow) error {
	rows := make([]postgres.IssueRow, len(issues))
	for i := range issues {
		rows[i] = toStoreIssueRow(issues[i])
	}
	return mapStoreErr(a.inner.BulkInsert(ctx, rows))
}

func (a *issueRepoAdapter) Query(ctx context.Context, filter model.IssueFilter) ([]*model.IssueRow, int, error) {
	rows, total, err := a.inner.Query(ctx, toStoreIssueFilter(filter))
	if err != nil {
		return nil, 0, mapStoreErr(err)
	}
	out := make([]*model.IssueRow, len(rows))
	for i, row := range rows {
		out[i] = toDomainIssueRow(row)
	}
	return out, total, nil
}

func (a *issueRepoAdapter) Facets(ctx context.Context, projectID, scanID int64) (*model.IssueFacets, error) {
	facets, err := a.inner.Facets(ctx, projectID, scanID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return toDomainIssueFacets(facets), nil
}

func (a *issueRepoAdapter) CountByProject(ctx context.Context, projectID int64) (int, error) {
	count, err := a.inner.CountByProject(ctx, projectID)
	if err != nil {
		return 0, mapStoreErr(err)
	}
	return count, nil
}

func (a *issueRepoAdapter) GetByID(ctx context.Context, id int64) (*model.IssueRow, error) {
	issue, err := a.inner.GetByID(ctx, id)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return toDomainIssueRow(issue), nil
}

func (a *issueRepoAdapter) Transition(ctx context.Context, issueID, userID int64, toStatus, resolution, comment string) error {
	return mapStoreErr(a.inner.Transition(ctx, issueID, userID, toStatus, resolution, comment))
}

type measureRepoAdapter struct {
	inner *postgres.MeasureRepository
}

type codeSnapshotRepoAdapter struct {
	inner *postgres.CodeSnapshotRepository
}

func (a *codeSnapshotRepoAdapter) Replace(ctx context.Context, state *model.CodeSnapshotState) error {
	return mapStoreErr(a.inner.Replace(ctx, state))
}

func (a *measureRepoAdapter) BulkInsert(ctx context.Context, measures []model.MeasureRow) error {
	rows := make([]postgres.MeasureRow, len(measures))
	for i := range measures {
		rows[i] = toStoreMeasureRow(measures[i])
	}
	return mapStoreErr(a.inner.BulkInsert(ctx, rows))
}

func (a *measureRepoAdapter) GetLatest(ctx context.Context, projectID int64, metricKey string) (*model.MeasureRow, error) {
	measure, err := a.inner.GetLatest(ctx, projectID, metricKey)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return toDomainMeasureRow(measure), nil
}

func (a *measureRepoAdapter) Trend(ctx context.Context, projectID int64, metricKey string, from, to time.Time) ([]model.TrendPoint, error) {
	points, err := a.inner.Trend(ctx, projectID, metricKey, from, to)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	out := make([]model.TrendPoint, len(points))
	for i, point := range points {
		out[i] = model.TrendPoint{AnalysisDate: point.Date, Value: point.Value}
	}
	return out, nil
}

type searchEnqueuerAdapter struct {
	inner IndexEnqueuer
}

func (a searchEnqueuerAdapter) Enqueue(ctx context.Context, scanID, projectID int64, projectKey string) {
	if err := a.inner.Enqueue(ctx, scanID, projectID, projectKey); err != nil {
		slog.WarnContext(ctx, "enqueue search index job", telemetry.WithTraceAttrs(ctx,
			"scan_id", scanID,
			"project_id", projectID,
			"project_key", projectKey,
			"error", err,
		)...)
	}
}

func toDomainProject(project *postgres.Project) *model.Project {
	if project == nil {
		return nil
	}
	return &model.Project{
		ID:          project.ID,
		Key:         project.Key,
		Name:        project.Name,
		Description: project.Description,
		MainBranch:  project.MainBranch,
		Tags:        project.Tags,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}
}

func toStoreProject(project *model.Project) *postgres.Project {
	if project == nil {
		return nil
	}
	return &postgres.Project{
		ID:          project.ID,
		Key:         project.Key,
		Name:        project.Name,
		Description: project.Description,
		MainBranch:  project.MainBranch,
		Tags:        project.Tags,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}
}

func copyProjectFromStore(dst *model.Project, src *postgres.Project) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.Key = src.Key
	dst.Name = src.Name
	dst.Description = src.Description
	dst.MainBranch = src.MainBranch
	dst.Tags = src.Tags
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func toDomainScan(scan *postgres.Scan) *model.Scan {
	if scan == nil {
		return nil
	}
	return &model.Scan{
		ID:                   scan.ID,
		ProjectID:            scan.ProjectID,
		Version:              scan.Version,
		ScopeType:            scan.ScopeType,
		Branch:               scan.Branch,
		CommitSHA:            scan.CommitSHA,
		PullRequestKey:       scan.PullRequestKey,
		PullRequestBase:      scan.PullRequestBase,
		Status:               scan.Status,
		ElapsedMs:            scan.ElapsedMs,
		GateStatus:           scan.GateStatus,
		AnalysisDate:         scan.AnalysisDate,
		CreatedAt:            scan.CreatedAt,
		TotalFiles:           scan.TotalFiles,
		TotalLines:           scan.TotalLines,
		TotalNcloc:           scan.TotalNcloc,
		TotalComments:        scan.TotalComments,
		TotalIssues:          scan.TotalIssues,
		TotalBugs:            scan.TotalBugs,
		TotalCodeSmells:      scan.TotalCodeSmells,
		TotalVulnerabilities: scan.TotalVulnerabilities,
		NewIssues:            scan.NewIssues,
		ClosedIssues:         scan.ClosedIssues,
	}
}

func toStoreScan(scan *model.Scan) *postgres.Scan {
	if scan == nil {
		return nil
	}
	return &postgres.Scan{
		ID:                   scan.ID,
		ProjectID:            scan.ProjectID,
		Version:              scan.Version,
		ScopeType:            scan.ScopeType,
		Branch:               scan.Branch,
		CommitSHA:            scan.CommitSHA,
		PullRequestKey:       scan.PullRequestKey,
		PullRequestBase:      scan.PullRequestBase,
		Status:               scan.Status,
		ElapsedMs:            scan.ElapsedMs,
		GateStatus:           scan.GateStatus,
		AnalysisDate:         scan.AnalysisDate,
		CreatedAt:            scan.CreatedAt,
		TotalFiles:           scan.TotalFiles,
		TotalLines:           scan.TotalLines,
		TotalNcloc:           scan.TotalNcloc,
		TotalComments:        scan.TotalComments,
		TotalIssues:          scan.TotalIssues,
		TotalBugs:            scan.TotalBugs,
		TotalCodeSmells:      scan.TotalCodeSmells,
		TotalVulnerabilities: scan.TotalVulnerabilities,
		NewIssues:            scan.NewIssues,
		ClosedIssues:         scan.ClosedIssues,
	}
}

func toDomainScanJob(job *postgres.ScanJob) *model.ScanJob {
	if job == nil {
		return nil
	}
	return &model.ScanJob{
		ID:          job.ID,
		ProjectKey:  job.ProjectKey,
		Status:      model.ScanJobStatus(job.Status),
		Payload:     job.Payload,
		TraceParent: job.TraceParent,
		TraceState:  job.TraceState,
		ScanID:      job.ScanID,
		WorkerID:    job.WorkerID,
		LastError:   job.LastError,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		StartedAt:   job.StartedAt,
		CompletedAt: job.CompletedAt,
	}
}

func toStoreScanJob(job *model.ScanJob) *postgres.ScanJob {
	if job == nil {
		return nil
	}
	return &postgres.ScanJob{
		ID:          job.ID,
		ProjectKey:  job.ProjectKey,
		Status:      string(job.Status),
		Payload:     job.Payload,
		TraceParent: job.TraceParent,
		TraceState:  job.TraceState,
		ScanID:      job.ScanID,
		WorkerID:    job.WorkerID,
		LastError:   job.LastError,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		StartedAt:   job.StartedAt,
		CompletedAt: job.CompletedAt,
	}
}

func copyScanJobFromStore(dst *model.ScanJob, src *postgres.ScanJob) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.ProjectKey = src.ProjectKey
	dst.Status = model.ScanJobStatus(src.Status)
	dst.Payload = src.Payload
	dst.TraceParent = src.TraceParent
	dst.TraceState = src.TraceState
	dst.ScanID = src.ScanID
	dst.WorkerID = src.WorkerID
	dst.LastError = src.LastError
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
	dst.StartedAt = src.StartedAt
	dst.CompletedAt = src.CompletedAt
}

func copyScanFromStore(dst *model.Scan, src *postgres.Scan) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.ProjectID = src.ProjectID
	dst.Version = src.Version
	dst.ScopeType = src.ScopeType
	dst.Branch = src.Branch
	dst.CommitSHA = src.CommitSHA
	dst.PullRequestKey = src.PullRequestKey
	dst.PullRequestBase = src.PullRequestBase
	dst.Status = src.Status
	dst.ElapsedMs = src.ElapsedMs
	dst.GateStatus = src.GateStatus
	dst.AnalysisDate = src.AnalysisDate
	dst.CreatedAt = src.CreatedAt
	dst.TotalFiles = src.TotalFiles
	dst.TotalLines = src.TotalLines
	dst.TotalNcloc = src.TotalNcloc
	dst.TotalComments = src.TotalComments
	dst.TotalIssues = src.TotalIssues
	dst.TotalBugs = src.TotalBugs
	dst.TotalCodeSmells = src.TotalCodeSmells
	dst.TotalVulnerabilities = src.TotalVulnerabilities
	dst.NewIssues = src.NewIssues
	dst.ClosedIssues = src.ClosedIssues
}

func toStoreIssueFilter(filter model.IssueFilter) postgres.IssueFilter {
	return postgres.IssueFilter{
		ProjectID:        filter.ProjectID,
		ScanID:           filter.ScanID,
		RuleKey:          filter.RuleKey,
		Severity:         filter.Severity,
		Type:             filter.Type,
		QualityDomain:    filter.QualityDomain,
		Status:           filter.Status,
		TrackingState:    filter.TrackingState,
		Language:         filter.Language,
		Tag:              filter.Tag,
		SecurityCategory: filter.SecurityCategory,
		Directory:        filter.Directory,
		FilePath:         filter.FilePath,
		EngineID:         filter.EngineID,
		Limit:            filter.Limit,
		Offset:           filter.Offset,
	}
}

func toDomainIssueRow(issue *postgres.IssueRow) *model.IssueRow {
	if issue == nil {
		return nil
	}
	return &model.IssueRow{
		ID:                 issue.ID,
		ScanID:             issue.ScanID,
		ProjectID:          issue.ProjectID,
		RuleKey:            issue.RuleKey,
		ComponentPath:      issue.ComponentPath,
		Line:               issue.Line,
		Column:             issue.Column,
		EndLine:            issue.EndLine,
		EndColumn:          issue.EndColumn,
		Message:            issue.Message,
		Type:               issue.Type,
		Severity:           issue.Severity,
		QualityDomain:      issue.QualityDomain,
		Language:           issue.Language,
		Status:             issue.Status,
		Resolution:         issue.Resolution,
		TrackingState:      issue.TrackingState,
		EffortMinutes:      issue.EffortMinutes,
		EngineID:           issue.EngineID,
		LineHash:           issue.LineHash,
		Tags:               issue.Tags,
		SecondaryLocations: issue.SecondaryLocations,
		CreatedAt:          issue.CreatedAt,
	}
}

func toStoreIssueRow(issue model.IssueRow) postgres.IssueRow {
	return postgres.IssueRow{
		ID:                 issue.ID,
		ScanID:             issue.ScanID,
		ProjectID:          issue.ProjectID,
		RuleKey:            issue.RuleKey,
		ComponentPath:      issue.ComponentPath,
		Line:               issue.Line,
		Column:             issue.Column,
		EndLine:            issue.EndLine,
		EndColumn:          issue.EndColumn,
		Message:            issue.Message,
		Type:               issue.Type,
		Severity:           issue.Severity,
		QualityDomain:      issue.QualityDomain,
		Language:           issue.Language,
		Status:             issue.Status,
		Resolution:         issue.Resolution,
		TrackingState:      issue.TrackingState,
		EffortMinutes:      issue.EffortMinutes,
		EngineID:           issue.EngineID,
		LineHash:           issue.LineHash,
		Tags:               issue.Tags,
		SecondaryLocations: issue.SecondaryLocations,
		CreatedAt:          issue.CreatedAt,
	}
}

func toDomainIssueFacets(facets *postgres.IssueFacets) *model.IssueFacets {
	if facets == nil {
		return &model.IssueFacets{
			BySeverity:         map[string]int{},
			ByType:             map[string]int{},
			ByQuality:          map[string]int{},
			ByRule:             map[string]int{},
			ByStatus:           map[string]int{},
			ByLifecycle:        map[string]int{},
			ByLanguage:         map[string]int{},
			ByEngineID:         map[string]int{},
			ByFile:             map[string]int{},
			ByDirectory:        map[string]int{},
			ByTags:             map[string]int{},
			BySecurityCategory: map[string]int{},
		}
	}
	return &model.IssueFacets{
		BySeverity:         cloneFacetMap(facets.BySeverity),
		ByType:             cloneFacetMap(facets.ByType),
		ByQuality:          cloneFacetMap(facets.ByQuality),
		ByRule:             cloneFacetMap(facets.ByRule),
		ByStatus:           cloneFacetMap(facets.ByStatus),
		ByLifecycle:        cloneFacetMap(facets.ByLifecycle),
		ByLanguage:         cloneFacetMap(facets.ByLanguage),
		ByEngineID:         cloneFacetMap(facets.ByEngineID),
		ByFile:             cloneFacetMap(facets.ByFile),
		ByDirectory:        cloneFacetMap(facets.ByDirectory),
		ByTags:             cloneFacetMap(facets.ByTags),
		BySecurityCategory: cloneFacetMap(facets.BySecurityCategory),
	}
}

func cloneFacetMap(src map[string]int) map[string]int {
	if src == nil {
		return map[string]int{}
	}
	dst := make(map[string]int, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func toDomainMeasureRow(measure *postgres.MeasureRow) *model.MeasureRow {
	if measure == nil {
		return nil
	}
	return &model.MeasureRow{
		ID:            measure.ID,
		ScanID:        measure.ScanID,
		ProjectID:     measure.ProjectID,
		MetricKey:     measure.MetricKey,
		ComponentPath: measure.ComponentPath,
		Value:         measure.Value,
		CreatedAt:     measure.CreatedAt,
	}
}

func toStoreMeasureRow(measure model.MeasureRow) postgres.MeasureRow {
	return postgres.MeasureRow{
		ID:            measure.ID,
		ScanID:        measure.ScanID,
		ProjectID:     measure.ProjectID,
		MetricKey:     measure.MetricKey,
		ComponentPath: measure.ComponentPath,
		Value:         measure.Value,
		CreatedAt:     measure.CreatedAt,
	}
}
