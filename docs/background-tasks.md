# Background Tasks

Ollanta processes scan intake, search indexing, and webhook delivery asynchronously. The admin Background Tasks page and `/api/v1/admin/background-tasks` endpoints provide one operational view over those durable jobs.

## Task Types

- `scan`: scanner reports accepted by the server and processed into scans, issues, measures, and projections.
- `index`: search projection jobs created after scans or reindex operations.
- `webhook`: outbound delivery jobs for project and scan events.
- Future task types such as reindex, test-signal, and mutation-signal processing should use the same normalized API contract.

## Worker Pool

Scan ingest uses a **bounded goroutine pool** for parallel processing. Each goroutine independently claims and processes jobs from the `scan_jobs` table using `SELECT ... FOR UPDATE SKIP LOCKED`, guaranteeing that no two goroutines process the same job.

| Config | Default | Description |
|--------|---------|-------------|
| `OLLANTA_WORKER_POOL` | `4` | Number of concurrent scan ingest goroutines |

```bash
# Single worker
OLLANTA_WORKER_POOL=1 ollantaworker

# 16 goroutines for high-throughput deployments (200k+ projects)
OLLANTA_WORKER_POOL=16 ollantaworker
```

Workers run in the same process sharing a single `pgxpool`. Each worker loop: `ClaimNext()` → `ProcessNext()` → repeat (100ms idle delay).

## Batch Optimizations

To handle thousands of scans per second with minimal database round-trips:

| Operation | Strategy | Round-trips/scan |
|-----------|----------|-----------------|
| Issue bulk insert | PostgreSQL `COPY` protocol | 1 |
| Measure bulk insert | PostgreSQL `COPY` protocol | 1 |
| Live measures UPSERT | Multi-row `unnest()` batch | 1 |
| Daily rollup UPSERT | Multi-row `unnest()` batch | 1 |
| Search indexing (ZincSearch) | Bulk API | 1 |
| Search indexing (PG FTS) | No-op (live queries) | 0 |

**Total: ~4 round-trips per scan** regardless of issue/measure count. Without batching, a scan with 20 metrics would need ~62 round-trips.

## Worker Heartbeat & Recovery

Each worker pings `worker_heartbeat = now()` every 10 seconds. If a worker crashes or stalls, its heartbeat stops updating. A recovery goroutine runs every 30 seconds:

```sql
UPDATE scan_jobs
SET status = 'accepted', worker_id = NULL, worker_heartbeat = NULL
WHERE status = 'running'
  AND worker_heartbeat IS NOT NULL
  AND worker_heartbeat < now() - interval '30 seconds'
```

Stale jobs are returned to the `accepted` pool and picked up by another goroutine. This ensures no job is permanently lost on worker failure.

## Data Lifecycle Cleanup

A cleanup goroutine runs every hour removing expired data:

| Table | TTL | Action |
|-------|-----|--------|
| `scan_jobs` | 7 days | DELETE completed jobs |
| `scans` | 365 days | DELETE (cascades to issues, measures) |
| `measures` | 90 days | DELETE per-scan measures |

The `live_measures` table is not cleaned — it stores only current values (UPSERT, ~4M rows constant). The `measure_daily_aggregates` table provides historical trends without the storage overhead of per-scan measures.

## States

- `queued`: the source job is accepted and ready to be claimed.
- `running`: a worker has claimed the task.
- `retrying`: the task is accepted but its next attempt time is in the future.
- `stale`: the task is still persisted as running, but has exceeded the type-specific stale threshold.
- `failed`: the worker exhausted the task or recorded a durable failure.
- `completed`: the task finished successfully.
- `cancelled`: an administrator cancelled a queued task before a worker claimed it.

Stale state is derived for visibility in the admin API. Worker roles also run automatic stale recovery loops: stale jobs below the configured max-attempt count are requeued, while jobs at or above the max-attempt count are failed with a recovery error. Manual admin actions remain useful when an operator wants to intervene immediately.

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

Automatic recovery is controlled per role:

| Job type | Stale threshold | Max attempts | Recovery interval |
|----------|-----------------|--------------|-------------------|
| scan | `OLLANTA_SCAN_JOB_STALE_AFTER` | `OLLANTA_SCAN_JOB_MAX_ATTEMPTS` | `OLLANTA_SCAN_JOB_RECOVERY_INTERVAL` |
| index | `OLLANTA_INDEX_JOB_STALE_AFTER` | `OLLANTA_INDEX_JOB_MAX_ATTEMPTS` | `OLLANTA_INDEX_JOB_RECOVERY_INTERVAL` |
| webhook | `OLLANTA_WEBHOOK_JOB_STALE_AFTER` | `OLLANTA_WEBHOOK_JOB_MAX_ATTEMPTS` | `OLLANTA_WEBHOOK_JOB_RECOVERY_INTERVAL` |

Set a max-attempt value high enough for transient database/search/webhook outages, but low enough that permanently broken jobs become visible as failed.

## Retention

Completed, failed, and cancelled job rows are retained in PostgreSQL until regular database maintenance removes or archives them. Large installations should define retention jobs based on operational needs:

- completed tasks can usually be retained for a shorter audit window.
- failed and cancelled tasks should be retained long enough for incident review.
- stale tasks should not be deleted until their worker condition is understood.

## Metrics And Troubleshooting

Prometheus-compatible gauges are refreshed on a timer, independently of the summary endpoint, under names such as `ollanta_background_tasks_scan_queued`, `ollanta_background_tasks_index_failed`, and `ollanta_background_tasks_webhook_stale`. Recovery outcome counters are emitted as `ollanta_scan_jobs_recovered_total`, `ollanta_index_jobs_failed_by_recovery_total`, and equivalent webhook counters.

When project processing is delayed:

1. Check `scan` queued/running/stale tasks for ingestion pressure.
2. Check `index` queued/retrying/stale tasks if code search or issue projections lag behind scans.
3. Check `webhook` failed/retrying tasks when integrations do not receive events.
4. Inspect worker ids and last errors in task details.
5. Retry failed tasks only after the underlying error is fixed.

## Runbooks

### Stale Jobs

1. Check the role readiness endpoint (`/readyz`) for the affected job type.
2. Inspect the job detail for `worker_id`, `attempts`, `last_error`, and timestamps.
3. If recovery counters are increasing, wait one recovery interval before manual action.
4. If the dependency is healthy and the job is still stale, use `requeue` for stale/retrying jobs.
5. If the job repeatedly fails after requeueing, treat the payload or external dependency as the incident source.

### Search Rebuild

1. Confirm PostgreSQL is healthy; it is the source of truth.
2. Confirm the search backend readiness and credentials.
3. Trigger the admin reindex operation.
4. Watch `index` queued/running/stale/failed metrics until the queue drains.

### Webhook Delivery Backlog

1. Check destination availability and network policy.
2. Inspect failed webhook task details and response codes.
3. Retry only after the downstream system is accepting requests.
