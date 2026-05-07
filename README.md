<p align="center">
  <img src="docs/imgs/logo-dark.png" alt="Ollanta logo" width="420">
</p>

<p align="center">
  <img src="https://raw.githubusercontent.com/scovl/Ollanta/refs/heads/main/docs/imgs/o01.png" alt="Ollanta local dashboard">
</p>

# Ollanta

Ollanta is a multi-language static analysis platform written in Go. It scans source code, reports bugs, code smells and vulnerabilities, computes metrics, and evaluates configurable quality gates. You can use Ollanta in two ways:

- **Scanner**: a local CLI that scans a project and writes JSON/SARIF reports. It can also open an embedded local UI on port `7777`.
- **Server**: a centralized API that receives scan reports, stores history, tracks issues across scans, evaluates quality gates, exposes dashboards, and runs background workers.

For the full first-project flow, see [docs/how-to-use.md](docs/how-to-use.md).

## Language Support

| Language | Parser | Bundled rules |
|----------|--------|---------------|
| Go | Native `go/ast` | 8 |
| JavaScript | Tree-sitter | 4 |
| TypeScript | Tree-sitter | Parser support; no bundled rules yet |
| Python | Tree-sitter | 5 |
| Rust | Tree-sitter | Parser support; no bundled rules yet |

## Requirements

For the scanner and local development:

- Go `1.21+`
- CGO enabled
- A C compiler for Tree-sitter
  - Linux/macOS: install `gcc` or `clang` with your package manager
  - Windows: install [MSYS2](https://www.msys2.org/), then run `pacman -S mingw-w64-x86_64-gcc`
- Docker and Docker Compose, optional but recommended for the server stack

For frontend changes in the scanner UI, Node.js is also required under [ollantascanner/server/static](ollantascanner/server/static).

On Windows, the Makefile prepends `C:\msys64\mingw64\bin` to `PATH` for its own targets. If MSYS2 is installed somewhere else, add your MinGW `bin` directory to `PATH` before running Go commands.

## Quick Start

This section is the shortest happy path. The complete scanner-to-server journey is in [docs/how-to-use.md](docs/how-to-use.md).

### 1. Scan this checkout

From the repository root:

```sh
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . \
  -project-key ollanta-self \
  -format all \
  -local-ui
```

This is the fastest way to try Ollanta on Ollanta itself. Open `http://localhost:7777` to inspect issues, metrics, rule details, and optional `Fix with AI` suggestions. Reports are written to `.ollanta/` inside the scanned project:

- `.ollanta/report.json`
- `.ollanta/report.sarif`

### 2. Scan with Docker

```sh
docker compose --profile scanner up local-ui
```

By default this scans the current checkout. For another project, set `PROJECT_DIR` and `PROJECT_KEY` in your shell or in a local `.env` file before running the same command. PowerShell example:

```powershell
$env:PROJECT_DIR = 'D:\projects\myapp'
$env:PROJECT_KEY = 'myapp'
docker compose --profile scanner up local-ui
```

### 3. Generate CI-friendly reports only

```sh
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . \
  -project-key my-project \
  -format sarif
```

Use `-format json`, `-format sarif`, or `-format all` depending on what your CI consumes.

### 4. Start the centralized server

```sh
docker compose --profile server up -d
```

This starts:

- PostgreSQL on port `5432`
- ZincSearch on port `4080`
- `ollantaweb` on port `8080`
- `ollantaworker`, `ollantaindexer`, and `ollantawebhookworker`

The local Docker stack works without extra variables. It uses `admin` / `admin` for the seeded development login and `ollanta-dev-scanner-token` for scanner pushes. Override `PG_PASSWORD`, `OLLANTA_JWT_SECRET`, and `OLLANTA_SCANNER_TOKEN` in a local `.env` file for any shared or long-lived environment.

### 5. Push a scan to the server

```sh
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . \
  -project-key my-project \
  -format all \
  -server http://localhost:8080 \
  -server-token ollanta-dev-scanner-token \
  -server-wait
```

`-server-wait` makes the CLI wait until the accepted server-side scan job finishes. Without it, the server returns after accepting the job.

When waiting is enabled, the scanner may exit non-zero after a successful push if the evaluated Quality Gate is `ERROR`. Treat that as a CI gate failure, not as an ingestion failure. Docker push mode is also available:

```sh
docker compose --profile push run --build --rm push
```

Set `PROJECT_DIR`, `PROJECT_KEY`, and `OLLANTA_SERVER_WAIT=true` in your shell or `.env` when pushing a different project or when you want the containerized scanner to wait for server-side processing.

## Configuration

Ollanta can read settings from [config.toml.example](config.toml.example). The example is intentionally small: scanner defaults, optional server push settings, and local server connectivity. Use [docs/how-to-use.md](docs/how-to-use.md) for the recommended scanner/server configuration split.

```sh
cp config.toml.example config.toml
```

Use CLI flags for one-off runs and `config.toml` when the same project or CI job should be repeatable. Advanced test, coverage, and mutation evidence settings are documented in [docs/test-signals.md](docs/test-signals.md). Configuration precedence:

| Runtime | Precedence |
|---------|------------|
| Scanner | defaults, then `config.toml`, then CLI flags |
| Server | defaults, then `config.toml`, then `OLLANTA_*` environment variables |

Important environment variables for Docker/server use:

| Variable | Default in local compose | Purpose |
|----------|--------------------------|---------|
| `PROJECT_DIR` | `.` | Host project directory mounted into the scanner container |
| `PROJECT_KEY` | `project` | Project identifier used in reports and server history |
| `OLLANTA_SERVER` | `http://ollantaweb:8080` | Server URL used by Docker push mode |
| `OLLANTA_TOKEN` | `ollanta-dev-scanner-token` | Scanner token sent by Docker push mode |
| `OLLANTA_SCANNER_TOKEN` | `ollanta-dev-scanner-token` | Token accepted by `ollantaweb` for scan push |
| `PG_PASSWORD` | `ollanta_dev` | PostgreSQL password for the compose stack |
| `ZINC_USER` | `admin` | ZincSearch user |
| `ZINC_PASSWORD` | `ollanta_dev` | ZincSearch password |

## Common Scanner Flags

| Flag | Purpose |
|------|---------|
| `-project-dir` | Root directory to scan |
| `-project-key` | Stable project identifier |
| `-sources` | Comma-separated source patterns; default is `./...` |
| `-exclusions` | Comma-separated glob exclusions |
| `-format` | `summary`, `json`, `sarif`, or `all` |
| `-local-ui` | Start the embedded local UI |
| `-bind` | Bind address for the local UI |
| `-port` | Port for the local UI; default is `7777` |
| `-server` | `ollantaweb` URL for pushing results |
| `-server-token` | Bearer token used when pushing results |
| `-server-wait` | Wait for server-side processing to complete |
| `-profile-source` | Quality profile source: `auto`, `local`, `server`, or `builtin` |
| `-profile-file` | JSON or YAML profile-as-code file for local scans |
| `-profile-strict` | Fail when the requested profile source cannot be loaded |
| `-profile-fetch-timeout` | Timeout for fetching effective profiles from the server |
| `-config` | Explicit path to a TOML config file |

Branch and pull request metadata can be provided with `-branch`, `-commit-sha`, `-pull-request-key`, `-pull-request-branch`, and `-pull-request-base`.

Test and mutation evidence collection is optional. See [docs/test-signals.md](docs/test-signals.md) for `collect`, `run`, and `doctor` modes.

## Quality Profiles

Quality Profiles decide which rules run. Quality Gates decide whether the finished scan passes based on metrics such as bugs, coverage, or mutation score. In server mode, Ollanta resolves the project profile per language and the scanner enforces that policy before analysis. In local mode, you can use profile-as-code without a server:

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

Run it with:

```sh
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . \
  -project-key my-project \
  -profile-source local \
  -profile-file profiles.yaml \
  -format all
```

Reports include `quality_profiles` snapshots with active rule counts and stable rule hashes. The server stores those snapshots during ingest and marks older reports without this field as profile metadata unavailable.

## Custom Rule Packs

Custom Rule Packs add declarative team rules without rebuilding Ollanta. Local scans load packs from `.ollanta/rules/*.yaml`, `.ollanta/rules/*.yml`, and `.ollanta/rules/*.json`; server users can create, validate, publish, and activate rules through Rule Studio. AI-assisted drafts are available through server-side provider configuration, but generated rules still require validation, publication, and Quality Profile activation.

See [docs/rules.md](docs/rules.md) for pack schema, examples, AI provider setup, and the full custom-rule smoke flow.

## Validate A Local Checkout

```sh
make build
make test
make smoke-local
```

The Makefile covers the scanner-side CGO modules. For the full contributor workflow, including module-specific tests and linting, see [CONTRIBUTIONS.md](CONTRIBUTIONS.md) and [docs/contributing.md](docs/contributing.md).

## Documentation

- Architecture: [docs/architecture.md](docs/architecture.md) and [docs/arquitetura.md](docs/arquitetura.md)
- API: [docs/api.md](docs/api.md)
- Authentication: [docs/authentication.md](docs/authentication.md)
- Background tasks: [docs/background-tasks.md](docs/background-tasks.md)
- Database: [docs/database.md](docs/database.md)
- Issue tracking: [docs/issue-tracking.md](docs/issue-tracking.md)
- Observability: [docs/observability.md](docs/observability.md)
- Operations runbooks: [docs/operations-runbooks.md](docs/operations-runbooks.md)
- Quality gates: [docs/quality-gates.md](docs/quality-gates.md)
- Rules: [docs/rules.md](docs/rules.md)
- Test and mutation evidence: [docs/test-signals.md](docs/test-signals.md)
- Webhooks: [docs/webhooks.md](docs/webhooks.md)

## License

Apache-2.0. See [LICENSE](LICENSE).
