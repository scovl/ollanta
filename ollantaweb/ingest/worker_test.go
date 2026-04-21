package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
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

type fakeIndexJobStore struct {
	created        *postgres.IndexJob
	next           *postgres.IndexJob
	rescheduledID  int64
	rescheduledErr string
	completedID    int64
	failedID       int64
}

func (s *fakeIndexJobStore) Create(_ context.Context, job *postgres.IndexJob) error {
	s.created = job
	return nil
}

func (s *fakeIndexJobStore) ClaimNext(_ context.Context, _ string) (*postgres.IndexJob, error) {
	if s.next == nil {
		return nil, postgres.ErrNotFound
	}
	return s.next, nil
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
	batches  [][]postgres.IssueRow
	indexErr error
}

func (f *fakeIndexer) Health(context.Context) error { return nil }

func (f *fakeIndexer) ConfigureIndexes(context.Context) error { return nil }

func (f *fakeIndexer) IndexIssues(_ context.Context, _ string, issues []postgres.IssueRow) error {
	if f.indexErr != nil {
		return f.indexErr
	}
	copyBatch := append([]postgres.IssueRow(nil), issues...)
	f.batches = append(f.batches, copyBatch)
	return nil
}

func (f *fakeIndexer) IndexProject(context.Context, *postgres.Project) error { return nil }

func (f *fakeIndexer) DeleteScanIssues(context.Context, int64) error { return nil }

func (f *fakeIndexer) ReindexAll(context.Context, *postgres.IssueRepository, *postgres.ProjectRepository) error {
	return nil
}

var _ search.IIndexer = (*fakeIndexer)(nil)
