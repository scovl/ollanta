# Observability

Ollanta can run a local observability stack with Prometheus, Jaeger, Loki, and Promtail using a dedicated Docker Compose profile.

## Start The Stack

For the centralized backend plus local observability:

```bash
docker compose --profile server --profile observability up -d
```

To include the local scanner UI as well:

```bash
docker compose --profile server --profile scanner --profile observability up -d local-ui
```

## Endpoints

- Prometheus UI: `http://localhost:9091`
- Jaeger UI: `http://localhost:16686`
- Loki API: `http://localhost:3100`
- Ollanta API: `http://localhost:8080`
- Scanner UI local: `http://localhost:7777`

## Optional UI Links

The Ollanta web UI always shows Ollanta-owned observability endpoints such as `/metrics`. Links to external tools are optional because deployments may use Prometheus, Jaeger, Loki, Datadog, Grafana Cloud, Elastic, Honeycomb, or another stack.

Configure links in `config.toml`:

```toml
[[ui.observability_links]]
label = "Datadog"
url = "https://app.datadoghq.com/dashboard/example"

[[ui.observability_links]]
label = "Grafana"
url = "https://grafana.example.com/d/ollanta"
```

Or with the `OLLANTA_OBSERVABILITY_LINKS` environment variable. Entries are separated by semicolons and each entry uses `Label=https://absolute-url`:

```bash
OLLANTA_OBSERVABILITY_LINKS="Prometheus=http://localhost:9091/targets;Jaeger=http://localhost:16686;Loki=http://localhost:3100/ready"
```

If no links are configured, no external observability tools are shown in the navigation.

## What Gets Collected

- Prometheus scrapes:
  - `ollantaweb:8080/metrics`
  - `ollantaworker:9090/metrics`
  - `ollantaindexer:9090/metrics`
  - `ollantawebhookworker:9090/metrics`
  - `local-ui:7777/metrics`
- Jaeger receives HTTP traces and spans from asynchronous loops over OTLP HTTP at `http://jaeger:4318`
- Promtail reads stdout logs from the `ollanta` project containers and sends them to Loki

## Health And Readiness

Every long-running role exposes `healthz`, `readyz`, and `metrics`.

| Role | Endpoint | Readiness semantics |
|------|----------|---------------------|
| API (`ollantaweb`) | `:8080/readyz` | Fails when PostgreSQL is unavailable. Returns `200` with `status: degraded` when only search is unavailable, because PostgreSQL remains the source of truth. |
| Scan worker (`ollantaworker`) | `OLLANTA_ADMIN_ADDR/readyz` | Fails when PostgreSQL is unavailable. |
| Indexer (`ollantaindexer`) | `OLLANTA_ADMIN_ADDR/readyz` | Fails when PostgreSQL or the configured search backend is unavailable. |
| Webhook worker (`ollantawebhookworker`) | `OLLANTA_ADMIN_ADDR/readyz` | Fails when PostgreSQL is unavailable. |

`/healthz` is a liveness signal. Use `/readyz` for traffic routing and worker availability.

## Key Metrics

Durable job metrics are refreshed by the server process on a timer and do not depend on anyone calling the admin summary endpoint.

| Metric family | Meaning |
|---------------|---------|
| `ollanta_background_tasks_<type>_queued` | Current queued durable jobs by type: `scan`, `index`, `webhook` |
| `ollanta_background_tasks_<type>_running` | Current running jobs by type |
| `ollanta_background_tasks_<type>_stale` | Running jobs past the derived stale threshold |
| `ollanta_background_tasks_<type>_retrying` | Accepted jobs delayed until a future retry time |
| `ollanta_background_tasks_<type>_failed` | Failed jobs by type |
| `ollanta_background_tasks_<type>_oldest_queued_age_seconds` | Oldest queued job age by type |
| `ollanta_<type>_jobs_recovered_total` | Stale jobs requeued by automatic recovery |
| `ollanta_<type>_jobs_failed_by_recovery_total` | Stale jobs failed after exhausting attempts |
| `ollanta_scan_jobs_processed_total` | Scan jobs processed successfully |
| `ollanta_scan_jobs_failed_total` | Scan jobs that failed during processing |
| `ollanta_ingest_duration_seconds` | Scan ingest processing duration histogram |
| `ollanta_db_pool_acquired_conns` | PostgreSQL pool connections currently acquired |
| `ollanta_db_pool_idle_conns` | PostgreSQL pool connections currently idle |
| `ollanta_db_pool_total_conns` | PostgreSQL pool connections currently open |
| `ollanta_db_health` | `1` when PostgreSQL health check succeeds, `0` otherwise |

Metric names intentionally avoid project keys, job IDs, URLs, and other high-cardinality labels.

## Alert Suggestions

- API readiness failing: page immediately; PostgreSQL is unavailable or schema verification failed.
- API degraded on search for more than 10 minutes: investigate ZincSearch or run the search rebuild workflow after recovery.
- `ollanta_background_tasks_scan_oldest_queued_age_seconds` above the expected ingest SLO: add scan workers, inspect worker errors, or raise database capacity.
- Any stale job gauge above `0` for more than one recovery interval: check worker crashes, stuck dependencies, or max-attempt failures.
- Failed-by-recovery counters increasing: inspect durable job details before retrying manually.
- PostgreSQL acquired connections near `OLLANTA_POSTGRES_MAX_CONNS` for several minutes: increase pool or database capacity, or reduce replica concurrency.

See `docs/operations-runbooks.md` for stale job, search rebuild, PostgreSQL restore, and token rotation procedures.

## Relevant Variables

- `OLLANTA_LOG_LEVEL`: controls the minimum structured log level
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP HTTP endpoint for traces
- `OLLANTA_ADMIN_ADDR`: worker admin address for `healthz`, `readyz`, and `metrics`

The Compose services are already preconfigured with `OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318`. When the `observability` profile is not active and the endpoint is unavailable, the tracing bootstrap falls back to a no-op implementation.

## Local Retention

- Prometheus stores its TSDB in the named volume `prometheusdata`; retention follows Prometheus defaults for as long as the volume exists.
- Loki stores chunks and indexes in the named volume `lokidata`; because no retention policy is configured, logs remain until the volume is removed.
- Jaeger all-in-one is configured without persistent storage; traces are ephemeral and are lost when the container is recreated.
- To fully clear local observability data, run `docker compose down -v`.

## Quick Validation

1. Open `http://localhost:9091` and confirm that the `ollantaweb`, `ollantaworker`, `ollantaindexer`, and `ollantawebhookworker` targets are `UP`.
2. Send a request to the server, for example `GET /healthz`, and confirm in Prometheus that `ollanta_http_requests_total` increased.
3. Open `http://localhost:16686`, choose one of `ollantaweb`, `ollantascanner`, `ollantaworker`, `ollantaindexer`, or `ollantawebhookworker`, and confirm that traces are present.
4. Query Loki through the API, for example:

```bash
curl "http://localhost:3100/loki/api/v1/query?query={service=\"ollantaweb\"}"
```

5. Look for `trace_id` and `span_id` in the JSON log entries emitted by the instrumented services.