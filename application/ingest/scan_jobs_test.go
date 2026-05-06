package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantacore/tracectx"
	"go.opentelemetry.io/otel/trace"
)

func TestScanJobServiceSubmitPersistsAcceptedJob(t *testing.T) {
	t.Parallel()

	repo := &fakeScanJobRepo{}
	svc := NewScanJobService(repo)
	ctx, expectedTraceID := tracedContext()

	job, err := svc.Submit(ctx, &IngestRequest{
		Metadata: IngestMetadata{ProjectKey: "demo"},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if job.Status != model.ScanJobStatusAccepted {
		t.Fatalf("job.Status = %q, want %q", job.Status, model.ScanJobStatusAccepted)
	}
	if repo.created == nil {
		t.Fatal("expected job to be persisted")
	}
	if repo.created.ProjectKey != "demo" {
		t.Fatalf("created.ProjectKey = %q, want demo", repo.created.ProjectKey)
	}
	if len(repo.created.Payload) == 0 {
		t.Fatal("expected payload to be stored")
	}
	if repo.created.IdempotencyKey == "" {
		t.Fatal("expected server-computed idempotency key to be stored")
	}
	if repo.created.PayloadHash == "" {
		t.Fatal("expected payload hash to be stored")
	}
	if repo.created.TraceParent == "" {
		t.Fatal("expected traceparent to be stored with accepted job")
	}
	if repo.created.TraceState != "" {
		t.Fatalf("created.TraceState = %q, want empty tracestate for test context", repo.created.TraceState)
	}
	if trace.SpanContextFromContext(tracectx.Extract(context.Background(), repo.created.TraceParent, repo.created.TraceState)).TraceID().String() != expectedTraceID {
		t.Fatalf("created trace id did not round-trip")
	}
	if repo.created.ID == 0 {
		t.Fatal("expected repository to assign an id")
	}
	if job.ID != repo.created.ID {
		t.Fatalf("job.ID = %d, want %d", job.ID, repo.created.ID)
	}
	if _, err := svc.Get(context.Background(), job.ID); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := svc.Get(context.Background(), job.ID+999); err == nil {
		t.Fatal("expected missing job to return an error")
	}
}

func TestScanJobServiceSubmitWithOptionsReturnsDuplicateForMatchingPayload(t *testing.T) {
	t.Parallel()

	req := &IngestRequest{Metadata: IngestMetadata{ProjectKey: "demo"}}
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	existing := &model.ScanJob{
		ID:             42,
		ProjectKey:     "demo",
		Status:         model.ScanJobStatusAccepted,
		IdempotencyKey: "ci-run-1",
		PayloadHash:    hashScanPayload(payload),
	}
	repo := &fakeScanJobRepo{jobsByIdempotency: map[string]*model.ScanJob{idempotencyLookupKey("demo", "ci-run-1"): existing}}
	svc := NewScanJobService(repo)

	result, err := svc.SubmitWithOptions(context.Background(), req, ScanJobSubmitOptions{IdempotencyKey: "ci-run-1"})
	if err != nil {
		t.Fatalf("SubmitWithOptions() error = %v", err)
	}
	if !result.Duplicate {
		t.Fatal("Duplicate = false, want true")
	}
	if result.Job.ID != existing.ID {
		t.Fatalf("Job.ID = %d, want existing id %d", result.Job.ID, existing.ID)
	}
	if repo.created != nil {
		t.Fatal("expected duplicate submission not to create a job")
	}
}

func TestScanJobServiceSubmitWithOptionsRejectsIdempotencyConflict(t *testing.T) {
	t.Parallel()

	repo := &fakeScanJobRepo{jobsByIdempotency: map[string]*model.ScanJob{
		idempotencyLookupKey("demo", "ci-run-1"): {
			ID:             42,
			ProjectKey:     "demo",
			Status:         model.ScanJobStatusAccepted,
			IdempotencyKey: "ci-run-1",
			PayloadHash:    hashScanPayload([]byte(`{"different":true}`)),
		},
	}}
	svc := NewScanJobService(repo)

	_, err := svc.SubmitWithOptions(context.Background(), &IngestRequest{Metadata: IngestMetadata{ProjectKey: "demo"}}, ScanJobSubmitOptions{IdempotencyKey: "ci-run-1"})
	if !errors.Is(err, ErrScanJobIdempotencyConflict) {
		t.Fatalf("SubmitWithOptions() error = %v, want ErrScanJobIdempotencyConflict", err)
	}
	if repo.created != nil {
		t.Fatal("expected conflicting submission not to create a job")
	}
}

func TestScanJobServiceSubmitWithOptionsRejectsBackpressure(t *testing.T) {
	t.Parallel()

	repo := &fakeScanJobRepo{pressure: model.ScanQueuePressure{Accepted: 3}}
	svc := NewScanJobService(repo)

	_, err := svc.SubmitWithOptions(context.Background(), &IngestRequest{Metadata: IngestMetadata{ProjectKey: "demo"}}, ScanJobSubmitOptions{
		Backpressure: ScanBackpressureConfig{MaxAccepted: 3, RetryAfter: 15 * time.Second},
	})
	var backpressureErr *ScanJobBackpressureError
	if !errors.As(err, &backpressureErr) {
		t.Fatalf("SubmitWithOptions() error = %v, want ScanJobBackpressureError", err)
	}
	if backpressureErr.RetryAfter != 15*time.Second {
		t.Fatalf("RetryAfter = %s, want 15s", backpressureErr.RetryAfter)
	}
	if repo.created != nil {
		t.Fatal("expected saturated intake not to create a job")
	}
}

func TestScanJobProcessorProcessNextMarksCompleted(t *testing.T) {
	t.Parallel()

	jobRepo := &fakeScanJobRepo{}
	projectRepo := &fakeProjectRepo{}
	scanRepo := &fakeScanRepo{}
	issueRepo := &fakeIssueRepo{}
	measureRepo := &fakeMeasureRepo{}
	snapshotRepo := &fakeCodeSnapshotRepo{}

	req := &IngestRequest{
		Metadata: IngestMetadata{ProjectKey: "demo", AnalysisDate: time.Now().UTC().Format(time.RFC3339)},
		Measures: IngestMeasures{Files: 1, Lines: 10, Ncloc: 8, Comments: 2},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jobCtx, expectedTraceID := tracedContext()
	traceParent, traceState := tracectx.Inject(jobCtx)
	jobRepo.next = &model.ScanJob{ID: 7, ProjectKey: "demo", Status: model.ScanJobStatusAccepted, Payload: payload, TraceParent: traceParent, TraceState: traceState}

	processor := NewScanJobProcessor(
		"worker-1",
		jobRepo,
		NewIngestUseCase(projectRepo, scanRepo, issueRepo, measureRepo, snapshotRepo, nil, nil),
	)

	job, err := processor.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	if job == nil {
		t.Fatal("expected a processed job")
	}
	if job.Status != model.ScanJobStatusCompleted {
		t.Fatalf("job.Status = %q, want %q", job.Status, model.ScanJobStatusCompleted)
	}
	if job.ScanID == nil || *job.ScanID == 0 {
		t.Fatal("expected completed job to link to a scan")
	}
	if jobRepo.completedID != job.ID {
		t.Fatalf("completedID = %d, want %d", jobRepo.completedID, job.ID)
	}
	if projectRepo.upserted == nil || projectRepo.upserted.Key != "demo" {
		t.Fatal("expected ingest to upsert the project")
	}
	if projectRepo.traceID != expectedTraceID {
		t.Fatalf("project repo traceID = %q, want %q", projectRepo.traceID, expectedTraceID)
	}
	if scanRepo.created == nil || scanRepo.created.ProjectID == 0 {
		t.Fatal("expected ingest to create a scan")
	}
	if len(measureRepo.inserted) == 0 {
		t.Fatal("expected ingest to persist measures")
	}
}

func TestIngestDiscoversIssueTags(t *testing.T) {
	t.Parallel()

	projectRepo := &fakeProjectRepo{}
	scanRepo := &fakeScanRepo{}
	issueRepo := &fakeIssueRepo{}
	measureRepo := &fakeMeasureRepo{}
	tagRepo := &fakeTagCatalogRepo{}
	uc := NewIngestUseCase(projectRepo, scanRepo, issueRepo, measureRepo, nil, nil, nil)
	uc.SetTagCatalogRepo(tagRepo)

	_, err := uc.Ingest(context.Background(), &IngestRequest{
		Metadata: IngestMetadata{ProjectKey: "demo"},
		Measures: IngestMeasures{Files: 1, Lines: 10, Ncloc: 8, Comments: 2},
		Issues: []*model.Issue{{
			RuleKey:       "go:test",
			ComponentPath: "main.go",
			Type:          model.TypeVulnerability,
			Severity:      model.SeverityMajor,
			Tags:          []string{"Security", "cwe-79"},
		}},
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if tagRepo.source != model.TagSourceScan {
		t.Fatalf("tag source = %q, want scan", tagRepo.source)
	}
	if !containsIngestTestString(tagRepo.keys, "Security") || !containsIngestTestString(tagRepo.keys, "cwe-79") {
		t.Fatalf("discovered tags = %#v, want issue tags", tagRepo.keys)
	}
}

func TestScanJobProcessorProcessNextMarksFailedOnInvalidPayload(t *testing.T) {
	t.Parallel()

	jobRepo := &fakeScanJobRepo{
		next: &model.ScanJob{ID: 9, ProjectKey: "demo", Status: model.ScanJobStatusAccepted, Payload: []byte("not-json")},
	}

	processor := NewScanJobProcessor("worker-2", jobRepo, &IngestUseCase{})

	job, err := processor.ProcessNext(context.Background())
	if err == nil {
		t.Fatal("expected ProcessNext() to fail for invalid payload")
	}
	if job == nil {
		t.Fatal("expected returned job on failure")
	}
	if job.Status != model.ScanJobStatusFailed {
		t.Fatalf("job.Status = %q, want %q", job.Status, model.ScanJobStatusFailed)
	}
	if jobRepo.failedID != job.ID {
		t.Fatalf("failedID = %d, want %d", jobRepo.failedID, job.ID)
	}
	if jobRepo.failedErr == "" {
		t.Fatal("expected failure reason to be recorded")
	}
}

type fakeScanJobRepo struct {
	created           *model.ScanJob
	next              *model.ScanJob
	completedID       int64
	completedRef      int64
	failedID          int64
	failedErr         string
	jobs              map[int64]*model.ScanJob
	jobsByIdempotency map[string]*model.ScanJob
	pressure          model.ScanQueuePressure
	nextID            int64
}

func (r *fakeScanJobRepo) Create(_ context.Context, job *model.ScanJob) error {
	r.nextID++
	job.ID = r.nextID
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	r.created = cloneScanJob(job)
	if r.jobs == nil {
		r.jobs = map[int64]*model.ScanJob{}
	}
	r.jobs[job.ID] = cloneScanJob(job)
	if job.IdempotencyKey != "" {
		if r.jobsByIdempotency == nil {
			r.jobsByIdempotency = map[string]*model.ScanJob{}
		}
		r.jobsByIdempotency[idempotencyLookupKey(job.ProjectKey, job.IdempotencyKey)] = cloneScanJob(job)
	}
	return nil
}

func (r *fakeScanJobRepo) GetByID(_ context.Context, id int64) (*model.ScanJob, error) {
	if job, ok := r.jobs[id]; ok {
		return cloneScanJob(job), nil
	}
	return nil, model.ErrNotFound
}

func (r *fakeScanJobRepo) FindByIdempotencyKey(_ context.Context, projectKey, idempotencyKey string) (*model.ScanJob, error) {
	if r.jobsByIdempotency == nil {
		return nil, model.ErrNotFound
	}
	job, ok := r.jobsByIdempotency[idempotencyLookupKey(projectKey, idempotencyKey)]
	if !ok {
		return nil, model.ErrNotFound
	}
	return cloneScanJob(job), nil
}

func (r *fakeScanJobRepo) QueuePressure(_ context.Context, _ string, _ time.Time) (model.ScanQueuePressure, error) {
	return r.pressure, nil
}

func (r *fakeScanJobRepo) ClaimNext(_ context.Context, workerID string) (*model.ScanJob, error) {
	if r.next == nil {
		return nil, model.ErrNotFound
	}
	r.next.Status = model.ScanJobStatusRunning
	r.next.WorkerID = workerID
	now := time.Now().UTC()
	r.next.StartedAt = &now
	r.next.UpdatedAt = now
	return cloneScanJob(r.next), nil
}

func (r *fakeScanJobRepo) MarkCompleted(_ context.Context, id, scanID int64) error {
	r.completedID = id
	r.completedRef = scanID
	if r.next != nil && r.next.ID == id {
		r.next.Status = model.ScanJobStatusCompleted
		r.next.ScanID = &scanID
		now := time.Now().UTC()
		r.next.CompletedAt = &now
		r.next.UpdatedAt = now
	}
	return nil
}

func (r *fakeScanJobRepo) MarkFailed(_ context.Context, id int64, lastError string) error {
	r.failedID = id
	r.failedErr = lastError
	if r.next != nil && r.next.ID == id {
		r.next.Status = model.ScanJobStatusFailed
		r.next.LastError = lastError
		now := time.Now().UTC()
		r.next.CompletedAt = &now
		r.next.UpdatedAt = now
	}
	return nil
}

type fakeProjectRepo struct {
	upserted *model.Project
	traceID  string
}

func (r *fakeProjectRepo) Create(_ context.Context, p *model.Project) error {
	p.ID = 1
	return nil
}

func (r *fakeProjectRepo) Upsert(ctx context.Context, p *model.Project) error {
	p.ID = 1
	r.upserted = &model.Project{ID: p.ID, Key: p.Key, Name: p.Name}
	if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
		r.traceID = spanContext.TraceID().String()
	}
	return nil
}

func (r *fakeProjectRepo) GetByKey(_ context.Context, _ string) (*model.Project, error) {
	return nil, model.ErrNotFound
}

func (r *fakeProjectRepo) GetByID(_ context.Context, _ int64) (*model.Project, error) {
	return nil, model.ErrNotFound
}

func (r *fakeProjectRepo) List(_ context.Context) ([]*model.Project, error) {
	return nil, nil
}

func (r *fakeProjectRepo) Delete(_ context.Context, _ int64) error {
	return nil
}

type fakeScanRepo struct {
	created *model.Scan
	nextID  int64
}

func (r *fakeScanRepo) Create(_ context.Context, s *model.Scan) error {
	r.nextID++
	s.ID = r.nextID
	r.created = &model.Scan{ID: s.ID, ProjectID: s.ProjectID, Status: s.Status, GateStatus: s.GateStatus}
	return nil
}

func (r *fakeScanRepo) Update(_ context.Context, _ *model.Scan) error {
	return nil
}

func (r *fakeScanRepo) GetByID(_ context.Context, _ int64) (*model.Scan, error) {
	return nil, model.ErrNotFound
}

func (r *fakeScanRepo) GetLatest(_ context.Context, _ int64) (*model.Scan, error) {
	return nil, model.ErrNotFound
}

func (r *fakeScanRepo) GetLatestInScope(_ context.Context, _ int64, _ model.AnalysisScope, _ string) (*model.Scan, error) {
	return nil, model.ErrNotFound
}

func (r *fakeScanRepo) ListByProject(_ context.Context, _ int64) ([]*model.Scan, error) {
	return nil, nil
}

func (r *fakeScanRepo) ListByProjectInScope(_ context.Context, _ int64, _ model.AnalysisScope, _ string) ([]*model.Scan, error) {
	return nil, nil
}

func (r *fakeScanRepo) ResolveDefaultBranch(_ context.Context, _ int64, configured string) (string, bool, error) {
	return configured, false, nil
}

type fakeIssueRepo struct {
	inserted []model.IssueRow
}

func (r *fakeIssueRepo) BulkInsert(_ context.Context, issues []model.IssueRow) error {
	r.inserted = append([]model.IssueRow(nil), issues...)
	return nil
}

func (r *fakeIssueRepo) Query(_ context.Context, _ model.IssueFilter) ([]*model.IssueRow, int, error) {
	return nil, 0, nil
}

func (r *fakeIssueRepo) Facets(_ context.Context, _, _ int64) (*model.IssueFacets, error) {
	return nil, nil
}

func (r *fakeIssueRepo) CountByProject(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

func (r *fakeIssueRepo) GetByID(_ context.Context, _ int64) (*model.IssueRow, error) {
	return nil, model.ErrNotFound
}

func (r *fakeIssueRepo) Transition(_ context.Context, _, _ int64, _, _, _ string) error {
	return nil
}

type fakeMeasureRepo struct {
	inserted []model.MeasureRow
}

func (r *fakeMeasureRepo) BulkInsert(_ context.Context, measures []model.MeasureRow) error {
	r.inserted = append([]model.MeasureRow(nil), measures...)
	return nil
}

func (r *fakeMeasureRepo) GetLatest(_ context.Context, _ int64, _ string) (*model.MeasureRow, error) {
	return nil, model.ErrNotFound
}

func (r *fakeMeasureRepo) Trend(_ context.Context, _ int64, _ string, _, _ time.Time) ([]model.TrendPoint, error) {
	return nil, nil
}

func (r *fakeMeasureRepo) UpsertLive(_ context.Context, _, _ int64, _, _ string, _ float64) error { return nil }
func (r *fakeMeasureRepo) UpsertLiveBatch(_ context.Context, _ int64, _ int64, _ map[string]float64) error { return nil }
func (r *fakeMeasureRepo) GetLive(_ context.Context, _ int64) (map[string]float64, error) { return nil, nil }
func (r *fakeMeasureRepo) UpsertDailyAggregate(_ context.Context, _ int64, _, _ string, _ float64) error { return nil }
func (r *fakeMeasureRepo) UpsertDailyAggregateBatch(_ context.Context, _ int64, _ string, _ map[string]float64) error { return nil }
func (r *fakeMeasureRepo) GetDailyAggregates(_ context.Context, _ int64, _ string, _ int) ([]model.TrendPoint, error) { return nil, nil }

type fakeCodeSnapshotRepo struct {
	replaced *model.CodeSnapshotState
}

func (r *fakeCodeSnapshotRepo) Replace(_ context.Context, state *model.CodeSnapshotState) error {
	r.replaced = state
	return nil
}

type fakeTagCatalogRepo struct {
	keys   []string
	source model.TagSource
}

func (r *fakeTagCatalogRepo) CreateTag(context.Context, model.TagCatalogEntry) (*model.TagCatalogEntry, error) {
	return nil, nil
}

func (r *fakeTagCatalogRepo) UpdateTag(context.Context, string, model.TagUpdate) (*model.TagCatalogEntry, error) {
	return nil, nil
}

func (r *fakeTagCatalogRepo) DeprecateTag(context.Context, string, string, int64) (*model.TagCatalogEntry, error) {
	return nil, nil
}

func (r *fakeTagCatalogRepo) MergeTag(context.Context, string, string, int64) (*model.TagCatalogEntry, error) {
	return nil, nil
}

func (r *fakeTagCatalogRepo) GetTag(context.Context, string) (*model.TagCatalogEntry, error) {
	return nil, nil
}

func (r *fakeTagCatalogRepo) ListTags(context.Context, model.TagFilter) ([]model.TagCatalogEntry, int, error) {
	return nil, 0, nil
}

func (r *fakeTagCatalogRepo) DiscoverTags(_ context.Context, keys []string, source model.TagSource) error {
	r.keys = append([]string(nil), keys...)
	r.source = source
	return nil
}

func (r *fakeTagCatalogRepo) ResolveTagKey(_ context.Context, keyOrAlias string) (string, error) {
	return model.NormalizeTagKey(keyOrAlias), nil
}

func (r *fakeTagCatalogRepo) TagUsage(context.Context, string) (model.TagUsageSummary, error) {
	return model.TagUsageSummary{}, nil
}

func (r *fakeTagCatalogRepo) TagAudit(context.Context, string, int, int) ([]model.TagAuditEntry, int, error) {
	return nil, 0, nil
}

func containsIngestTestString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cloneScanJob(job *model.ScanJob) *model.ScanJob {
	if job == nil {
		return nil
	}
	clone := *job
	if job.Payload != nil {
		clone.Payload = append([]byte(nil), job.Payload...)
	}
	return &clone
}

func idempotencyLookupKey(projectKey, idempotencyKey string) string {
	return projectKey + "\x00" + idempotencyKey
}

func tracedContext() (context.Context, string) {
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe, 0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe},
		SpanID:     trace.SpanID{0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), spanContext)
	return ctx, spanContext.TraceID().String()
}
