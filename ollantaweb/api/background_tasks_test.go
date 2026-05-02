package api

import (
	"testing"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

func TestNormalizeBackgroundTask_StatusActionsAndStale(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	started := now.Add(-45 * time.Minute)
	job := &postgres.ScanJob{
		ID:         12,
		ProjectKey: "ollanta",
		Status:     "running",
		WorkerID:   "worker-1",
		CreatedAt:  now.Add(-50 * time.Minute),
		UpdatedAt:  now.Add(-40 * time.Minute),
		StartedAt:  &started,
	}

	task := normalizeScanTask(job, now)

	if task.ID != "scan:12" {
		t.Fatalf("unexpected task id %q", task.ID)
	}
	if task.Status != backgroundStatusStale || !task.Stale {
		t.Fatalf("expected stale status, got status=%q stale=%v", task.Status, task.Stale)
	}
	if len(task.SupportedActions) != 1 || task.SupportedActions[0] != backgroundActionRequeue {
		t.Fatalf("unexpected supported actions: %#v", task.SupportedActions)
	}
	if task.DurationSeconds == nil || *task.DurationSeconds != 2700 {
		t.Fatalf("unexpected duration: %#v", task.DurationSeconds)
	}
}

func TestNormalizeBackgroundTask_RetryingAndQueued(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	nextAttempt := now.Add(5 * time.Minute)
	job := &postgres.IndexJob{
		ID:            7,
		ScanID:        99,
		ProjectID:     4,
		ProjectKey:    "ollanta",
		Status:        "accepted",
		Attempts:      2,
		NextAttemptAt: nextAttempt,
		CreatedAt:     now.Add(-10 * time.Minute),
		UpdatedAt:     now.Add(-1 * time.Minute),
	}

	task := normalizeIndexTask(job, now)

	if task.Status != backgroundStatusRetrying {
		t.Fatalf("expected retrying, got %q", task.Status)
	}
	if len(task.SupportedActions) != 1 || task.SupportedActions[0] != backgroundActionRequeue {
		t.Fatalf("unexpected supported actions: %#v", task.SupportedActions)
	}
	if task.ScanID == nil || *task.ScanID != 99 {
		t.Fatalf("expected scan id link, got %#v", task.ScanID)
	}
}

func TestSummarizeBackgroundTasks(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	completedAt := now.Add(-1 * time.Hour)
	tasks := []*backgroundTaskDTO{
		{Type: backgroundTaskTypeScan, Status: backgroundStatusQueued, QueuedAgeSeconds: int64Ptr(120)},
		{Type: backgroundTaskTypeScan, Status: backgroundStatusStale, Stale: true},
		{Type: backgroundTaskTypeIndex, Status: backgroundStatusFailed},
		{Type: backgroundTaskTypeWebhook, Status: backgroundStatusRetrying},
		{Type: backgroundTaskTypeWebhook, Status: backgroundStatusCompleted, CompletedAt: &completedAt},
	}

	summary := summarizeBackgroundTasks(tasks, now)

	if summary.Total != 5 || summary.QueueDepth != 1 || summary.StaleCount != 1 || summary.FailedCount != 1 || summary.RetryCount != 1 || summary.RecentCompletions != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.ByType[backgroundTaskTypeScan].QueueDepth != 1 || summary.ByType[backgroundTaskTypeScan].StaleCount != 1 {
		t.Fatalf("unexpected scan summary: %#v", summary.ByType[backgroundTaskTypeScan])
	}
}

func TestPaginateBounds(t *testing.T) {
	start, end := paginateBounds(7, 3, 6)
	if start != 6 || end != 7 {
		t.Fatalf("unexpected bounds %d:%d", start, end)
	}
	start, end = paginateBounds(7, 3, 10)
	if start != 7 || end != 7 {
		t.Fatalf("unexpected capped bounds %d:%d", start, end)
	}
}

func TestParseBackgroundTaskID_AcceptsEscapedColon(t *testing.T) {
	taskType, id, err := parseBackgroundTaskID("index%3A22")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskType != backgroundTaskTypeIndex || id != 22 {
		t.Fatalf("unexpected parsed task id type=%q id=%d", taskType, id)
	}
}

func TestScannerParametersFromPayload(t *testing.T) {
	payload := []byte(`{
		"metadata":{"project_key":"ollanta","branch":"main","commit_sha":"abc123"},
		"scanner_options":{"project_dir":"/workspace/ollanta","sources":["./..."],"format":"all","tests":{"enabled":true,"mode":"run","modules":[{"name":"api","root":"ollantaweb","command":"go test ./api"}]}},
		"test_signals":{"summary":{"enabled":true,"modules":1}}
	}`)

	params := scannerParametersFromPayload(payload)
	if params == nil {
		t.Fatal("expected scanner parameters")
	}
	options, ok := params["scanner_options"].(map[string]any)
	if !ok {
		t.Fatalf("scanner_options missing or wrong type: %#v", params["scanner_options"])
	}
	if options["project_dir"] != "/workspace/ollanta" {
		t.Fatalf("project_dir = %#v", options["project_dir"])
	}
	scope, ok := params["analysis_scope"].(map[string]any)
	if !ok || scope["branch"] != "main" {
		t.Fatalf("analysis_scope = %#v", params["analysis_scope"])
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}
