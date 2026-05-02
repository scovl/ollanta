# Background Tasks

Ollanta processes scan intake, search indexing, and webhook delivery asynchronously. The admin Background Tasks page and `/api/v1/admin/background-tasks` endpoints provide one operational view over those durable jobs.

## Task Types

- `scan`: scanner reports accepted by the server and processed into scans, issues, measures, and projections.
- `index`: search projection jobs created after scans or reindex operations.
- `webhook`: outbound delivery jobs for project and scan events.
- Future task types such as reindex, test-signal, and mutation-signal processing should use the same normalized API contract.

## States

- `queued`: the source job is accepted and ready to be claimed.
- `running`: a worker has claimed the task.
- `retrying`: the task is accepted but its next attempt time is in the future.
- `stale`: the task is still persisted as running, but has exceeded the type-specific stale threshold.
- `failed`: the worker exhausted the task or recorded a durable failure.
- `completed`: the task finished successfully.
- `cancelled`: an administrator cancelled a queued task before a worker claimed it.

Stale state is derived for visibility. It does not mutate the persisted job status unless an admin explicitly requeues or retries the task.

## Admin Actions

- `retry`: available for failed or cancelled tasks. It resets the task to queued.
- `requeue`: available for stale or retrying tasks. It clears worker and timing fields so the task can be claimed again.
- `cancel`: available for queued tasks. It marks the task cancelled so workers no longer claim it.

Actions are state-aware. Unsupported actions return a JSON error and do not mutate the job.

## API

All endpoints require the global `admin` permission.

- `GET /api/v1/admin/background-tasks`: list normalized tasks. Supports `type`, `status`, `project_key`, `scan_id`, `worker_id`, `failed_only`, `stale_only`, `created_after`, `created_before`, `limit`, and `offset`.
- `GET /api/v1/admin/background-tasks/summary`: queue health counts and lag indicators using the same filters.
- `GET /api/v1/admin/background-tasks/{task_id}`: inspect a single task such as `scan:12` or `index:42`.
- `POST /api/v1/admin/background-tasks/{task_id}/retry`: retry a failed or cancelled task.
- `POST /api/v1/admin/background-tasks/{task_id}/requeue`: requeue a stale or retrying task.
- `POST /api/v1/admin/background-tasks/{task_id}/cancel`: cancel a queued task.

Specialized index and webhook job endpoints remain available for compatibility, but the normalized endpoint is the canonical admin surface.

## Stale Thresholds

Default derived stale thresholds are:

- scan: 30 minutes
- index: 10 minutes
- webhook: 10 minutes

Long-running tasks that exceed these thresholds should be inspected before requeueing. A stale task often means a worker crashed, lost database connectivity, or is blocked on an external dependency.

## Retention

Completed, failed, and cancelled job rows are retained in PostgreSQL until regular database maintenance removes or archives them. Large installations should define retention jobs based on operational needs:

- completed tasks can usually be retained for a shorter audit window.
- failed and cancelled tasks should be retained long enough for incident review.
- stale tasks should not be deleted until their worker condition is understood.

## Metrics And Troubleshooting

The summary endpoint updates Prometheus-compatible gauges under names such as `ollanta_background_tasks_scan_queued`, `ollanta_background_tasks_index_failed`, and `ollanta_background_tasks_webhook_stale`.

When project processing is delayed:

1. Check `scan` queued/running/stale tasks for ingestion pressure.
2. Check `index` queued/retrying/stale tasks if code search or issue projections lag behind scans.
3. Check `webhook` failed/retrying tasks when integrations do not receive events.
4. Inspect worker ids and last errors in task details.
5. Retry failed tasks only after the underlying error is fixed.
