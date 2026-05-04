# How To Use Ollanta

This guide is the first customer journey: scan a real project, push the result to the server, inspect the findings, and decide what to configure next.

The examples use the Ollanta checkout itself as the target project because it exercises the same path a new team will use with their own repository.

## 1. Choose A Mode

Use the scanner-only mode when you want a fast local report or a CI artifact:

- CLI writes `.ollanta/report.json` and `.ollanta/report.sarif`.
- Optional local UI runs on `http://localhost:7777`.
- No database or server is required.

Use the server mode when you want history, issue tracking, quality gates, profiles, dashboards, custom rules, AI provider setup, or team access:

- `ollantaweb` runs on `http://localhost:8080`.
- PostgreSQL stores reports, issues, profiles, gates, users, jobs, and project metadata.
- ZincSearch provides search unless the deployment uses PostgreSQL search.
- Background workers process scan intake, indexing, and webhook delivery.

## 2. Run The First Local Scan

From the repository root:

```sh
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . \
  -project-key ollanta-self \
  -format all \
  -local-ui
```

Open `http://localhost:7777` and inspect the overview, issues, rule details, metrics, and generated report files under `.ollanta/`.

Docker scanner mode does the same thing without requiring a local Go toolchain:

```sh
docker compose --profile scanner up local-ui
```

For another project, set `PROJECT_DIR` and `PROJECT_KEY` before running the command.

PowerShell example:

```powershell
$env:PROJECT_DIR = 'D:\projects\myapp'
$env:PROJECT_KEY = 'myapp'
docker compose --profile scanner up local-ui
```

## 3. Start The Server

Start the local server stack:

```sh
docker compose --profile server up -d --build --wait
```

Open `http://localhost:8080` and sign in with the local development account:

- Login: `admin`
- Password: `admin`

The local Compose stack works without a `.env` file. It uses local development defaults for PostgreSQL, ZincSearch, the JWT secret, and the scanner token. For shared or long-lived environments, create a local `.env` and override at least `PG_PASSWORD`, `OLLANTA_JWT_SECRET`, and `OLLANTA_SCANNER_TOKEN`.

## 4. Push A Scan To The Server

With the server running, push the current checkout from the local CLI:

```sh
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . \
  -project-key ollanta-self \
  -format all \
  -server http://localhost:8080 \
  -server-token ollanta-dev-scanner-token \
  -server-wait
```

Containerized push mode is the customer-friendly Docker path:

```sh
docker compose --profile push run --build --rm push
```

Set these values when scanning a different project or when you want the push container to wait for server processing:

```powershell
$env:PROJECT_DIR = 'D:\projects\myapp'
$env:PROJECT_KEY = 'myapp'
$env:OLLANTA_SERVER_WAIT = 'true'
docker compose --profile push run --build --rm push
```

When server wait is enabled, a completed push can still exit non-zero if the evaluated Quality Gate is `ERROR`. In that case the scan was accepted and processed; the non-zero exit is the CI signal that the project did not pass the configured gate.

For repeated local validation after the scanner image already exists, omit `--build` to check runtime behavior without rebuilding the image.

## 5. Inspect The Result

In the server UI, open the project and check these tabs:

- Overview: latest gate status, issue trends, distributions, hotspots, and project details.
- Issues: filter by severity, type, quality, lifecycle, language, rule, tag, directory, or file.
- Coverage: browse the stored code snapshot with matching issues.
- Activity: compare scans over time.
- Scopes: switch between branches and pull requests.
- Quality Gate: review the pass/fail conditions.
- Profiles: choose which rules run for each language.
- Rule Studio: create and publish Custom Rule Packs.
- AI Providers: connect local or cloud models for Rule Studio draft generation.

## 6. Configure Only What You Need

The root `config.toml.example` is intentionally a starter, not a full reference. Copy it only when a project or local run needs repeatable settings:

```sh
cp config.toml.example config.toml
```

Recommended split:

- Scanner settings live under `[scanner]`, `[tests]`, and `[mutations]`.
- Server, worker, and migration settings live under `[server]`, `[database]`, `[search]`, and `[ui]`.
- Docker Compose should remain environment-first because containers usually receive secrets and endpoints from `.env`, CI, or the orchestrator.

Separate per-component TOML files are useful as deployment examples, but they should not be mandatory for first use. If you run binaries directly, point each one at the file you want with `OLLANTA_CONFIG_FILE=/path/to/config.toml`.

## 7. Customize Rules

For local-only rules, place Custom Rule Pack files in the scanned project:

```text
.ollanta/rules/team-rules.yaml
```

For server-managed rules, use Rule Studio:

1. Create or import a draft.
2. Validate examples.
3. Publish the rule.
4. Add it to a compatible Quality Profile.
5. Run another scan with server profiles enabled.

AI assistance only drafts rule content. A user still saves, validates, publishes, and activates the rule explicitly.

## 8. Understand Tags

Tags already exist as metadata on projects, rules, custom rules, and issues. They are useful today for issue filters, facets, quality-domain derivation, and security categories.

What is still missing is a first-class tagging workflow: a managed tag catalog, tag descriptions, bulk editing, project/rule tag pages, saved tag filters, and governance around team-specific vocabularies. Treat that as a separate product feature rather than a docs-only cleanup.

## 9. Troubleshoot The First Run

If Docker push appears stuck, check whether it is rebuilding the scanner image or waiting for server processing:

```sh
docker compose --profile server --profile push ps
docker ps --filter name=ollanta
```

If only the server containers are running and no `push` container exists, the push command already exited.

If the output ends with `Server job ... completed` followed by `gate=ERROR`, Docker did not hang. The scan completed and the scanner returned a failing Quality Gate exit code.

If a previous local UI is still occupying port `7777`, stop only that service:

```sh
docker compose --profile scanner stop local-ui
```

If server readiness fails, inspect the last logs:

```sh
docker compose --profile server logs --tail=100 ollantaweb ollantaworker ollantaindexer
```
