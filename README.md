# Ollanta

Ollanta is a multi-language static analysis platform written in Go. It analyses source code, reports quality issues, computes code metrics, and evaluates configurable quality gates — making it easy to enforce coding standards in any CI/CD pipeline.

Inspired by [OpenStaticAnalyzer](https://github.com/sed-inf-u-szeged/OpenStaticAnalyzer), Ollanta is designed as a modular where each concern lives in its own Go module.

---

## Supported Languages

| Language   | Engine            | Rules |
|------------|-------------------|-------|
| Go         | Native (`go/ast`) | 8 rules: large functions, naked returns, naming, cognitive complexity, nesting depth, magic numbers, too many params, TODO comments |
| JavaScript | Tree-sitter       | 4 rules: large functions, console.log, strict equality, too many params |
| Python     | Tree-sitter       | 5 rules: large functions, broad except, mutable default args, comparison to None, too many params |

---

## Architecture

Ollanta follows a **hexagonal (ports & adapters)** layout. The inner modules have no external dependencies; adapters plug in at the edges.

```
domain/           — pure models, port interfaces, and domain services (zero external deps)
application/      — use cases: ingest, analysis, scan orchestration
adapter/
  config/         — environment-based configuration loader
  primary/http/   — chi-based HTTP handlers (primary adapter)
  secondary/
    postgres/     — pgx/v5 repository implementations
    search/       — Meilisearch indexer, searcher, and async worker
    oauth/        — GitHub / GitLab / Google OAuth providers, JWT, bcrypt
    webhook/      — outbound webhook dispatcher with retry
    breaker/      — circuit breaker for external calls
    telemetry/    — request tracing and Prometheus-style metrics
    parser/       — Tree-sitter adapter (CGo)
    rules/        — rule registry bridge to ollantarules
  cmd/web/        — main entry point: wires all adapters and starts the server

ollantacore/      — shared domain types (Component, Issue, Metric, Rule)
ollantaparser/    — language parsers (Go AST, Tree-sitter for JS/Py/TS/Rust)
ollantarules/     — rule definitions, language sensors, thresholds
ollantascanner/   — scan orchestration, file discovery, reporting (JSON + SARIF)
ollantaengine/    — post-scan analysis (quality gates, issue tracking, metric aggregation)
ollantastore/     — PostgreSQL repositories + Meilisearch search layer
ollantaweb/       — centralized REST API server (legacy monolith, kept for Docker builds)
```

---

## Prerequisites

- **Go 1.21+** with CGO enabled
- **GCC** (required by the Tree-sitter runtime)
  - Linux/macOS: `gcc` from your system package manager
  - Windows: [MSYS2](https://www.msys2.org/) → `pacman -S mingw-w64-x86_64-gcc`
- **Docker** (optional) — for container-based scanning or running the server stack

---

## Build

```sh
# Build all modules
make build

# Run all tests
make test

# Format source code
make fmt

# Run the linter (requires golangci-lint)
make lint
```

On Windows, the Makefile automatically prepends `C:\msys64\mingw64\bin` to PATH and sets `CGO_ENABLED=1`.

---

## Usage

### CLI scan

```sh
ollanta \
  -project-dir /path/to/myproject \
  -project-key  my-project \
  -format       all
```

### Interactive web UI

```sh
ollanta \
  -project-dir /path/to/myproject \
  -project-key  my-project \
  -format       all \
  -serve -port 7777
```

Opens a local web UI at `http://localhost:7777` with the scan results.

### Push results to a centralized server

```sh
ollanta \
  -project-dir /path/to/myproject \
  -project-key  my-project \
  -format       all \
  -server       http://localhost:8080
```

Posts the report to the ollantaweb API. Exits with code 1 if the quality gate fails.

### CLI flags

| Flag            | Default      | Description |
|-----------------|--------------|-------------|
| `-project-dir`  | `.`          | Root directory to scan |
| `-project-key`  | *(dir name)* | Identifier used in reports |
| `-sources`      | `./...`      | Comma-separated source patterns |
| `-exclusions`   | *(none)*     | Comma-separated glob patterns to exclude |
| `-format`       | `all`        | Output: `summary`, `json`, `sarif`, `all` |
| `-debug`        | `false`      | Enable verbose debug output |
| `-serve`        | `false`      | Open interactive web UI after scan |
| `-port`         | `7777`       | Port for `-serve` |
| `-bind`         | `127.0.0.1`  | Bind address for `-serve` (use `0.0.0.0` in Docker) |
| `-server`       | *(none)*     | URL of ollantaweb server to push results to |

### Output formats

| Format    | Description |
|-----------|-------------|
| `summary` | Human-readable table to stdout |
| `json`    | `.ollanta/report.json` |
| `sarif`   | `.ollanta/report.sarif` (GitHub Code Scanning compatible) |
| `all`     | Both `json` and `sarif` |

---

## Docker

### Scan with Docker

```sh
# Scan current directory and open UI at http://localhost:7777
docker compose up serve

# Scan a specific project
PROJECT_DIR=/path/to/myapp PROJECT_KEY=myapp docker compose up serve

# One-shot scan (no UI, just write report files)
docker compose run --rm scan-only
```

### Centralized server stack

Start PostgreSQL, Meilisearch, and the ollantaweb API server:

```sh
docker compose --profile server up -d
```

Then push scan results from any machine:

```sh
ollanta -project-dir . -project-key my-project -server http://your-server:8080
```

Or via Docker:

```sh
OLLANTA_SERVER=http://your-server:8080 docker compose --profile push run --rm push
```

### Environment variables

| Variable          | Default             | Description |
|-------------------|---------------------|-------------|
| `PROJECT_DIR`     | `.`                 | Host directory to scan |
| `PROJECT_KEY`     | `project`           | Project identifier |
| `PORT`            | `7777`              | Scanner UI port |
| `PG_PASSWORD`     | `ollanta_dev`       | PostgreSQL password |
| `MEILI_KEY`       | `ollanta_dev_key`   | Meilisearch master key |
| `OLLANTA_SERVER`  | `http://ollantaweb:8080` | API server URL (for push mode) |

---

## Server API (ollantaweb)

When running the server stack, the REST API is available at `:8080`. All `/api/v1` routes (except auth) require a `Bearer` token or an API token (`olt_…`) in the `Authorization` header.

**Auth**

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

**Core**

| Method | Endpoint                                  | Description |
|--------|-------------------------------------------|-------------|
| GET    | `/healthz`                                | Liveness probe |
| GET    | `/readyz`                                 | Readiness (PG + Meilisearch) |
| GET    | `/metrics`                                | Prometheus-style metrics |
| POST   | `/api/v1/projects`                        | Create/update a project |
| GET    | `/api/v1/projects`                        | List projects |
| GET    | `/api/v1/projects/{key}`                  | Get project by key |
| DELETE | `/api/v1/projects/{key}`                  | Delete project |
| POST   | `/api/v1/scans`                           | Ingest a scan report |
| GET    | `/api/v1/scans/{id}`                      | Get scan by ID |
| GET    | `/api/v1/projects/{key}/scans`            | List scans for project |
| GET    | `/api/v1/projects/{key}/scans/latest`     | Latest scan for project |
| GET    | `/api/v1/issues`                          | List/filter issues (project, severity, rule, status) |
| GET    | `/api/v1/issues/facets`                   | Issue distribution facets |
| GET    | `/api/v1/projects/{key}/measures/trend`   | Metric trend over time |
| GET    | `/api/v1/search`                          | Full-text search (Meilisearch) |
| POST   | `/admin/reindex`                          | Rebuild Meilisearch from PostgreSQL |

**Users, groups & permissions**

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

**API tokens**

| Method | Endpoint                    | Description |
|--------|-----------------------------|-------------|
| GET    | `/api/v1/tokens`            | List API tokens for current user |
| POST   | `/api/v1/tokens`            | Generate a new API token (`olt_…`) |
| DELETE | `/api/v1/tokens/{id}`       | Revoke token |

**Quality gates & profiles**

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

**New-code periods & webhooks**

| Method | Endpoint                                  | Description |
|--------|-------------------------------------------|-------------|
| GET    | `/api/v1/projects/{key}/new-code`         | Get new-code period setting |
| PUT    | `/api/v1/projects/{key}/new-code`         | Set new-code period |
| GET    | `/api/v1/projects/{key}/webhooks`         | List webhooks |
| POST   | `/api/v1/projects/{key}/webhooks`         | Register webhook |
| PUT    | `/api/v1/projects/{key}/webhooks/{id}`    | Update webhook |
| DELETE | `/api/v1/projects/{key}/webhooks/{id}`    | Delete webhook |
| GET    | `/api/v1/projects/{key}/webhooks/{id}/deliveries` | List recent deliveries |

---

## Example output (summary)

```
Project : my-project
Files   : 42    Lines : 3 218    NCLOC : 2 104    Comments : 311

ISSUES (7)
  CRITICAL  go:no-naked-returns       handlers/auth.go:87
  MAJOR     go:no-large-functions     handlers/auth.go:12
  MAJOR     go:no-large-functions     services/payment.go:34
  MINOR     go:naming-conventions     models/user_model.go:8
  ...

Quality Gate : ERROR
  ✗  bugs > 0  (actual: 1)
  ✓  coverage ≥ 80
```

---

## Rules reference

### Go (8 rules)

| Rule key                       | Severity | Description |
|--------------------------------|----------|-------------|
| `go:no-large-functions`        | Major    | Functions exceeding `max_lines` (default: 40) |
| `go:no-naked-returns`          | Critical | Naked `return` in functions with named returns longer than `min_lines` |
| `go:naming-conventions`        | Minor    | Exported names must use MixedCaps per Effective Go |
| `go:cognitive-complexity`      | Critical | Functions with high cognitive complexity |
| `go:too-many-parameters`       | Major    | Functions with too many parameters |
| `go:function-nesting-depth`    | Major    | Code nested too deeply |
| `go:magic-number`              | Minor    | Numeric literals that should be named constants |
| `go:todo-comment`              | Info     | TODO, FIXME, HACK and XXX comments |

### JavaScript (4 rules)

| Rule key                 | Severity | Description |
|--------------------------|----------|-------------|
| `js:no-large-functions`  | Major    | Functions exceeding `max_lines` (default: 40) |
| `js:no-console-log`      | Minor    | `console.log` left in production code |
| `js:eqeqeq`              | Major    | Use `===`/`!==` instead of `==`/`!=` |
| `js:too-many-parameters`  | Major    | Functions with too many parameters |

### Python (5 rules)

| Rule key                      | Severity | Description |
|-------------------------------|----------|-------------|
| `py:no-large-functions`       | Major    | Functions exceeding `max_lines` (default: 40) |
| `py:broad-except`             | Major    | Catching broad exceptions like `Exception` |
| `py:mutable-default-argument` | Major    | Mutable default argument values (list, dict, set) |
| `py:comparison-to-none`       | Minor    | Use `is`/`is not` for None comparisons |
| `py:too-many-parameters`       | Major    | Functions with too many parameters |

---

## Quality Gates

Quality gates evaluate numeric metrics against configurable thresholds after every scan. Conditions support operators: `gt`, `lt`, `eq`, `gte`, `lte`.

Gates can be managed via the API or defined in-process:

```go
conditions := []qualitygate.Condition{
    {MetricKey: "bugs",     Operator: qualitygate.OpGreaterThan, ErrorThreshold: 0,  Description: "Zero bugs"},
    {MetricKey: "coverage", Operator: qualitygate.OpLessThan,    ErrorThreshold: 80, Description: "Coverage ≥ 80%"},
}
status := qualitygate.Evaluate(conditions, measures)
if !status.Passed() {
    log.Fatal("Quality gate failed")
}
```

The server persists gates in PostgreSQL. Each scan stores its `gate_status` (`OK` / `WARN` / `ERROR`) and the scanner CLI exits with code 1 when the gate fails.

## Authentication

The server supports three authentication mechanisms:

| Mechanism | How to use |
|-----------|------------|
| **Local** | `POST /api/v1/auth/login` with `login` + `password`; returns a short-lived JWT and a refresh token |
| **OAuth** | GitHub, GitLab, or Google — configure via environment variables (see below); users are auto-provisioned on first login |
| **API tokens** | Generate via `POST /api/v1/tokens`; prefix `olt_`; use as `Authorization: Bearer olt_…` in CI scripts |

OAuth environment variables:

| Variable | Description |
|---|---|
| `OLLANTA_GITHUB_CLIENT_ID` / `_SECRET` | GitHub OAuth app credentials |
| `OLLANTA_GITLAB_CLIENT_ID` / `_SECRET` | GitLab OAuth app credentials |
| `OLLANTA_GOOGLE_CLIENT_ID` / `_SECRET` | Google OAuth app credentials |
| `OLLANTA_OAUTH_REDIRECT_BASE` | External base URL for OAuth callbacks (e.g. `https://ollanta.example.com`) |
| `OLLANTA_JWT_SECRET` | HMAC-SHA256 signing key (random if unset — tokens won't survive restarts) |
| `OLLANTA_JWT_EXPIRY` | Access token lifetime, e.g. `15m` (default) |

## Webhooks

Projects can register outbound webhooks that fire on scan events. Each webhook receives a signed JSON payload:

```json
{
  "event": "scan.completed",
  "project_key": "my-project",
  "scan_id": 42,
  "gate_status": "OK",
  "timestamp": "2026-04-17T12:00:00Z"
}
```

The `X-Ollanta-Signature` header contains an HMAC-SHA256 hex digest of the payload signed with the webhook secret. Deliveries are retried automatically and stored for audit via `GET /api/v1/projects/{key}/webhooks/{id}/deliveries`.

---

## Issue Tracking

The `ollantaengine/tracking` package compares current scan results against a previous baseline to classify each issue as **new**, **unchanged**, **closed**, or **reopened**. Issues are matched by rule key + line hash, with a fallback to file path + line number.

---

## Adding a new rule

1. Create a struct implementing the `ollantarules.Rule` interface in the appropriate language package.
2. Register it in the sensor's `ActiveRules()` method.
3. Add tests in the corresponding `*_test.go` file.

---

## License

MIT
