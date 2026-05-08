# How To Use Ollanta

This guide is the first journey: scan a project, push to the server, inspect findings, and decide what to configure next.

## 1. The three-line happy path

```bash
make up          # start the server (docker, one-time)
make run         # scan + open local interactive UI at http://localhost:7777
make push        # scan + send results to the server at http://localhost:8080
```

`make run` blocks the terminal while serving the UI. Use `make run-bg` to run in background instead, and `make stop` to kill it.

Override defaults as needed:

```sh
make run   PROJECT_DIR=D:\projects\myapp PROJECT_KEY=my-app
make push  PROJECT_DIR=D:\projects\myapp PROJECT_KEY=my-app SERVER=http://prod:8080 TOKEN=my-secret
```

## 2. What each mode gives you

| Command | What it does |
|---------|-------------|
| `make run` | Scan + embedded local UI on port 7777. No database. Writes `.ollanta/report.json`. |
| `make push` | Scan + push to `ollantaweb` server. Results stored in PostgreSQL with history, tracking, and gates. |
| `make up` | Start the full docker server stack (postgres, ollantaweb, workers). |
| `make down` | Stop the server stack. |
| `make stop` | Kill the scanner background process (from `make run-bg`). |

## 3. Start the server

```bash
make up
```

Open `http://localhost:8080` and sign in:
- Login: `admin`
- Password: `admin`

The server stack uses local development defaults. For shared environments, create a `.env` file overriding `PG_PASSWORD`, `OLLANTA_JWT_SECRET`, and `OLLANTA_SCANNER_TOKEN`.

For high-throughput environments:

```bash
OLLANTA_WORKER_POOL=16 make up
```

## 4. Inspect the Results

**Local UI** (`make run` → `http://localhost:7777`):
- Overview: quality gate, measures, severity distribution, hotspot files, mutation testing, test signal modules, coverage detail
- Issues: filter by severity, type, rule; search; view rule details with rationale and code examples
- Coverage: file-level coverage with source viewer, covered/uncovered line markers
- Detail panel: severity, type, rule, engine, location, tags; Rule tab with rationale/code; Fix with AI tab

**Server UI** (`http://localhost:8080`):
- Overview: review summary, new code metrics, changed-code mutation, survived mutants, hotpot files
- Issues: full faceted search by severity, type, quality, lifecycle, language, rule, tag, directory
- Coverage: browse stored code snapshots with inline issue markers
- Activity: compare scans over time with trend charts
- Scopes: switch between branches and pull requests
- Quality Gate: review pass/fail/warn conditions
- Profiles: choose which rules run per language; import/export profile-as-code
- Rule Studio: create and publish custom rules; AI-assisted draft generation
- Tags: governed tag catalog with audit trail

## 5. Scanner exit codes

| Exit code | Meaning |
|-----------|---------|
| `0` | Success or `-skip` activated |
| `1` | Internal error (I/O, parse failure) |
| `2` | User error (invalid flag, bad config) |
| `3` | Quality Gate `ERROR` (scan succeeded, gate failed) |

## 6. Configure only what you need

Copy the starter config only when you need repeatable settings:

```sh
cp config.toml.example config.toml
```

- Scanner settings: `[scanner]`, `[tests]`, `[mutations]`
- Server settings: `[server]`, `[database]`, `[search]`, `[ui]`
- Binaries can read custom paths via `OLLANTA_CONFIG_FILE=/path/to/config.toml`

## 7. Customize rules

Local-only rules: place Custom Rule Pack files in `.ollanta/rules/`:

```yaml
# .ollanta/rules/team-rules.yaml
version: 1
rules:
  - key: team:no-debug-print
    name: No debug prints in production
    language: go
    engine: text
    engine_config: { pattern: "fmt.Println", target: "" }
    type: code_smell
    severity: major
    message: Remove debug printing before committing.
```

Server-managed rules via Rule Studio:
1. Create or import a draft
2. Validate examples
3. Publish the rule
4. Add to a Quality Profile
5. Re-scan with server profiles enabled

## 8. Alternative: raw CLI commands

If you prefer not to use `make`, the equivalent raw commands are:

```sh
# Local scan + UI
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . -project-key myapp -format all \
  -with-tests -with-mutations -local-ui

# Push to server
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . -project-key myapp -format all \
  -with-tests -with-mutations \
  -server http://localhost:8080 -server-token ollanta-dev-scanner-token -server-wait
```

Or via Docker:

```sh
docker compose --profile server up -d                     # start server
export PROJECT_DIR=. PROJECT_KEY=myapp
docker compose --profile push run --build --rm push       # push scan
```

## 9. Troubleshooting

**Scanner already running on port 7777:**
```sh
make stop           # kill the background scanner
```

**Server containers not starting:**
```sh
docker compose --profile server ps
docker compose --profile server logs --tail=100 ollantaweb
```

**Push appears stuck:**
The scanner waits for server processing when `-server-wait` is active. `make push` uses it by default to confirm the scan is fully ingested.

**Config variable interpolation:**
`config.toml` supports `${VAR}`, `${env.VAR}`, and `${env.VAR:-default}`. Use `$$` for a literal `$`.

**Global config file:**
Place shared settings in `~/.ollanta/config.toml` (`%USERPROFILE%\.ollanta\config.toml` on Windows).

**Proxy support:**
```sh
ollanta -proxy http://corp-proxy:3128
# or set HTTP_PROXY / HTTPS_PROXY environment variables
```

**Skip scanning in CI:**
```sh
ollanta -skip
```
Useful when scanning is conditionally disabled. Exit code 0.
