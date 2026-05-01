# Server API Reference

All `/api/v1` routes (except auth) require authentication.

- Most routes expect a `Bearer` token or an API token (`olt_…`) in the `Authorization` header
- `POST /api/v1/scans` and `GET /api/v1/scan-jobs/{id}` also accept the shared scanner token configured through `OLLANTA_SCANNER_TOKEN`

This document covers the centralized `ollantaweb` API. The scanner-local UI on port `7777` has its own embedded endpoints for browser use, such as rule lookup and local `Fix with AI` preview/apply actions.

The server listens on `:8080` by default. Start it with:

```sh
docker compose --profile server up -d
```

`ollantaweb` also reads a local `config.toml` file when present. To point at a different path, start the binary with `OLLANTA_CONFIG_FILE=/path/to/config.toml`.
The backend reads its own settings from `[server]`, PostgreSQL connectivity from `[database]`, and search configuration from `[search]`.
Database and search settings can be expressed either as a full `url` or as explicit host/port/credential fields in `config.toml`.

---

## Auth

| Method | Endpoint                        | Description |
|--------|---------------------------------|-------------|
| POST   | `/api/v1/auth/login`            | Email+password login → JWT + refresh token |
| POST   | `/api/v1/auth/refresh`          | Refresh access token |
| POST   | `/api/v1/auth/logout`           | Invalidate refresh token |
| GET    | `/api/v1/auth/github`           | Start GitHub OAuth flow |
| GET    | `/api/v1/auth/github/callback`  | GitHub OAuth callback |
| GET    | `/api/v1/auth/gitlab`           | Start GitLab OAuth flow |
| GET    | `/api/v1/auth/gitlab/callback`  | GitLab OAuth callback |
| GET    | `/api/v1/auth/google`           | Start Google OAuth flow |
| GET    | `/api/v1/auth/google/callback`  | Google OAuth callback |

## Core

| Method | Endpoint                                  | Description |
|--------|-------------------------------------------|-------------|
| GET    | `/healthz`                                | Liveness probe |
| GET    | `/readyz`                                 | Readiness (PG + search backend) |
| GET    | `/metrics`                                | Prometheus-style metrics |
| POST   | `/api/v1/projects`                        | Create/update a project |
| GET    | `/api/v1/projects`                        | List projects |
| GET    | `/api/v1/projects/{key}`                  | Get project by key |
| PUT    | `/api/v1/projects/{key}`                  | Update project metadata, including `main_branch` |
| DELETE | `/api/v1/projects/{key}`                  | Delete project |
| POST   | `/api/v1/scans`                           | Accept a scan report for asynchronous processing |
| GET    | `/api/v1/scan-jobs/{id}`                  | Get durable scan-job status |
| GET    | `/api/v1/scans/{id}`                      | Get scan by ID |
| GET    | `/api/v1/projects/{key}/scans`            | List scans for project |
| GET    | `/api/v1/projects/{key}/scans/latest`     | Latest scan for project |
| GET    | `/api/v1/projects/{key}/overview`         | Scope-aware dashboard payload |
| GET    | `/api/v1/projects/{key}/activity`         | Scope-aware activity timeline |
| GET    | `/api/v1/projects/{key}/branches`         | Known branches and latest scan metadata |
| GET    | `/api/v1/projects/{key}/pull-requests`    | Known pull requests and latest scan metadata |
| GET    | `/api/v1/projects/{key}/information`      | Project metadata plus active-scope metadata |
| GET    | `/api/v1/projects/{key}/code/tree`        | Latest code snapshot manifest for the selected scope |
| GET    | `/api/v1/projects/{key}/code/file`        | Snapshot file content and matching issues for the selected scope |
| GET    | `/api/v1/issues`                          | List/filter issues (project, severity, rule, status) |
| GET    | `/api/v1/issues/facets`                   | Issue distribution facets |
| GET    | `/api/v1/projects/{key}/measures/trend`   | Metric trend over time |
| GET    | `/api/v1/search`                          | Full-text search (ZincSearch / Postgres FTS) |
| GET    | `/api/v1/admin/index-jobs`                | List durable search-index jobs |
| POST   | `/api/v1/admin/index-jobs/{id}/retry`     | Retry a failed search-index job |
| GET    | `/api/v1/admin/webhook-jobs`              | List durable webhook delivery jobs |
| POST   | `/api/v1/admin/webhook-jobs/{id}/retry`   | Retry a failed webhook job |
| POST   | `/admin/reindex`                          | Rebuild search indexes from PostgreSQL |

### Scanner ingestion authentication

The scanner push workflow can authenticate without a user account by sharing a pre-configured scanner secret:

```sh
export OLLANTA_SCANNER_TOKEN=ollanta-dev-scanner-token
export OLLANTA_TOKEN=ollanta-dev-scanner-token
docker compose --profile server up -d
docker compose --profile push run --build --rm push
```

If `OLLANTA_SCANNER_TOKEN` is empty on the server, ingestion falls back to regular token-based authentication.
The same value can also be provided through `[server].scanner_token` in `config.toml`.

### Scope-aware project endpoints

The following routes accept optional mutually exclusive `branch` and `pull_request` query parameters when their state depends on analysis scope:

- `GET /api/v1/projects/{key}/scans`
- `GET /api/v1/projects/{key}/scans/latest`
- `GET /api/v1/projects/{key}/overview`
- `GET /api/v1/projects/{key}/activity`
- `GET /api/v1/projects/{key}/information`
- `GET /api/v1/projects/{key}/code/tree`
- `GET /api/v1/projects/{key}/code/file?path=...`
- `GET /api/v1/issues` and `GET /api/v1/issues/facets` when `project_key` is provided

Rules:

- `branch` and `pull_request` cannot be combined in the same request.
- If neither is provided, branch mode falls back to the resolved default branch for the project.
- Default-branch resolution uses `projects.main_branch` first, then observed `main`, then observed `master`, then the most recent non-empty branch, and finally legacy blank-branch scans.
- Historic scans that never recorded a branch remain visible through that resolved default branch for backward compatibility.

### Scanner SCM metadata

Scan reports now carry scope metadata in `report.json` and during ingestion:

- `scope_type`: `branch` or `pull_request`
- `branch`
- `commit_sha`
- `pull_request_key`
- `pull_request_base`

The scanner resolves these fields from explicit CLI flags, Git metadata, and supported CI variables. Detached HEAD runs must provide `-branch` explicitly or the scanner fails fast.

### Code snapshots

Every successful ingest stores the latest successful code snapshot for the selected branch or pull request scope.

- `GET /api/v1/projects/{key}/code/tree` returns snapshot metadata and the file manifest without file contents.
- `GET /api/v1/projects/{key}/code/file?path=...` returns the selected file content together with issues from the latest scan in the same scope.
- Snapshot storage is bounded to `128 KB` per file and `4 MB` total per scope. Truncated or omitted files expose flags and omission reasons in the response.

### Asynchronous scan intake

`POST /api/v1/scans` now returns `202 Accepted` after the report is durably stored as a `scan_job`.

Typical flow:

1. Submit `report.json` to `POST /api/v1/scans`.
2. Read the returned `id` and `status` fields from the accepted scan job.
3. Poll `GET /api/v1/scan-jobs/{id}` until the job becomes `completed` or `failed`.
4. Once completed, use `scan_id` to query `/api/v1/scans/{id}` or the project scan-history endpoints.

The scanner CLI can perform this polling automatically with:

```sh
ollanta -project-dir . -server http://localhost:8080 -server-token ollanta-dev-scanner-token -server-wait
```

Useful flags:

- `-server-wait`: wait for the accepted scan job to finish
- `-server-wait-timeout=10m`: fail if the job does not finish in time
- `-server-wait-poll=2s`: polling interval while waiting

### Durable side-effect inspection

Search indexing and webhook deliveries now run through durable PostgreSQL-backed job tables and dedicated worker processes. Administrators can inspect and retry failed outbox work through the `/api/v1/admin/index-jobs` and `/api/v1/admin/webhook-jobs` endpoints.

## Users, Groups & Permissions

| Method | Endpoint                                  | Description |
|--------|-------------------------------------------|-------------|
| GET    | `/api/v1/users`                           | List users |
| GET    | `/api/v1/users/me`                        | Current user profile |
| POST   | `/api/v1/users`                           | Create user |
| PUT    | `/api/v1/users/{id}`                      | Update user |
| POST   | `/api/v1/users/{id}/change-password`      | Change password |
| DELETE | `/api/v1/users/{id}`                      | Deactivate user |
| GET    | `/api/v1/groups`                          | List groups |
| POST   | `/api/v1/groups`                          | Create group |
| POST   | `/api/v1/groups/{id}/members`             | Add member to group |
| DELETE | `/api/v1/groups/{id}/members/{userID}`    | Remove member |
| DELETE | `/api/v1/groups/{id}`                     | Delete group |
| GET    | `/api/v1/projects/{key}/permissions`      | List project permissions |
| POST   | `/api/v1/projects/{key}/permissions`      | Grant permission |
| DELETE | `/api/v1/projects/{key}/permissions/{id}` | Revoke permission |

## API Tokens

| Method | Endpoint                    | Description |
|--------|-----------------------------|-------------|
| GET    | `/api/v1/tokens`            | List API tokens for current user |
| POST   | `/api/v1/tokens`            | Generate a new API token (`olt_…`) |
| DELETE | `/api/v1/tokens/{id}`       | Revoke token |

## Quality Gates & Profiles

| Method | Endpoint                                           | Description |
|--------|----------------------------------------------------|-------------|
| GET    | `/api/v1/gates`                                    | List quality gates |
| POST   | `/api/v1/gates`                                    | Create quality gate |
| GET    | `/api/v1/gates/{id}`                               | Get gate details |
| PUT    | `/api/v1/gates/{id}`                               | Update gate |
| DELETE | `/api/v1/gates/{id}`                               | Delete gate |
| POST   | `/api/v1/gates/{id}/conditions`                    | Add condition to gate |
| DELETE | `/api/v1/gates/{id}/conditions/{condID}`           | Remove condition |
| GET    | `/api/v1/profiles`                                 | List quality profiles |
| POST   | `/api/v1/profiles`                                 | Create quality profile |
| GET    | `/api/v1/profiles/{id}`                            | Get profile |
| POST   | `/api/v1/profiles/{id}/rules`                      | Activate rule in profile |
| DELETE | `/api/v1/profiles/{id}/rules/{ruleKey}`            | Deactivate rule |
| POST   | `/api/v1/profiles/{id}/import`                     | Import profile from YAML |

## New-code Periods & Webhooks

| Method | Endpoint                                  | Description |
|--------|-------------------------------------------|-------------|
| GET    | `/api/v1/projects/{key}/new-code`         | Get new-code period setting |
| PUT    | `/api/v1/projects/{key}/new-code`         | Set new-code period |
| GET    | `/api/v1/projects/{key}/webhooks`         | List webhooks |
| POST   | `/api/v1/projects/{key}/webhooks`         | Register webhook |
| PUT    | `/api/v1/projects/{key}/webhooks/{id}`    | Update webhook |
| DELETE | `/api/v1/projects/{key}/webhooks/{id}`    | Delete webhook |
| GET    | `/api/v1/projects/{key}/webhooks/{id}/deliveries` | List recent deliveries |
