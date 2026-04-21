package ingest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/scovl/ollanta/domain/model"
)

func TestScanJobServiceSubmitPersistsAcceptedJob(t *testing.T) {
	t.Parallel()

	repo := &fakeScanJobRepo{}
	svc := NewScanJobService(repo)

	job, err := svc.Submit(context.Background(), &IngestRequest{
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

func TestScanJobProcessorProcessNextMarksCompleted(t *testing.T) {
	t.Parallel()

	jobRepo := &fakeScanJobRepo{}
	projectRepo := &fakeProjectRepo{}
	scanRepo := &fakeScanRepo{}
	issueRepo := &fakeIssueRepo{}
	measureRepo := &fakeMeasureRepo{}

	req := &IngestRequest{
		Metadata: IngestMetadata{ProjectKey: "demo", AnalysisDate: time.Now().UTC().Format(time.RFC3339)},
		Measures: IngestMeasures{Files: 1, Lines: 10, Ncloc: 8, Comments: 2},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jobRepo.next = &model.ScanJob{ID: 7, ProjectKey: "demo", Status: model.ScanJobStatusAccepted, Payload: payload}

	processor := NewScanJobProcessor(
		"worker-1",
		jobRepo,
		NewIngestUseCase(projectRepo, scanRepo, issueRepo, measureRepo, nil, nil),
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
	if scanRepo.created == nil || scanRepo.created.ProjectID == 0 {
		t.Fatal("expected ingest to create a scan")
	}
	if len(measureRepo.inserted) == 0 {
		t.Fatal("expected ingest to persist measures")
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
	created      *model.ScanJob
	next         *model.ScanJob
	completedID  int64
	completedRef int64
	failedID     int64
	failedErr    string
	jobs         map[int64]*model.ScanJob
	nextID       int64
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
	return nil
}

func (r *fakeScanJobRepo) GetByID(_ context.Context, id int64) (*model.ScanJob, error) {
	if job, ok := r.jobs[id]; ok {
		return cloneScanJob(job), nil
	}
	return nil, model.ErrNotFound
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
}

func (r *fakeProjectRepo) Create(_ context.Context, p *model.Project) error {
	p.ID = 1
	return nil
}

func (r *fakeProjectRepo) Upsert(_ context.Context, p *model.Project) error {
	p.ID = 1
	r.upserted = &model.Project{ID: p.ID, Key: p.Key, Name: p.Name}
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

func (r *fakeScanRepo) ListByProject(_ context.Context, _ int64) ([]*model.Scan, error) {
	return nil, nil
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
