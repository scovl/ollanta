package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantacore/tracectx"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
	"go.opentelemetry.io/otel/trace"
)

func TestWorkerProcessNextMarksCompleted(t *testing.T) {
	t.Parallel()

	jobs := &fakeIndexJobStore{
		next: &postgres.IndexJob{ID: 1, ScanID: 42, ProjectID: 7, ProjectKey: "demo", Status: "running", Attempts: 1},
	}
	issues := &fakeIssueQueryer{
		pages: [][]*postgres.IssueRow{{
			{ID: 1, ScanID: 42, ProjectID: 7, RuleKey: "go:foo", ComponentPath: "a.go"},
			{ID: 2, ScanID: 42, ProjectID: 7, RuleKey: "go:bar", ComponentPath: "b.go"},
		}},
	}
	indexer := &fakeIndexer{}
	jobCtx, expectedTraceID := tracedIndexContext()
	jobs.next.TraceParent, jobs.next.TraceState = tracectx.Inject(jobCtx)

	worker := &Worker{indexer: indexer, issues: issues, jobs: jobs, workerID: "worker-1", maxRetries: 3, batchSize: 1000}
	processed, err := worker.processNext(context.Background())
	if err != nil {
		t.Fatalf("processNext() error = %v", err)
	}
	if !processed {
		t.Fatal("expected a job to be processed")
	}
	if jobs.completedID != 1 {
		t.Fatalf("completedID = %d, want 1", jobs.completedID)
	}
	if len(indexer.batches) != 1 || len(indexer.batches[0]) != 2 {
		t.Fatalf("unexpected index batches: %+v", indexer.batches)
	}
	if indexer.traceID != expectedTraceID {
		t.Fatalf("index traceID = %q, want %q", indexer.traceID, expectedTraceID)
	}
}

func TestWorkerProcessNextReschedulesOnFailure(t *testing.T) {
	t.Parallel()

	jobs := &fakeIndexJobStore{
		next: &postgres.IndexJob{ID: 3, ScanID: 99, ProjectID: 11, ProjectKey: "demo", Status: "running", Attempts: 1},
	}
	issues := &fakeIssueQueryer{pages: [][]*postgres.IssueRow{{{ID: 9, ScanID: 99, ProjectID: 11, RuleKey: "go:foo", ComponentPath: "a.go"}}}}
	indexer := &fakeIndexer{indexErr: errors.New("backend down")}

	worker := &Worker{indexer: indexer, issues: issues, jobs: jobs, workerID: "worker-1", maxRetries: 3, batchSize: 1000}
	processed, err := worker.processNext(context.Background())
	if err != nil {
		t.Fatalf("processNext() error = %v", err)
	}
	if !processed {
		t.Fatal("expected a job to be processed")
	}
	if jobs.rescheduledID != 3 {
		t.Fatalf("rescheduledID = %d, want 3", jobs.rescheduledID)
	}
	if jobs.failedID != 0 {
		t.Fatalf("failedID = %d, want 0", jobs.failedID)
	}
}

func TestWorkerRefreshQueueMetricsSetsGauge(t *testing.T) {
	t.Parallel()

	reg := telemetry.NewRegistry()
	metrics := telemetry.NewMetrics(reg)
	worker := &Worker{
		jobs:    &fakeIndexJobStore{counts: map[string]int{"accepted": 3}},
		metrics: metrics,
	}

	worker.refreshQueueMetrics(context.Background())

	body := readMetricsBody(t, reg)
	if !strings.Contains(body, "ollanta_index_queue_depth 3") {
		t.Fatalf("expected index queue depth in metrics output, got: %s", body)
	}
}

func TestIndexJobEnqueuerReturnsExistingActiveJob(t *testing.T) {
	t.Parallel()

	jobs := &fakeIndexJobStore{active: &postgres.IndexJob{ID: 42, ScanID: 99, Status: "accepted"}}
	enqueuer := &IndexJobEnqueuer{jobs: jobs}

	if err := enqueuer.Enqueue(context.Background(), 99, 7, "demo"); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if jobs.created != nil {
		t.Fatalf("created job = %+v, want no duplicate create", jobs.created)
	}
}

type fakeIndexJobStore struct {
	created        *postgres.IndexJob
	active         *postgres.IndexJob
	next           *postgres.IndexJob
	counts         map[string]int
	rescheduledID  int64
	rescheduledErr string
	completedID    int64
	failedID       int64
}

func (s *fakeIndexJobStore) Create(_ context.Context, job *postgres.IndexJob) error {
	s.created = job
	return nil
}

func (s *fakeIndexJobStore) GetActiveByScanID(_ context.Context, _ int64) (*postgres.IndexJob, error) {
	if s.active == nil {
		return nil, postgres.ErrNotFound
	}
	return s.active, nil
}

func (s *fakeIndexJobStore) ClaimNext(_ context.Context, _ string) (*postgres.IndexJob, error) {
	if s.next == nil {
		return nil, postgres.ErrNotFound
	}
	return s.next, nil
}

func (s *fakeIndexJobStore) CountByStatus(_ context.Context, status string) (int, error) {
	return s.counts[status], nil
}

func (s *fakeIndexJobStore) Reschedule(_ context.Context, id int64, lastError string, _ time.Time) error {
	s.rescheduledID = id
	s.rescheduledErr = lastError
	return nil
}

func (s *fakeIndexJobStore) MarkCompleted(_ context.Context, id int64) error {
	s.completedID = id
	return nil
}

func (s *fakeIndexJobStore) MarkFailed(_ context.Context, id int64, _ string) error {
	s.failedID = id
	return nil
}

type fakeIssueQueryer struct {
	pages [][]*postgres.IssueRow
	idx   int
}

func (q *fakeIssueQueryer) Query(_ context.Context, _ postgres.IssueFilter) ([]*postgres.IssueRow, int, error) {
	if q.idx >= len(q.pages) {
		return nil, 0, nil
	}
	page := q.pages[q.idx]
	q.idx++
	return page, len(page), nil
}

type fakeIndexer struct {
	batches  [][]search.IndexIssue
	indexErr error
	traceID  string
}

func (f *fakeIndexer) Health(context.Context) error { return nil }

func (f *fakeIndexer) ConfigureIndexes(context.Context) error { return nil }

func (f *fakeIndexer) indexIssues(ctx context.Context, issues []search.IndexIssue) error {
	if f.indexErr != nil {
		return f.indexErr
	}
	if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
		f.traceID = spanContext.TraceID().String()
	}
	copyBatch := append([]search.IndexIssue(nil), issues...)
	f.batches = append(f.batches, copyBatch)
	return nil
}

func (f *fakeIndexer) IndexIssues(ctx context.Context, _ string, issues []search.IndexIssue) error {
	return f.indexIssues(ctx, issues)
}

func (f *fakeIndexer) IndexProject(context.Context, search.IndexProject) error { return nil }

func (f *fakeIndexer) DeleteScanIssues(context.Context, int64) error { return nil }

var _ search.IIndexer = (*fakeIndexer)(nil)

func tracedIndexContext() (context.Context, string) {
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x01, 0x02, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x01, 0x02},
		SpanID:     trace.SpanID{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x01, 0x02},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), spanContext)
	return ctx, spanContext.TraceID().String()
}
