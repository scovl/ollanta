# Operations Runbooks

These runbooks assume PostgreSQL is the source of truth and search is a rebuildable projection. Prefer fixing the dependency first, then retrying durable work.

## Stale Durable Jobs

1. Identify the affected type with metrics such as `ollanta_background_tasks_scan_stale`, `ollanta_background_tasks_index_stale`, or `ollanta_background_tasks_webhook_stale`.
2. Check the role readiness endpoint: scan worker needs PostgreSQL; indexer needs PostgreSQL and search; webhook worker needs PostgreSQL.
3. Open the background task detail and inspect `worker_id`, `attempts`, `last_error`, `created_at`, `updated_at`, and `next_attempt_at`.
4. Wait one configured recovery interval if automatic recovery is active.
5. Use `requeue` for stale or retrying tasks after dependencies are healthy.
6. Use `retry` for failed tasks only after the root cause is fixed.

## Search Rebuild

1. Verify PostgreSQL health and schema readiness first.
2. Verify the configured search backend and credentials.
3. Trigger the admin reindex operation.
4. Watch `ollanta_background_tasks_index_queued`, `ollanta_background_tasks_index_running`, `ollanta_background_tasks_index_failed`, and `ollanta_background_tasks_index_oldest_queued_age_seconds` until the queue drains.
5. If index jobs fail, inspect their last error before retrying or re-running reindex.

## PostgreSQL Restore

1. Stop or scale down API and worker roles before restoring data.
2. Restore PostgreSQL from backup, including `schema_migrations` and durable job tables.
3. Run `/ollantamigrate` or the migration Job if the restored schema is behind the deployed image.
4. Start API and worker roles with `OLLANTA_AUTO_MIGRATE=false` so they verify schema compatibility.
5. Rebuild search from PostgreSQL if the search backend was not restored from a matching snapshot.
6. Check queue metrics for old accepted/running jobs; stale recovery should requeue or fail them according to max attempts.

## JWT Secret Rotation

1. Generate a strong new `OLLANTA_JWT_SECRET` and update the secret store.
2. Roll API pods together so token validation is consistent across replicas.
3. Expect existing access tokens and refresh tokens signed with the old secret to fail after rotation.
4. Ask users or automation to log in again or create fresh API tokens as needed.
5. Never enable `OLLANTA_ALLOW_RANDOM_JWT_SECRET` outside local development.

## Scanner Token Rotation

1. Generate a new high-entropy `OLLANTA_SCANNER_TOKEN` for the server.
2. Update scanner automation to send the same value as `OLLANTA_TOKEN`.
3. Roll the API pods.
4. Run a test scanner push and verify it returns `202 Accepted` or `200 OK` for an idempotent duplicate.
5. Remove the old token from CI secrets after all pipelines are updated.

## API Token Rotation

1. Create a replacement API token for the user or service account.
2. Update all consumers and run a read-only API call to verify the new token.
3. Revoke the old token through the API or UI.
4. Inspect recent auth failures if consumers continue using the revoked token.

## Worker Pool Tuning

The scan ingest pool size is controlled by `OLLANTA_WORKER_POOL` (default 4). Each goroutine consumes one database connection during active processing.

**Sizing guidelines:**

| Deployment scale | Recommended pool | Rationale |
|-----------------|-----------------|-----------|
| Dev / < 100 projects | 1-2 | Low throughput, saves connections |
| 100-1000 projects | 4 | Default, balanced |
| 1k-10k projects | 8 | Handles scan bursts |
| 10k-100k projects | 16 | High throughput, needs read replica |
| 100k+ projects | 16-32 | Multiple worker pods recommended |

```bash
# High-throughput deployment
OLLANTA_WORKER_POOL=16 ollantaworker

# Scale horizontally with multiple pods for 200k+ projects
OLLANTA_WORKER_POOL=8 ollantaworker  # pod 1
OLLANTA_WORKER_POOL=8 ollantaworker  # pod 2
```

Ensure `OLLANTA_POSTGRES_MAX_CONNS` is at least `worker_pool + api_pool + 10` to avoid connection starvation.

## Worker Heartbeat Monitoring

- Metrics: not yet exposed via Prometheus; monitor via `SELECT count(*) FROM scan_jobs WHERE status = 'running' AND worker_heartbeat < now() - interval '30 seconds'` for stale count
- Alert when stale count > 0 for more than 1 minute (indicates worker crash)
- Recovery: jobs are auto-reclaimed every 30 seconds. No manual action needed.

## Data Lifecycle Verification

- Check `SELECT count(*) FROM scan_jobs WHERE status = 'completed' AND completed_at < now() - interval '7 days'` — should be 0
- Check `SELECT count(*) FROM scans WHERE created_at < now() - interval '365 days'` — should be 0
- Disk usage: live_measures stays constant (~4M rows). measure_daily_aggregates grows ~1.4B/year. Plan storage accordingly.
- If cleanup stalls, restart the worker process. Cleanup runs every hour.
