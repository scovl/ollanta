# Server API Reference

All `/api/v1` routes (except auth) require authentication.

- Most routes expect a `Bearer` token or an API token (`olt_…`) in the `Authorization` header
- `POST /api/v1/scans` and `GET /api/v1/scan-jobs/{id}` also accept the shared scanner token configured through `OLLANTA_SCANNER_TOKEN`

This document covers the centralized `ollantaweb` API. The scanner-local UI on port `7777` has its own embedded endpoints for browser use, such as rule lookup and local `Fix with AI` preview/apply actions.

The server listens on `:8080` by default. Start it with:

```sh
docker compose --profile server up -d
```

The local compose stack has development defaults for PostgreSQL, ZincSearch, the seeded admin login, and the scanner token. Override them in `.env` for shared environments.

`ollantaweb` also reads a local `config.toml` file when present. To point at a different path, start the binary with `OLLANTA_CONFIG_FILE=/path/to/config.toml`.
The backend reads its own settings from `[server]`, PostgreSQL connectivity from `[database]`, and search configuration from `[search]`.
Database and search settings can be expressed either as a full `url` or as explicit host/port/credential fields in `config.toml`. Environment-based deployments can use `OLLANTA_DATABASE_URL` or the discrete `OLLANTA_POSTGRES_HOST`, `OLLANTA_POSTGRES_PORT`, `OLLANTA_POSTGRES_DB`, `OLLANTA_POSTGRES_USER`, `OLLANTA_POSTGRES_PASSWORD`, and `OLLANTA_POSTGRES_SSLMODE` variables.

---

## Auth

| Method | Endpoint                        | Description |
|--------|---------------------------------|-------------|
| POST   | `/api/v1/auth/login`            | Local login with `{ "login": "...", "password": "..." }` → JWT + refresh token |
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
| GET    | `/api/v1/scans/{id}`                      | Get scan by ID (includes `test_signals` when available) |
| GET    | `/api/v1/scans/{id}/survived-mutants`    | Survived mutant list extracted from test signals |
| GET    | `/api/v1/projects/{key}/scans`            | List scans for project |
| GET    | `/api/v1/projects/{key}/scans/latest`     | Latest scan for project |
| GET    | `/api/v1/projects/{key}/overview`         | Scope-aware dashboard payload |
| GET    | `/api/v1/projects/{key}/activity`         | Scope-aware activity timeline |
| GET    | `/api/v1/projects/{key}/branches`         | Known branches and latest scan metadata |
| GET    | `/api/v1/projects/{key}/pull-requests`    | Known pull requests and latest scan metadata |
| GET    | `/api/v1/projects/{key}/information`      | Project metadata plus active-scope metadata |
| GET    | `/api/v1/projects/{key}/code/tree`        | Latest code snapshot manifest for the selected scope |
| GET    | `/api/v1/projects/{key}/code/file`        | Snapshot file content and matching issues for the selected scope |
| GET    | `/api/v1/issues`                          | List/filter issues by scope, quality, severity, lifecycle, language, rule, tag, directory, or file |
| GET    | `/api/v1/issues/facets`                   | Issue distribution facets for the active filtered result set |
| POST   | `/api/v1/projects/{key}/issues/backfill-tracking-state` | Backfill legacy `tracking_state` values for the project's issue history |
| GET    | `/api/v1/projects/{key}/measures/trend`   | Metric trend over time |
| GET    | `/api/v1/search`                          | Full-text search (ZincSearch / Postgres FTS) |
| GET    | `/api/v1/tags`                            | List governed and discovered tags with usage metadata |
| POST   | `/api/v1/tags`                            | Create a governed tag catalog entry |
| GET    | `/api/v1/tags/{key}`                      | Read a tag page with metadata, usage, aliases, and audit summary |
| PUT    | `/api/v1/tags/{key}`                      | Update tag metadata and ownership |
| GET    | `/api/v1/tags/{key}/audit`                | List audit entries for a tag |
| POST   | `/api/v1/tags/{key}/deprecate`            | Deprecate a tag, optionally with a replacement key |
| POST   | `/api/v1/tags/{key}/merge`                | Merge one tag into another and keep the old key as an alias |
| POST   | `/api/v1/tags/bulk/preview`               | Preview a bulk tag edit across issues, projects, rules, or custom rules |
| POST   | `/api/v1/tags/bulk/apply`                 | Apply a validated bulk tag edit and write audit entries |
| GET    | `/api/v1/saved-filters`                   | List saved filters by visibility and filter type |
| POST   | `/api/v1/saved-filters`                   | Create a private or shared saved filter |
| GET    | `/api/v1/saved-filters/{id}`              | Get a saved filter |
| PUT    | `/api/v1/saved-filters/{id}`              | Update a saved filter |
| DELETE | `/api/v1/saved-filters/{id}`              | Delete a saved filter |
| POST   | `/api/v1/saved-filters/{id}/apply`        | Resolve a saved filter into criteria, including tag aliases |
| GET    | `/api/v1/admin/index-jobs`                | List durable search-index jobs |
| POST   | `/api/v1/admin/index-jobs/{id}/retry`     | Retry a failed search-index job |
| GET    | `/api/v1/admin/webhook-jobs`              | List durable webhook delivery jobs |
| POST   | `/api/v1/admin/webhook-jobs/{id}/retry`   | Retry a failed webhook job |
| POST   | `/admin/reindex`                          | Rebuild search indexes from PostgreSQL |

### Scanner ingestion authentication

The scanner push workflow can authenticate without a user account by sharing a pre-configured scanner secret:

```sh
docker compose --profile server up -d
docker compose --profile push run --build --rm push
```

The compose defaults set both scanner-token sides to `ollanta-dev-scanner-token`. If `OLLANTA_SCANNER_TOKEN` is empty on the server, ingestion falls back to regular token-based authentication.
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

### Issue filtering and facets

`GET /api/v1/issues` and `GET /api/v1/issues/facets` share the same filter query parameters. Facets are calculated from the full filtered result set before list pagination is applied.

Common filters:

| Query parameter | Meaning |
|-----------------|---------|
| `project_id` / `project_key` | Project selector. `project_key` is required for branch or pull request scoped queries. |
| `scan_id` | Exact scan selector. Cannot be combined with `branch` or `pull_request`. |
| `branch` / `pull_request` | Resolve the latest scan for a branch or pull request scope. |
| `quality` | Software quality domain: `security`, `reliability`, `maintainability`, `testability`. |
| `severity` | `blocker`, `critical`, `major`, `minor`, `info`. |
| `type` | `bug`, `vulnerability`, `code_smell`, `security_hotspot`. |
| `status` | Current issue status, such as `open` or `closed`. |
| `tracking_state` | Lifecycle in the selected scope: `new`, `unchanged`, `reopened`, `unknown`. |
| `language` | Derived from file extension, for example `go`, `javascript`, `typescript`, `python`, `rust`. |
| `rule_key` | Exact rule key. |
| `tag` | Exact rule or issue tag. |
| `security_category` | Security-oriented tag such as `owasp-*`, `cwe-*`, `injection`, `auth`, `crypto`, or `secrets`. |
| `directory` | Directory prefix. |
| `file` | File path substring search. |
| `limit`, `offset` | List pagination for `/issues`; facets ignore pagination. |

`GET /api/v1/issues/facets` returns maps named `by_quality`, `by_severity`, `by_lifecycle`, `by_type`, `by_status`, `by_language`, `by_rule`, `by_tags`, `by_security_category`, `by_directory`, `by_file`, and `by_engine_id`.

Quality domains and languages are derived during report generation and API reads, so older reports remain queryable. Testability is selected by tags such as `testability`, `coverage-gap`, `mutation`, `survived-mutant`, `failing-test`, or `flaky-test`.

### Tag catalog and saved filters

The tag catalog governs labels used across issues, projects, built-in rules, and custom rules. Existing report tags remain compatible: ingestion discovers unknown issue tags and inserts them into the catalog as `discovered` entries.

Common tag request fields:

| Field | Meaning |
|-------|---------|
| `key` | Stable normalized key, such as `team-api` or `owasp-a01`. |
| `display_name` | Human-friendly name shown in the UI. |
| `description` | Catalog description for the vocabulary entry. |
| `color` | Hex color used by tag chips and swatches. |
| `owner_type`, `owner_id`, `owner_name` | Optional owner metadata for governance. |
| `scope` | Optional vocabulary scope, such as a team, domain, or project grouping. |

Bulk edits use this request shape:

```json
{
  "target_type": "issue",
  "target_ids": [101, 102],
  "target_keys": [],
  "add_tags": ["team-api"],
  "remove_tags": ["legacy"]
}
```

`target_type` accepts `issue`, `project`, `rule`, or `custom_rule`. Preview returns the affected targets and before/after tag arrays; apply persists the same change and records audit entries.

Saved filters store reusable criteria for issue and governance workflows:

```json
{
  "name": "API security review",
  "visibility": "shared",
  "filter_type": "issues",
  "criteria": { "tag": "team-api", "severity": "critical" }
}
```

Applying a saved filter resolves tag aliases before returning criteria to the caller.

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

Scan reports may include optional aggregate test and mutation fields inside `measures`. Missing fields are ignored for backward compatibility. Reports may also include `quality_profiles`, a scanner policy snapshot array. New reports include profile source, profile name/id, active rule count, stable rules hash, and diagnostics per language; legacy reports without this field are accepted and marked as profile metadata unavailable.

| Measure key | Meaning |
|-------------|---------|
| `coverage` | Project coverage percentage, when supplied by an external test/coverage tool. |
| `tests` | Total unit tests. |
| `test_failures` | Failed tests. |
| `test_errors` | Tests that errored before assertion completion. |
| `test_skipped` | Skipped tests. |
| `test_duration_ms` | Total test duration in milliseconds. |
| `mutation_score` | Mutation score percentage. |
| `mutants_total` | Total generated mutants. |
| `mutants_killed` | Mutants killed by tests. |
| `mutants_survived` | Mutants that survived. |
| `mutants_timeout` | Mutants that timed out. |
| `mutants_error` | Mutants that errored. |
| `mutants_skipped` | Mutants skipped during execution. |
| `changed_mutation_score` | Mutation score restricted to changed code. |
| `changed_mutants_total` | Total mutants generated for changed code. |
| `changed_mutants_killed` | Changed-code mutants killed by tests. |
| `changed_mutants_survived` | Changed-code mutants that survived. |

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

## Measures Live

Fast current-value queries without expensive subqueries. Also supports daily trend rollups.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/projects/{key}/measures/live` | Current values for all project measures |
| `GET` | `/api/v1/projects/{key}/measures/trend?metric=coverage&days=30` | Daily aggregated trend for a metric |
| `GET` | `/api/v1/projects/{key}/measures/trend?metric=mutation_score&component=domain` | Per-module metric trend with optional `component` filter |

## Components

Persistent project directory tree with issue counts.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/projects/{key}/components` | Directory tree with issue counts per node |
| `GET` | `/api/v1/projects/{key}/components?path=src/` | Subtree starting at path |

## Quality Gates & Profiles

Quality Profiles and Quality Gates are separate APIs. Profiles decide which rules run and with which effective severity/params. Gates evaluate the resulting metrics after ingestion.

| Method | Endpoint                                           | Description |
|--------|----------------------------------------------------|-------------|
| GET    | `/api/v1/quality-gates`                            | List quality gates |
| POST   | `/api/v1/quality-gates`                            | Create quality gate |
| GET    | `/api/v1/quality-gates/{id}`                       | Get gate details with conditions |
| PUT    | `/api/v1/quality-gates/{id}`                       | Update gate |
| DELETE | `/api/v1/quality-gates/{id}`                       | Delete gate |
| POST   | `/api/v1/quality-gates/{id}/conditions`            | Add condition to gate |
| PUT    | `/api/v1/quality-gates/{id}/conditions/{cid}`      | Update condition |
| DELETE | `/api/v1/quality-gates/{id}/conditions/{cid}`      | Remove condition |
| POST   | `/api/v1/quality-gates/{id}/copy`                  | Copy gate with all conditions |
| POST   | `/api/v1/quality-gates/{id}/set-default`           | Set gate as global default |
| POST   | `/api/v1/projects/{key}/quality-gate`              | Assign gate to project |
| GET    | `/api/v1/profiles`                                 | List quality profiles |
| POST   | `/api/v1/profiles`                                 | Create quality profile |
| GET    | `/api/v1/profiles/{id}`                            | Get profile |
| GET    | `/api/v1/profiles/{id}/effective-rules`            | Resolved profile rules after inheritance, overrides, and disabled markers |
| GET    | `/api/v1/profiles/{id}/export`                     | Export a JSON profile-as-code document |
| GET    | `/api/v1/profiles/{id}/changelog`                  | Paginated profile change history |
| POST   | `/api/v1/profiles/{id}/rules`                      | Activate rule in profile |
| DELETE | `/api/v1/profiles/{id}/rules/{ruleKey}`            | Deactivate rule |
| POST   | `/api/v1/profiles/{id}/import`                     | Import profile from JSON or YAML |
| GET    | `/api/v1/projects/{key}/profiles`                  | Project profile assignment/default state per supported language |
| POST   | `/api/v1/projects/{key}/profiles`                  | Assign a profile to a project language |
| GET    | `/api/v1/projects/{key}/profiles/effective`        | Effective scanner policy for all supported languages |

## Custom Rule Packs

Custom Rule Packs are catalog entries managed separately from Quality Profiles. Published custom rules appear in `/api/v1/rules`, can be activated in profiles, and are fetched by scanners as a scan-start catalog snapshot. Draft, invalid, disabled, and deprecated rules stay out of scanner/profile catalog responses. Draft/detail lifecycle endpoints require the `admin` permission; the published catalog snapshot is readable by authenticated scanners.

| Method | Endpoint                                      | Description |
|--------|-----------------------------------------------|-------------|
| GET    | `/api/v1/rule-engines`                        | List supported declarative custom rule engines and capabilities |
| GET    | `/api/v1/custom-rules`                        | List custom rule versions for Rule Studio/admin views |
| POST   | `/api/v1/custom-rules`                        | Import/create a draft rule pack document |
| POST   | `/api/v1/custom-rules/import`                 | Import a JSON or YAML rule pack document |
| GET    | `/api/v1/custom-rules/{id}`                   | Get a custom rule version |
| PUT    | `/api/v1/custom-rules/{id}`                   | Update an editable draft, or create a new draft version from a published rule |
| POST   | `/api/v1/custom-rules/{id}/validate`          | Run CGo-free validation and example checks |
| POST   | `/api/v1/custom-rules/{id}/preview`           | Preview matches against a supplied source snippet for CGo-free engines |
| POST   | `/api/v1/custom-rules/{id}/publish`           | Publish a rule version after current validation passes |
| POST   | `/api/v1/custom-rules/{id}/disable`           | Disable a custom rule version |
| POST   | `/api/v1/custom-rules/{id}/deprecate`         | Deprecate a custom rule version |
| GET    | `/api/v1/custom-rules/{id}/export`            | Export a rule pack document containing the selected rule |
| GET    | `/api/v1/custom-rules/{id}/audit`             | Paginated lifecycle audit entries |
| GET    | `/api/v1/custom-rules/catalog`                | Published custom rule catalog snapshot used by scanners |
| GET    | `/api/v1/custom-rules/ai/models`              | List AI providers, model states, setup status, and local/cloud labels without secrets |
| POST   | `/api/v1/custom-rules/ai/suggest`             | Generate an editable custom rule draft suggestion from provider, model, intent, and current draft fields |

Rule pack schema:

```yaml
version: 1
pack:
  name: Team Rules
  namespace: team
rules:
  - key: no-debug-print
    name: Debug prints should not be committed
    language: go
    type: code_smell
    severity: major
    engine: go-ast
    engine_config:
      pattern: forbidden_call
      target: fmt.Println
    message: Remove debug printing before committing.
    examples:
      - name: compliant
        code: 'package main; func main() { println("ok") }'
        compliant: true
      - name: noncompliant
        code: 'package main; import "fmt"; func main() { fmt.Println("debug") }'
        compliant: false
```

`ollantaweb` keeps validation CGo-free. Text and Go AST rules can be validated and published directly through the server. Tree-sitter query compilation and full source preview require scanner/local runtime support; server validation records that runtime validation is required until an engine-capable validator is used.

Rule Studio AI assistance returns suggestions only. It does not create, validate, publish, or activate a rule unless the user explicitly saves the draft and follows the normal lifecycle. Provider metadata exposes setup state and model identifiers but never returns API keys. Ollama is treated as a local provider via `OLLANTA_AI_OLLAMA_BASE_URL`; cloud providers use server-side base URL, model list, label, and credential configuration. The default OpenAI list is `gpt-5.5`, `gpt-5.4`, `gpt-5.4-mini`, and `gpt-5.4-nano`; Ollanta uses `OLLANTA_AI_OPENAI_API=responses` for OpenAI's endpoint and accepts `OLLANTA_AI_OPENAI_API=chat` for chat-completions-compatible providers. Anthropic Claude is available through `ANTHROPIC_API_KEY` and the Messages API, with default models `claude-opus-4-7`, `claude-sonnet-4-6`, and `claude-haiku-4-5`. Kimi K2 uses Moonshot's Chat Completions API with `MOONSHOT_API_KEY` or `KIMI_API_KEY`, default base URL `https://api.moonshot.ai/v1`, and default model `kimi-k2.6`. Qwen uses DashScope's OpenAI-compatible Chat Completions API with `DASHSCOPE_API_KEY` or `QWEN_API_KEY`, default base URL `https://dashscope-intl.aliyuncs.com/compatible-mode/v1`, and default model `qwen3.6-max-preview`. Use each provider's `OLLANTA_AI_<PROVIDER>_MODELS`, `OLLANTA_AI_<PROVIDER>_MODEL`, `OLLANTA_AI_<PROVIDER>_BASE_URL`, or `OLLANTA_AI_<PROVIDER>_LABEL` variables to override details.

### Profile-as-code schema

`POST /api/v1/profiles/{id}/import` accepts JSON or YAML. `GET /api/v1/profiles/{id}/export` returns JSON using the same schema.

```yaml
version: 1
profiles:
  - language: go
    name: Strict Go
    rules:
      - key: go:no-large-functions
        severity: critical
        params:
          max_lines: "30"
      - key: go:todo-comment
        active: false
```

Single-language documents can also place `language`, `name`, and `rules` at the top level. A rule can use `key`, `rule_key`, or `rule`. `active: false`, `activate: false`, or `severity: off` stores a disabled marker so child profiles and effective responses can explain why the rule is inactive.

### Effective profile response

`GET /api/v1/projects/{key}/profiles/effective` returns one item per supported language. Each item includes:

| Field | Meaning |
|-------|---------|
| `language` | `go`, `javascript`, `typescript`, `python`, or `rust` |
| `profile_id`, `profile_name` | Resolved assigned/default profile when available |
| `source` | `assigned`, `default`, `local`, `remote`, or `builtin` |
| `rules` | Effective rules with `origin` (`local`, `inherited`, `overridden`, `disabled`) |
| `active_rule_count` | Count excluding disabled/OFF rules |
| `rules_hash` | Stable hash of normalized rules, severities, params, and disabled markers |
| `has_rules`, `parser_only` | Distinguishes actionable languages from parser-only languages |
| `diagnostics` | Warnings such as missing defaults or parser-only states |

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

## Activity timeline contract

`GET /api/v1/projects/{key}/activity` powers the project Activity tab. The response is paginated and supports `limit` and `offset` query parameters; the Activity UI fetches 60 entries per page and sends `offset=N` when the user clicks "Load more".

The `events[].category` enum is part of the public contract — the UI uses it to drive client-side filtering and event styling. Current values:

- `QUALITY_GATE` — gate status changed since the previous scan.
- `ISSUE_SPIKE` — significant rise in total issues.
- `FIRST_ANALYSIS` — first scan recorded for the project.
- `VERSION` — synthetic event derived from the scan's `project_version` field.
- `ANALYSIS` — default category for an analysis with no other notable signals.

The Activity tab also issues `GET /api/v1/issues?scan_id={id}` for drill-down navigation; `scan_id` is already supported by the issues endpoint.

