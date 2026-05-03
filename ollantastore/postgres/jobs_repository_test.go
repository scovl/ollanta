package postgres

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func openJobRepositoryTestDB(t *testing.T) (*DB, context.Context, string) {
	t.Helper()

	databaseURL := os.Getenv("OLLANTA_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("set OLLANTA_TEST_DATABASE_URL or DATABASE_URL to run PostgreSQL repository tests")
	}

	ctx := context.Background()
	db, err := New(ctx, databaseURL, PoolConfig{
		MaxConns:        4,
		MinConns:        0,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: time.Minute,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := db.Migrate(ctx); err != nil {
		db.Close()
		t.Fatalf("Migrate() error = %v", err)
	}

	prefix := fmt.Sprintf("jobs-test-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = db.Pool.Exec(context.Background(), "DELETE FROM scan_jobs WHERE project_key LIKE $1", prefix+"%")
		_, _ = db.Pool.Exec(context.Background(), "DELETE FROM projects WHERE key LIKE $1", prefix+"%")
		db.Close()
	})
	return db, ctx, prefix
}

func createJobTestProjectAndScan(t *testing.T, db *DB, ctx context.Context, projectKey string) (int64, int64) {
	t.Helper()

	var projectID int64
	if err := db.Pool.QueryRow(ctx, `
		INSERT INTO projects (key, name)
		VALUES ($1, $1)
		RETURNING id`, projectKey,
	).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	var scanID int64
	if err := db.Pool.QueryRow(ctx, `
		INSERT INTO scans (project_id, version, status)
		VALUES ($1, 'test', 'completed')
		RETURNING id`, projectID,
	).Scan(&scanID); err != nil {
		t.Fatalf("insert scan: %v", err)
	}
	return projectID, scanID
}

func createJobTestWebhook(t *testing.T, db *DB, ctx context.Context, projectID int64, name string) int64 {
	t.Helper()

	var webhookID int64
	if err := db.Pool.QueryRow(ctx, `
		INSERT INTO webhooks (project_id, name, url, events)
		VALUES ($1, $2, 'https://example.invalid/hook', ARRAY['scan.completed'])
		RETURNING id`, projectID, name,
	).Scan(&webhookID); err != nil {
		t.Fatalf("insert webhook: %v", err)
	}
	return webhookID
}

func TestScanJobRepository_IdempotencyPressureAndRecovery(t *testing.T) {
	db, ctx, prefix := openJobRepositoryTestDB(t)
	repo := NewScanJobRepository(db)
	projectKey := prefix + "-scan"

	job := createAcceptedScanJob(t, repo, ctx, projectKey, "key-1", []byte(`{"metadata":{"project_key":"demo"}}`))
	assertScanJobIdempotency(t, repo, ctx, projectKey, job)
	assertDuplicateScanJobRejected(t, repo, ctx, projectKey)
	createAgedAcceptedScanJob(t, db, repo, ctx, projectKey)
	requeuedJob, failedJob := createStaleScanJobs(t, db, repo, ctx, projectKey)
	assertScanQueuePressure(t, repo, ctx, projectKey)
	assertScanJobRecovery(t, repo, ctx, requeuedJob.ID, failedJob.ID)
}

func createAcceptedScanJob(t *testing.T, repo *ScanJobRepository, ctx context.Context, projectKey, key string, payload []byte) *ScanJob {
	t.Helper()
	job := &ScanJob{ProjectKey: projectKey, Status: "accepted", Payload: payload, IdempotencyKey: key, PayloadHash: HashPayload(payload)}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return job
}

func assertScanJobIdempotency(t *testing.T, repo *ScanJobRepository, ctx context.Context, projectKey string, job *ScanJob) {
	t.Helper()
	found, err := repo.FindByIdempotencyKey(ctx, projectKey, job.IdempotencyKey)
	if err != nil {
		t.Fatalf("FindByIdempotencyKey() error = %v", err)
	}
	if found.ID != job.ID || !PayloadHashesEqual(found.PayloadHash, job.PayloadHash) {
		t.Fatalf("found job = %+v, want id %d and matching hash", found, job.ID)
	}
	if PayloadHashesEqual(found.PayloadHash, HashPayload([]byte(`{"different":true}`))) {
		t.Fatal("PayloadHashesEqual() matched different payload hashes")
	}
}

func assertDuplicateScanJobRejected(t *testing.T, repo *ScanJobRepository, ctx context.Context, projectKey string) {
	t.Helper()
	payload := []byte(`{"metadata":{"project_key":"demo"},"changed":true}`)
	duplicate := &ScanJob{ProjectKey: projectKey, Status: "accepted", Payload: payload, IdempotencyKey: "key-1", PayloadHash: HashPayload(payload)}
	if err := repo.Create(ctx, duplicate); err == nil {
		t.Fatal("Create() duplicate idempotency key error = nil, want unique constraint error")
	}
}

func createAgedAcceptedScanJob(t *testing.T, db *DB, repo *ScanJobRepository, ctx context.Context, projectKey string) {
	t.Helper()
	job := createAcceptedScanJob(t, repo, ctx, projectKey, "key-2", []byte(`{"old":true}`))
	createdAt := time.Now().UTC().Add(-2 * time.Hour)
	if _, err := db.Pool.Exec(ctx, "UPDATE scan_jobs SET created_at = $1 WHERE id = $2", createdAt, job.ID); err != nil {
		t.Fatalf("age accepted job: %v", err)
	}
}

func createStaleScanJobs(t *testing.T, db *DB, repo *ScanJobRepository, ctx context.Context, projectKey string) (*ScanJob, *ScanJob) {
	t.Helper()
	requeuedJob := createRunningScanJob(t, repo, ctx, projectKey, "key-3", []byte(`{"requeue":true}`), 1)
	failedJob := createRunningScanJob(t, repo, ctx, projectKey, "key-4", []byte(`{"fail":true}`), 3)
	ageScanJobs(t, db, ctx, requeuedJob.ID, failedJob.ID)
	return requeuedJob, failedJob
}

func createRunningScanJob(t *testing.T, repo *ScanJobRepository, ctx context.Context, projectKey, key string, payload []byte, attempts int) *ScanJob {
	t.Helper()
	job := &ScanJob{ProjectKey: projectKey, Status: "running", Payload: payload, IdempotencyKey: key, PayloadHash: HashPayload(payload), Attempts: attempts}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("Create(stale job) error = %v", err)
	}
	return job
}

func ageScanJobs(t *testing.T, db *DB, ctx context.Context, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		if _, err := db.Pool.Exec(ctx, "UPDATE scan_jobs SET updated_at = $1 WHERE id = $2", time.Now().UTC().Add(-time.Hour), id); err != nil {
			t.Fatalf("age stale scan job: %v", err)
		}
	}
}

func assertScanQueuePressure(t *testing.T, repo *ScanJobRepository, ctx context.Context, projectKey string) {
	t.Helper()
	pressure, err := repo.QueuePressure(ctx, projectKey, time.Now().UTC())
	if err != nil {
		t.Fatalf("QueuePressure() error = %v", err)
	}
	if pressure.Accepted != 2 || pressure.Running != 2 || pressure.OldestAcceptedAge < time.Hour {
		t.Fatalf("QueuePressure() = %+v, want accepted=2 running=2 oldest>=1h", pressure)
	}
}

func assertScanJobRecovery(t *testing.T, repo *ScanJobRepository, ctx context.Context, requeuedID, failedID int64) {
	t.Helper()
	result, err := repo.RecoverStale(ctx, time.Now().UTC().Add(-time.Minute), 3, "stale")
	if err != nil {
		t.Fatalf("RecoverStale() error = %v", err)
	}
	if result.Requeued != 1 || result.Failed != 1 {
		t.Fatalf("RecoverStale() = %+v, want 1 requeued and 1 failed", result)
	}
	assertRecoveredScanJobStatuses(t, repo, ctx, requeuedID, failedID)
}

func assertRecoveredScanJobStatuses(t *testing.T, repo *ScanJobRepository, ctx context.Context, requeuedID, failedID int64) {
	t.Helper()
	requeued, err := repo.GetByID(ctx, requeuedID)
	if err != nil {
		t.Fatalf("GetByID(requeued) error = %v", err)
	}
	failed, err := repo.GetByID(ctx, failedID)
	if err != nil {
		t.Fatalf("GetByID(failed) error = %v", err)
	}
	if requeued.Status != "accepted" || failed.Status != "failed" || failed.LastError != "stale" {
		t.Fatalf("recovery statuses = requeued:%s failed:%s/%s", requeued.Status, failed.Status, failed.LastError)
	}
}

func TestScanJobRepository_ConcurrentStaleRecoveryDoesNotDoubleRecover(t *testing.T) {
	db, ctx, prefix := openJobRepositoryTestDB(t)
	repo := NewScanJobRepository(db)
	projectKey := prefix + "-scan-race"

	staleJob := &ScanJob{
		ProjectKey:     projectKey,
		Status:         "running",
		Payload:        []byte(`{"race":true}`),
		IdempotencyKey: "race-key",
		PayloadHash:    HashPayload([]byte(`{"race":true}`)),
		Attempts:       1,
	}
	if err := repo.Create(ctx, staleJob); err != nil {
		t.Fatalf("Create(stale job) error = %v", err)
	}
	if _, err := db.Pool.Exec(ctx, "UPDATE scan_jobs SET updated_at = $1 WHERE id = $2", time.Now().UTC().Add(-time.Hour), staleJob.ID); err != nil {
		t.Fatalf("age stale scan job: %v", err)
	}

	start := make(chan struct{})
	results := make(chan JobRecoveryResult, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := repo.RecoverStale(ctx, time.Now().UTC().Add(-time.Minute), 3, "stale")
			results <- result
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("RecoverStale() concurrent error = %v", err)
		}
	}
	var totalRequeued int64
	for result := range results {
		totalRequeued += result.Requeued
	}
	if totalRequeued != 1 {
		t.Fatalf("total requeued = %d, want exactly one recovery", totalRequeued)
	}
}

func TestIndexJobRepository_ActiveDedupeAndRecovery(t *testing.T) {
	db, ctx, prefix := openJobRepositoryTestDB(t)
	repo := NewIndexJobRepository(db)
	projectID, scanID := createJobTestProjectAndScan(t, db, ctx, prefix+"-index")

	job := &IndexJob{ScanID: scanID, ProjectID: projectID, ProjectKey: prefix + "-index", Status: "accepted"}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	active, err := repo.GetActiveByScanID(ctx, scanID)
	if err != nil {
		t.Fatalf("GetActiveByScanID() error = %v", err)
	}
	if active.ID != job.ID {
		t.Fatalf("active job id = %d, want %d", active.ID, job.ID)
	}
	if err := repo.Create(ctx, &IndexJob{ScanID: scanID, ProjectID: projectID, ProjectKey: prefix + "-index", Status: "accepted"}); err == nil {
		t.Fatal("Create() duplicate active index job error = nil, want unique constraint error")
	}

	_, requeueScanID := createJobTestProjectAndScan(t, db, ctx, prefix+"-index-requeue")
	_, failScanID := createJobTestProjectAndScan(t, db, ctx, prefix+"-index-fail")
	requeuedJob := &IndexJob{ScanID: requeueScanID, ProjectID: projectID, ProjectKey: prefix + "-index", Status: "running", Attempts: 1}
	failedJob := &IndexJob{ScanID: failScanID, ProjectID: projectID, ProjectKey: prefix + "-index", Status: "running", Attempts: 3}
	for _, staleJob := range []*IndexJob{requeuedJob, failedJob} {
		if err := repo.Create(ctx, staleJob); err != nil {
			t.Fatalf("Create(stale index job) error = %v", err)
		}
		if _, err := db.Pool.Exec(ctx, "UPDATE index_jobs SET updated_at = $1 WHERE id = $2", time.Now().UTC().Add(-time.Hour), staleJob.ID); err != nil {
			t.Fatalf("age stale index job: %v", err)
		}
	}

	result, err := repo.RecoverStale(ctx, time.Now().UTC().Add(-time.Minute), 3, "stale")
	if err != nil {
		t.Fatalf("RecoverStale() error = %v", err)
	}
	if result.Requeued != 1 || result.Failed != 1 {
		t.Fatalf("RecoverStale() = %+v, want 1 requeued and 1 failed", result)
	}
}

func TestWebhookJobRepository_ActiveDedupeAndRecovery(t *testing.T) {
	db, ctx, prefix := openJobRepositoryTestDB(t)
	repo := NewWebhookJobRepository(db)
	projectID, _ := createJobTestProjectAndScan(t, db, ctx, prefix+"-webhook")
	webhookID := createJobTestWebhook(t, db, ctx, projectID, prefix+"-webhook")

	payload := []byte(`{"scan_id":1}`)
	payloadHash := HashPayload(payload)
	job := &WebhookJob{WebhookID: webhookID, ProjectID: &projectID, Event: "scan.completed", Payload: payload, PayloadHash: payloadHash, Status: "accepted"}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	active, err := repo.GetActiveByIdentity(ctx, webhookID, "scan.completed", payloadHash)
	if err != nil {
		t.Fatalf("GetActiveByIdentity() error = %v", err)
	}
	if active.ID != job.ID {
		t.Fatalf("active job id = %d, want %d", active.ID, job.ID)
	}
	if err := repo.Create(ctx, &WebhookJob{WebhookID: webhookID, ProjectID: &projectID, Event: "scan.completed", Payload: payload, PayloadHash: payloadHash, Status: "accepted"}); err == nil {
		t.Fatal("Create() duplicate active webhook job error = nil, want unique constraint error")
	}

	requeuedPayload := []byte(`{"scan_id":2}`)
	failedPayload := []byte(`{"scan_id":3}`)
	requeuedJob := &WebhookJob{WebhookID: webhookID, ProjectID: &projectID, Event: "scan.completed", Payload: requeuedPayload, PayloadHash: HashPayload(requeuedPayload), Status: "running", Attempts: 1}
	failedJob := &WebhookJob{WebhookID: webhookID, ProjectID: &projectID, Event: "scan.completed", Payload: failedPayload, PayloadHash: HashPayload(failedPayload), Status: "running", Attempts: 3}
	for _, staleJob := range []*WebhookJob{requeuedJob, failedJob} {
		if err := repo.Create(ctx, staleJob); err != nil {
			t.Fatalf("Create(stale webhook job) error = %v", err)
		}
		if _, err := db.Pool.Exec(ctx, "UPDATE webhook_jobs SET updated_at = $1 WHERE id = $2", time.Now().UTC().Add(-time.Hour), staleJob.ID); err != nil {
			t.Fatalf("age stale webhook job: %v", err)
		}
	}

	result, err := repo.RecoverStale(ctx, time.Now().UTC().Add(-time.Minute), 3, "stale")
	if err != nil {
		t.Fatalf("RecoverStale() error = %v", err)
	}
	if result.Requeued != 1 || result.Failed != 1 {
		t.Fatalf("RecoverStale() = %+v, want 1 requeued and 1 failed", result)
	}
}
