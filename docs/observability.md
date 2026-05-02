# Observability

Ollanta can run a local observability stack with Prometheus, Jaeger, Loki, and Promtail using a dedicated Docker Compose profile.

## Start The Stack

For the centralized backend plus local observability:

```bash
docker compose --profile server --profile observability up -d
```

To include the local scanner UI as well:

```bash
docker compose --profile server --profile observability up -d serve
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
  - `serve:7777/metrics`
- Jaeger receives HTTP traces and spans from asynchronous loops over OTLP HTTP at `http://jaeger:4318`
- Promtail reads stdout logs from the `ollanta` project containers and sends them to Loki

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