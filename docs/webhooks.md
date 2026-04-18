# Webhooks

Projects can register outbound webhooks that fire on scan events.

## Payload

Each webhook receives a signed JSON payload:

```json
{
  "event": "scan.completed",
  "project_key": "my-project",
  "scan_id": 42,
  "gate_status": "OK",
  "timestamp": "2026-04-17T12:00:00Z"
}
```

## Signature Verification

The `X-Ollanta-Signature` header contains an HMAC-SHA256 hex digest of the payload body, signed with the webhook secret configured at registration time.

## Retry & Audit

Deliveries are retried automatically on failure. The full delivery history (status, response code, response body) is available via the API for audit purposes.

## API Endpoints

| Method | Endpoint                                          | Description |
|--------|---------------------------------------------------|-------------|
| GET    | `/api/v1/projects/{key}/webhooks`                 | List webhooks |
| POST   | `/api/v1/projects/{key}/webhooks`                 | Register webhook |
| PUT    | `/api/v1/projects/{key}/webhooks/{id}`            | Update webhook |
| DELETE | `/api/v1/projects/{key}/webhooks/{id}`            | Delete webhook |
| GET    | `/api/v1/projects/{key}/webhooks/{id}/deliveries` | List recent deliveries |
