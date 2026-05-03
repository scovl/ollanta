package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

const (
	backgroundTaskTypeScan    = "scan"
	backgroundTaskTypeIndex   = "index"
	backgroundTaskTypeWebhook = "webhook"

	backgroundStatusQueued    = "queued"
	backgroundStatusRunning   = "running"
	backgroundStatusRetrying  = "retrying"
	backgroundStatusCompleted = "completed"
	backgroundStatusFailed    = "failed"
	backgroundStatusCancelled = "cancelled"
	backgroundStatusStale     = "stale"

	backgroundActionRetry   = "retry"
	backgroundActionRequeue = "requeue"
	backgroundActionCancel  = "cancel"

	invalidBackgroundTaskIDMessage = "invalid background task id"
)

var backgroundTaskStaleThresholds = map[string]time.Duration{
	backgroundTaskTypeScan:    30 * time.Minute,
	backgroundTaskTypeIndex:   10 * time.Minute,
	backgroundTaskTypeWebhook: 10 * time.Minute,
}

// BackgroundTasksHandler exposes the canonical admin view over durable jobs.
type BackgroundTasksHandler struct {
	scanJobs    *postgres.ScanJobRepository
	indexJobs   *postgres.IndexJobRepository
	webhookJobs *postgres.WebhookJobRepository
	metricsReg  *telemetry.Registry
}

// NewBackgroundTasksHandler creates the durable background-task admin handler.
func NewBackgroundTasksHandler(scanJobs *postgres.ScanJobRepository, indexJobs *postgres.IndexJobRepository, webhookJobs *postgres.WebhookJobRepository, metricsReg *telemetry.Registry) *BackgroundTasksHandler {
	return &BackgroundTasksHandler{scanJobs: scanJobs, indexJobs: indexJobs, webhookJobs: webhookJobs, metricsReg: metricsReg}
}

// StartBackgroundTaskMetricsLoop refreshes durable job metrics without relying on admin endpoint traffic.
func StartBackgroundTaskMetricsLoop(ctx context.Context, handler *BackgroundTasksHandler, interval time.Duration) {
	if handler == nil || interval <= 0 {
		return
	}
	go func() {
		handler.refreshMetrics(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				handler.refreshMetrics(ctx)
			}
		}
	}()
}

func (h *BackgroundTasksHandler) refreshMetrics(ctx context.Context) {
	filter := backgroundTaskFilter{Limit: 500, Offset: 0}
	tasks, err := h.loadBackgroundTasks(ctx, filter)
	if err != nil {
		slog.WarnContext(ctx, "refresh background task metrics", "error", err)
		return
	}
	h.observeSummary(summarizeBackgroundTasks(tasks, time.Now().UTC()))
}

type backgroundTaskDTO struct {
	ID               string         `json:"id"`
	Type             string         `json:"type"`
	SourceJobID      int64          `json:"source_job_id"`
	Status           string         `json:"status"`
	InternalStatus   string         `json:"internal_status"`
	ProjectKey       string         `json:"project_key,omitempty"`
	ProjectID        *int64         `json:"project_id,omitempty"`
	ScanID           *int64         `json:"scan_id,omitempty"`
	WorkerID         string         `json:"worker_id,omitempty"`
	Attempts         int            `json:"attempts"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	StartedAt        *time.Time     `json:"started_at,omitempty"`
	CompletedAt      *time.Time     `json:"completed_at,omitempty"`
	NextAttemptAt    *time.Time     `json:"next_attempt_at,omitempty"`
	QueuedAgeSeconds *int64         `json:"queued_age_seconds,omitempty"`
	DurationSeconds  *int64         `json:"duration_seconds,omitempty"`
	Stale            bool           `json:"stale"`
	StaleSeconds     *int64         `json:"stale_seconds,omitempty"`
	LastError        string         `json:"last_error,omitempty"`
	SupportedActions []string       `json:"supported_actions"`
	ScannerParams    map[string]any `json:"scanner_parameters,omitempty"`
	Details          map[string]any `json:"details,omitempty"`
	Links            map[string]any `json:"links,omitempty"`
}

type backgroundTaskFilter struct {
	TaskType       string
	Status         string
	InternalStatus string
	ProjectKey     string
	ScanID         *int64
	WorkerID       string
	FailedOnly     bool
	StaleOnly      bool
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
	Limit          int
	Offset         int
}

type backgroundTaskSummary struct {
	Total             int                                 `json:"total"`
	QueueDepth        int                                 `json:"queue_depth"`
	RunningCount      int                                 `json:"running_count"`
	FailedCount       int                                 `json:"failed_count"`
	StaleCount        int                                 `json:"stale_count"`
	RetryCount        int                                 `json:"retry_count"`
	RecentCompletions int                                 `json:"recent_completion_count"`
	OldestQueuedAge   *int64                              `json:"oldest_queued_age_seconds,omitempty"`
	ByType            map[string]backgroundTaskTypeHealth `json:"by_type"`
}

type backgroundTaskTypeHealth struct {
	Total             int    `json:"total"`
	QueueDepth        int    `json:"queue_depth"`
	RunningCount      int    `json:"running_count"`
	FailedCount       int    `json:"failed_count"`
	StaleCount        int    `json:"stale_count"`
	RetryCount        int    `json:"retry_count"`
	RecentCompletions int    `json:"recent_completion_count"`
	OldestQueuedAge   *int64 `json:"oldest_queued_age_seconds,omitempty"`
}

type backgroundTaskParams struct {
	TaskType       string
	ID             int64
	InternalStatus string
	ProjectKey     string
	ProjectID      *int64
	ScanID         *int64
	WorkerID       string
	Attempts       int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	NextAttemptAt  *time.Time
	LastError      string
	Now            time.Time
}

// List handles GET /api/v1/admin/background-tasks.
func (h *BackgroundTasksHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := parseBackgroundTaskFilter(r)
	tasks, err := h.loadBackgroundTasks(r.Context(), filter)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	total := len(tasks)
	start, end := paginateBounds(total, filter.Limit, filter.Offset)
	jsonOK(w, http.StatusOK, map[string]any{
		"items":  tasks[start:end],
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// Summary handles GET /api/v1/admin/background-tasks/summary.
func (h *BackgroundTasksHandler) Summary(w http.ResponseWriter, r *http.Request) {
	filter := parseBackgroundTaskFilter(r)
	filter.Limit = 500
	filter.Offset = 0
	tasks, err := h.loadBackgroundTasks(r.Context(), filter)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	summary := summarizeBackgroundTasks(tasks, time.Now().UTC())
	h.observeSummary(summary)
	jsonOK(w, http.StatusOK, summary)
}

// Detail handles GET /api/v1/admin/background-tasks/{taskID}.
func (h *BackgroundTasksHandler) Detail(w http.ResponseWriter, r *http.Request) {
	task, err := h.loadBackgroundTaskByID(r.Context(), routeParam(r, "taskID"))
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "background task not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, task)
}

// Retry handles POST /api/v1/admin/background-tasks/{taskID}/retry.
func (h *BackgroundTasksHandler) Retry(w http.ResponseWriter, r *http.Request) {
	h.runAction(w, r, backgroundActionRetry)
}

// Requeue handles POST /api/v1/admin/background-tasks/{taskID}/requeue.
func (h *BackgroundTasksHandler) Requeue(w http.ResponseWriter, r *http.Request) {
	h.runAction(w, r, backgroundActionRequeue)
}

// Cancel handles POST /api/v1/admin/background-tasks/{taskID}/cancel.
func (h *BackgroundTasksHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	h.runAction(w, r, backgroundActionCancel)
}

func (h *BackgroundTasksHandler) runAction(w http.ResponseWriter, r *http.Request, action string) {
	task, err := h.loadBackgroundTaskByID(r.Context(), routeParam(r, "taskID"))
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "background task not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !stringIn(action, task.SupportedActions) {
		jsonError(w, http.StatusConflict, fmt.Sprintf("%s is not supported for %s task in %s state", action, task.Type, task.Status))
		return
	}

	if err := h.applyBackgroundTaskAction(r.Context(), task, action); errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusConflict, "task state changed; refresh and try again")
		return
	} else if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := h.loadBackgroundTaskByID(r.Context(), task.ID)
	if err != nil {
		jsonOK(w, http.StatusAccepted, map[string]any{"id": task.ID, "status": backgroundStatusQueued})
		return
	}
	jsonOK(w, http.StatusAccepted, updated)
}

func (h *BackgroundTasksHandler) applyBackgroundTaskAction(ctx context.Context, task *backgroundTaskDTO, action string) error {
	switch action {
	case backgroundActionRetry, backgroundActionRequeue:
		switch task.Type {
		case backgroundTaskTypeScan:
			return h.scanJobs.Retry(ctx, task.SourceJobID)
		case backgroundTaskTypeIndex:
			return h.indexJobs.Retry(ctx, task.SourceJobID)
		case backgroundTaskTypeWebhook:
			return h.webhookJobs.Retry(ctx, task.SourceJobID)
		}
	case backgroundActionCancel:
		switch task.Type {
		case backgroundTaskTypeScan:
			return h.scanJobs.CancelQueued(ctx, task.SourceJobID)
		case backgroundTaskTypeIndex:
			return h.indexJobs.CancelQueued(ctx, task.SourceJobID)
		case backgroundTaskTypeWebhook:
			return h.webhookJobs.CancelQueued(ctx, task.SourceJobID)
		}
	}
	return fmt.Errorf("unsupported action %q", action)
}

func (h *BackgroundTasksHandler) loadBackgroundTasks(ctx context.Context, filter backgroundTaskFilter) ([]*backgroundTaskDTO, error) {
	queryFilter := postgres.JobListFilter{
		Status:        filter.InternalStatus,
		ProjectKey:    filter.ProjectKey,
		ScanID:        filter.ScanID,
		WorkerID:      filter.WorkerID,
		CreatedAfter:  filter.CreatedAfter,
		CreatedBefore: filter.CreatedBefore,
		Limit:         500,
	}
	now := time.Now().UTC()
	tasks := []*backgroundTaskDTO{}

	if shouldLoadTaskType(filter, backgroundTaskTypeScan) {
		scanTasks, err := h.loadScanBackgroundTasks(ctx, queryFilter, now)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, scanTasks...)
	}
	if shouldLoadTaskType(filter, backgroundTaskTypeIndex) {
		indexTasks, err := h.loadIndexBackgroundTasks(ctx, queryFilter, now)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, indexTasks...)
	}
	if shouldLoadTaskType(filter, backgroundTaskTypeWebhook) {
		webhookTasks, err := h.loadWebhookBackgroundTasks(ctx, queryFilter, now)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, webhookTasks...)
	}

	filtered := filterBackgroundTasks(tasks, filter)
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].ID > filtered[j].ID
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	return filtered, nil
}

func shouldLoadTaskType(filter backgroundTaskFilter, taskType string) bool {
	return filter.TaskType == "" || filter.TaskType == taskType
}

func (h *BackgroundTasksHandler) loadScanBackgroundTasks(ctx context.Context, filter postgres.JobListFilter, now time.Time) ([]*backgroundTaskDTO, error) {
	jobs, _, err := h.scanJobs.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	tasks := make([]*backgroundTaskDTO, 0, len(jobs))
	for _, job := range jobs {
		tasks = append(tasks, normalizeScanTask(job, now))
	}
	return tasks, nil
}

func (h *BackgroundTasksHandler) loadIndexBackgroundTasks(ctx context.Context, filter postgres.JobListFilter, now time.Time) ([]*backgroundTaskDTO, error) {
	jobs, _, err := h.indexJobs.ListFiltered(ctx, filter)
	if err != nil {
		return nil, err
	}
	tasks := make([]*backgroundTaskDTO, 0, len(jobs))
	for _, job := range jobs {
		tasks = append(tasks, normalizeIndexTask(job, now))
	}
	return tasks, nil
}

func (h *BackgroundTasksHandler) loadWebhookBackgroundTasks(ctx context.Context, filter postgres.JobListFilter, now time.Time) ([]*backgroundTaskDTO, error) {
	jobs, _, err := h.webhookJobs.ListFiltered(ctx, filter)
	if err != nil {
		return nil, err
	}
	tasks := make([]*backgroundTaskDTO, 0, len(jobs))
	for _, job := range jobs {
		tasks = append(tasks, normalizeWebhookTask(job, now))
	}
	return tasks, nil
}

func filterBackgroundTasks(tasks []*backgroundTaskDTO, filter backgroundTaskFilter) []*backgroundTaskDTO {
	filtered := tasks[:0]
	for _, task := range tasks {
		if matchesBackgroundTaskFilter(task, filter) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func matchesBackgroundTaskFilter(task *backgroundTaskDTO, filter backgroundTaskFilter) bool {
	if filter.Status != "" && task.Status != filter.Status {
		return false
	}
	if filter.FailedOnly && task.Status != backgroundStatusFailed {
		return false
	}
	return !filter.StaleOnly || task.Stale
}

func (h *BackgroundTasksHandler) loadBackgroundTaskByID(ctx context.Context, taskID string) (*backgroundTaskDTO, error) {
	taskType, id, err := parseBackgroundTaskID(taskID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	switch taskType {
	case backgroundTaskTypeScan:
		job, err := h.scanJobs.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		return normalizeScanTask(job, now), nil
	case backgroundTaskTypeIndex:
		job, err := h.indexJobs.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		task := normalizeIndexTask(job, now)
		if task.ScanID != nil {
			h.enrichTaskWithScanPayload(ctx, task, *task.ScanID)
		}
		return task, nil
	case backgroundTaskTypeWebhook:
		job, err := h.webhookJobs.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		return normalizeWebhookTask(job, now), nil
	default:
		return nil, fmt.Errorf("unsupported background task type %q", taskType)
	}
}

func normalizeScanTask(job *postgres.ScanJob, now time.Time) *backgroundTaskDTO {
	scanID := job.ScanID
	task := newBackgroundTask(backgroundTaskParams{
		TaskType:       backgroundTaskTypeScan,
		ID:             job.ID,
		InternalStatus: job.Status,
		ProjectKey:     job.ProjectKey,
		ScanID:         scanID,
		WorkerID:       job.WorkerID,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
		StartedAt:      job.StartedAt,
		CompletedAt:    job.CompletedAt,
		LastError:      job.LastError,
		Now:            now,
	})
	task.ScannerParams = scannerParametersFromPayload(job.Payload)
	task.Details = map[string]any{
		"source_job_type": "scan_job",
		"project_key":     job.ProjectKey,
	}
	if scanID != nil {
		task.Details["produced_scan_id"] = *scanID
	}
	return task
}

func (h *BackgroundTasksHandler) enrichTaskWithScanPayload(ctx context.Context, task *backgroundTaskDTO, scanID int64) {
	if h == nil || h.scanJobs == nil || task == nil {
		return
	}
	job, err := h.scanJobs.GetByScanID(ctx, scanID)
	if err != nil {
		return
	}
	task.ScannerParams = scannerParametersFromPayload(job.Payload)
}

func scannerParametersFromPayload(payload []byte) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	var report struct {
		Metadata       map[string]any  `json:"metadata"`
		ScannerOptions map[string]any  `json:"scanner_options"`
		TestSignals    json.RawMessage `json:"test_signals"`
	}
	if err := json.Unmarshal(payload, &report); err != nil {
		return nil
	}
	params := map[string]any{}
	if len(report.ScannerOptions) > 0 {
		params["scanner_options"] = report.ScannerOptions
	}
	if len(report.Metadata) > 0 {
		params["analysis_scope"] = report.Metadata
	}
	if tests := scannerTestSignalParameters(report.TestSignals); len(tests) > 0 {
		params["test_signals"] = tests
	}
	if len(params) == 0 {
		return nil
	}
	return params
}

func scannerTestSignalParameters(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var testSignals struct {
		Summary map[string]any   `json:"summary"`
		Modules []map[string]any `json:"modules"`
	}
	if err := json.Unmarshal(raw, &testSignals); err != nil {
		return nil
	}
	tests := map[string]any{}
	if len(testSignals.Summary) > 0 {
		tests["summary"] = testSignals.Summary
	}
	if len(testSignals.Modules) > 0 {
		tests["modules"] = testSignals.Modules
	}
	return tests
}

func normalizeIndexTask(job *postgres.IndexJob, now time.Time) *backgroundTaskDTO {
	scanID := job.ScanID
	projectID := job.ProjectID
	nextAttemptAt := job.NextAttemptAt
	task := newBackgroundTask(backgroundTaskParams{
		TaskType:       backgroundTaskTypeIndex,
		ID:             job.ID,
		InternalStatus: job.Status,
		ProjectKey:     job.ProjectKey,
		ProjectID:      &projectID,
		ScanID:         &scanID,
		WorkerID:       job.WorkerID,
		Attempts:       job.Attempts,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
		StartedAt:      job.StartedAt,
		CompletedAt:    job.CompletedAt,
		NextAttemptAt:  &nextAttemptAt,
		LastError:      job.LastError,
		Now:            now,
	})
	task.Details = map[string]any{
		"source_job_type": "index_job",
		"project_id":      job.ProjectID,
		"project_key":     job.ProjectKey,
		"scan_id":         job.ScanID,
	}
	return task
}

func normalizeWebhookTask(job *postgres.WebhookJob, now time.Time) *backgroundTaskDTO {
	nextAttemptAt := job.NextAttemptAt
	task := newBackgroundTask(backgroundTaskParams{
		TaskType:       backgroundTaskTypeWebhook,
		ID:             job.ID,
		InternalStatus: job.Status,
		ProjectID:      job.ProjectID,
		WorkerID:       job.WorkerID,
		Attempts:       job.Attempts,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
		StartedAt:      job.StartedAt,
		CompletedAt:    job.CompletedAt,
		NextAttemptAt:  &nextAttemptAt,
		LastError:      job.LastError,
		Now:            now,
	})
	task.Details = map[string]any{
		"source_job_type": "webhook_job",
		"webhook_id":      job.WebhookID,
		"event":           job.Event,
	}
	if job.ProjectID != nil {
		task.Details["project_id"] = *job.ProjectID
	}
	if job.LastResponseCode != nil {
		task.Details["last_response_code"] = *job.LastResponseCode
	}
	if job.LastResponseBody != nil {
		task.Details["last_response_body"] = *job.LastResponseBody
	}
	return task
}

func newBackgroundTask(params backgroundTaskParams) *backgroundTaskDTO {
	status := normalizeBackgroundStatus(params.InternalStatus, params.TaskType, params.StartedAt, params.NextAttemptAt, params.Now)
	task := &backgroundTaskDTO{
		ID:             fmt.Sprintf("%s:%d", params.TaskType, params.ID),
		Type:           params.TaskType,
		SourceJobID:    params.ID,
		Status:         status,
		InternalStatus: params.InternalStatus,
		ProjectKey:     params.ProjectKey,
		ProjectID:      params.ProjectID,
		ScanID:         params.ScanID,
		WorkerID:       params.WorkerID,
		Attempts:       params.Attempts,
		CreatedAt:      params.CreatedAt,
		UpdatedAt:      params.UpdatedAt,
		StartedAt:      params.StartedAt,
		CompletedAt:    params.CompletedAt,
		NextAttemptAt:  params.NextAttemptAt,
		Stale:          status == backgroundStatusStale,
		LastError:      params.LastError,
		Links:          map[string]any{},
	}
	if status == backgroundStatusQueued || status == backgroundStatusRetrying {
		age := int64(params.Now.Sub(params.CreatedAt).Seconds())
		if age < 0 {
			age = 0
		}
		task.QueuedAgeSeconds = &age
	}
	if params.StartedAt != nil {
		end := params.Now
		if params.CompletedAt != nil {
			end = *params.CompletedAt
		}
		duration := int64(end.Sub(*params.StartedAt).Seconds())
		if duration < 0 {
			duration = 0
		}
		task.DurationSeconds = &duration
	}
	if task.Stale && params.StartedAt != nil {
		threshold := backgroundTaskStaleThresholds[params.TaskType]
		stale := int64(params.Now.Sub(*params.StartedAt).Seconds() - threshold.Seconds())
		if stale < 0 {
			stale = 0
		}
		task.StaleSeconds = &stale
	}
	task.SupportedActions = supportedBackgroundActions(task)
	if params.ProjectKey != "" {
		task.Links["project"] = map[string]string{"key": params.ProjectKey}
	}
	if params.ScanID != nil {
		task.Links["scan"] = map[string]int64{"id": *params.ScanID}
	}
	return task
}

func normalizeBackgroundStatus(internalStatus, taskType string, startedAt, nextAttemptAt *time.Time, now time.Time) string {
	switch internalStatus {
	case "accepted":
		if nextAttemptAt != nil && nextAttemptAt.After(now) {
			return backgroundStatusRetrying
		}
		return backgroundStatusQueued
	case "running":
		if startedAt != nil && now.Sub(*startedAt) > backgroundTaskStaleThresholds[taskType] {
			return backgroundStatusStale
		}
		return backgroundStatusRunning
	case "completed":
		return backgroundStatusCompleted
	case "failed":
		return backgroundStatusFailed
	case "cancelled":
		return backgroundStatusCancelled
	default:
		return internalStatus
	}
}

func supportedBackgroundActions(task *backgroundTaskDTO) []string {
	switch task.Status {
	case backgroundStatusFailed, backgroundStatusCancelled:
		return []string{backgroundActionRetry}
	case backgroundStatusQueued:
		return []string{backgroundActionCancel}
	case backgroundStatusRetrying, backgroundStatusStale:
		return []string{backgroundActionRequeue}
	default:
		return []string{}
	}
}

func parseBackgroundTaskFilter(r *http.Request) backgroundTaskFilter {
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	filter := backgroundTaskFilter{
		TaskType:   strings.TrimSpace(q.Get("type")),
		Status:     normalizeRequestedBackgroundStatus(status),
		ProjectKey: strings.TrimSpace(q.Get("project_key")),
		ScanID:     parseQueryInt64(q.Get("scan_id")),
		WorkerID:   strings.TrimSpace(q.Get("worker_id")),
		FailedOnly: parseQueryBool(q.Get("failed_only")),
		StaleOnly:  parseQueryBool(q.Get("stale_only")),
		Limit:      boundedAPILimit(q.Get("limit")),
		Offset:     parseNonNegativeInt(q.Get("offset")),
	}
	filter.CreatedAfter = parseQueryTime(q.Get("created_after"))
	filter.CreatedBefore = parseQueryTime(q.Get("created_before"))
	filter.InternalStatus = requestedInternalStatus(filter.Status)
	if filter.StaleOnly {
		filter.InternalStatus = "running"
	}
	return filter
}

func parseBackgroundTaskID(taskID string) (string, int64, error) {
	decoded, err := url.PathUnescape(taskID)
	if err != nil {
		return "", 0, errors.New(invalidBackgroundTaskIDMessage)
	}
	taskID = decoded
	parts := strings.SplitN(taskID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", 0, errors.New(invalidBackgroundTaskIDMessage)
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		return "", 0, errors.New(invalidBackgroundTaskIDMessage)
	}
	return parts[0], id, nil
}

func normalizeRequestedBackgroundStatus(status string) string {
	if status == "accepted" {
		return backgroundStatusQueued
	}
	return status
}

func requestedInternalStatus(status string) string {
	switch status {
	case backgroundStatusQueued, backgroundStatusRetrying:
		return "accepted"
	case backgroundStatusStale:
		return "running"
	case backgroundStatusRunning, backgroundStatusCompleted, backgroundStatusFailed, backgroundStatusCancelled:
		return status
	default:
		return ""
	}
}

func boundedAPILimit(value string) int {
	limit := parseNonNegativeInt(value)
	if limit <= 0 {
		return 25
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func parseNonNegativeInt(value string) int {
	n, _ := strconv.Atoi(value)
	if n < 0 {
		return 0
	}
	return n
}

func parseQueryInt64(value string) *int64 {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func parseQueryBool(value string) bool {
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func parseQueryTime(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &parsed
}

func paginateBounds(total, limit, offset int) (int, int) {
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return offset, end
}

func summarizeBackgroundTasks(tasks []*backgroundTaskDTO, now time.Time) backgroundTaskSummary {
	summary := backgroundTaskSummary{ByType: map[string]backgroundTaskTypeHealth{}}
	for _, task := range tasks {
		addBackgroundTaskToSummary(&summary, task, now)
	}
	return summary
}

func addBackgroundTaskToSummary(summary *backgroundTaskSummary, task *backgroundTaskDTO, now time.Time) {
	summary.Total++
	typeSummary := summary.ByType[task.Type]
	typeSummary.Total++
	addBackgroundTaskStatusCounts(summary, &typeSummary, task, now)
	summary.ByType[task.Type] = typeSummary
}

func addBackgroundTaskStatusCounts(summary *backgroundTaskSummary, typeSummary *backgroundTaskTypeHealth, task *backgroundTaskDTO, now time.Time) {
	switch task.Status {
	case backgroundStatusQueued:
		addQueuedBackgroundTask(summary, typeSummary, task)
	case backgroundStatusRunning:
		summary.RunningCount++
		typeSummary.RunningCount++
	case backgroundStatusFailed:
		summary.FailedCount++
		typeSummary.FailedCount++
	case backgroundStatusStale:
		summary.StaleCount++
		typeSummary.StaleCount++
	case backgroundStatusRetrying:
		summary.RetryCount++
		typeSummary.RetryCount++
	case backgroundStatusCompleted:
		addRecentCompletedBackgroundTask(summary, typeSummary, task, now)
	}
}

func addQueuedBackgroundTask(summary *backgroundTaskSummary, typeSummary *backgroundTaskTypeHealth, task *backgroundTaskDTO) {
	summary.QueueDepth++
	typeSummary.QueueDepth++
	if task.QueuedAgeSeconds == nil {
		return
	}
	setMaxAge(&summary.OldestQueuedAge, *task.QueuedAgeSeconds)
	setMaxAge(&typeSummary.OldestQueuedAge, *task.QueuedAgeSeconds)
}

func addRecentCompletedBackgroundTask(summary *backgroundTaskSummary, typeSummary *backgroundTaskTypeHealth, task *backgroundTaskDTO, now time.Time) {
	if task.CompletedAt == nil || now.Sub(*task.CompletedAt) > 24*time.Hour {
		return
	}
	summary.RecentCompletions++
	typeSummary.RecentCompletions++
}

func setMaxAge(target **int64, value int64) {
	if *target == nil || value > **target {
		v := value
		*target = &v
	}
}

func (h *BackgroundTasksHandler) observeSummary(summary backgroundTaskSummary) {
	if h.metricsReg == nil {
		return
	}
	for taskType, item := range summary.ByType {
		h.metricsReg.Gauge("ollanta_background_tasks_"+taskType+"_queued", "Current queued background tasks by type").Set(int64(item.QueueDepth))
		h.metricsReg.Gauge("ollanta_background_tasks_"+taskType+"_running", "Current running background tasks by type").Set(int64(item.RunningCount))
		h.metricsReg.Gauge("ollanta_background_tasks_"+taskType+"_failed", "Current failed background tasks by type").Set(int64(item.FailedCount))
		h.metricsReg.Gauge("ollanta_background_tasks_"+taskType+"_stale", "Current stale background tasks by type").Set(int64(item.StaleCount))
		h.metricsReg.Gauge("ollanta_background_tasks_"+taskType+"_retrying", "Current retrying background tasks by type").Set(int64(item.RetryCount))
		if item.OldestQueuedAge != nil {
			h.metricsReg.Gauge("ollanta_background_tasks_"+taskType+"_oldest_queued_age_seconds", "Oldest queued background task age in seconds by type").Set(*item.OldestQueuedAge)
		}
	}
}

func stringIn(value string, list []string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
