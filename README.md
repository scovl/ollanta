<p align="center">
  <img src="docs/imgs/logo-dark.png" alt="Ollanta logo" width="420">
</p>

<p align="center">
  <img src="https://raw.githubusercontent.com/scovl/Ollanta/refs/heads/main/docs/imgs/o01.png" alt="Ollanta screenshot">
</p>

Ollanta is a multi-language static analysis platform written in Go. It analyzes source code, reports quality issues, computes code metrics, and evaluates configurable quality gates so teams can enforce coding standards locally, in CI, and in centralized review workflows.

Inspired by [OpenStaticAnalyzer](https://github.com/sed-inf-u-szeged/OpenStaticAnalyzer), [Semgrep](https://semgrep.dev/), and [SonarQube](https://www.sonarqube.org/), Ollanta is organized as a modular platform where each concern lives in its own Go module.

---

## Language Coverage

| Language | Engine | Bundled rules |
|----------|--------|---------------|
| Go | Native (`go/ast`) | 8 built-in rules |
| JavaScript | Tree-sitter | 4 built-in rules |
| TypeScript | Tree-sitter | Parser and scan-pipeline support; no bundled rules yet |
| Python | Tree-sitter | 5 built-in rules |
| Rust | Tree-sitter | Parser and scan-pipeline support; no bundled rules yet |

---

## Architecture

Ollanta follows a hexagonal (ports and adapters) layout: inner modules stay dependency-light while adapters plug in at the edges. See [docs/architecture.md](docs/architecture.md) for the full module layout.

For contributor workflow and module-aware validation, see [CONTRIBUTIONS.md](CONTRIBUTIONS.md) and [docs/contributing.md](docs/contributing.md).

---

## Quick Start

### Local scanner UI

```sh
ollanta \
  -project-dir . \
  -project-key my-project \
  -format all \
  -serve
```

This runs a local scan and opens the embedded web UI at `http://localhost:7777`.

### Docker scanner UI

```sh
docker compose up serve
```

This builds the scanner image, scans the mounted project directory, and serves the embedded UI on port `7777`.

### Centralized server stack

```sh
docker compose --profile server up -d
```

This starts PostgreSQL, ZincSearch, the `ollantaweb` API on port `8080`, plus three background roles: `ollantaworker`, `ollantaindexer`, and `ollantawebhookworker`.

### Push results to a centralized server

```sh
ollanta \
  -project-dir . \
  -project-key my-project \
  -format all \
  -server http://localhost:8080 \
  -server-token ollanta-dev-scanner-token
```

Add `-server-wait` if you want the CLI to wait for the accepted server-side scan job to finish instead of stopping at `202 Accepted`.

---

## Prerequisites

- **Go 1.21+** with CGO enabled
- **GCC** (required by the Tree-sitter runtime)
  - Linux/macOS: `gcc` from your system package manager
  - Windows: [MSYS2](https://www.msys2.org/) → `pacman -S mingw-w64-x86_64-gcc`
- **Docker** (optional) — for container-based scanning or running the server stack

---

## Build And Validation

```sh
# Build the scanner-side CGO modules
make build

# Run their tests
make test

# Run their linter
make lint

# Format those same modules
make fmt
```

Those targets cover `ollantacore`, `ollantaparser`, `ollantarules`, `ollantascanner`, and `ollantaengine`.

For `application/`, `domain/`, `ollantastore/`, `ollantaweb/`, and `adapter/`, use the module-aware workflow documented in [CONTRIBUTIONS.md](CONTRIBUTIONS.md) and [docs/contributing.md](docs/contributing.md).

On Windows, the `Makefile` prepends `C:\msys64\mingw64\bin` to `PATH` and sets `CGO_ENABLED=1` for its own targets.

---

## Common CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-project-dir` | `.` | Root directory to scan |
| `-project-key` | directory name | Identifier used in reports |
| `-sources` | `./...` | Comma-separated source patterns |
| `-exclusions` | none | Comma-separated glob patterns to exclude |
| `-format` | `all` | `summary`, `json`, `sarif`, or `all` |
| `-serve` | `false` | Open the local scanner UI after the scan |
| `-port` | `7777` | Port for `-serve` |
| `-bind` | `127.0.0.1` | Bind address for `-serve` |
| `-server` | none | URL of `ollantaweb` for pushed scans |
| `-server-token` | none | Bearer token used when pushing to `ollantaweb` |

Scope flags:

- `-branch`
- `-commit-sha`
- `-pull-request-key`
- `-pull-request-branch`
- `-pull-request-base`

Async server-wait flags:

- `-server-wait`
- `-server-wait-timeout`
- `-server-wait-poll`

See [docs/api.md](docs/api.md) for the async intake flow and the server-side scan-job API.

## Scope-aware Analysis

Ollanta now tracks scan state per analysis scope instead of flattening everything into one project timeline.

- Branch mode uses `-branch` when provided, otherwise it auto-detects the Git branch for `-project-dir`.
- Pull request mode is enabled when `-pull-request-key`, `-pull-request-branch`, and `-pull-request-base` are present, whether from flags or CI metadata.
- Supported CI pull-request environments are `OLLANTA_PULL_REQUEST_*`, GitHub Actions, GitLab CI, and Azure Pipelines.
- Detached HEAD executions fail fast unless `-branch` is provided explicitly.

If you are enabling scope-aware analysis on an existing project, set `main_branch` when the default branch is not `main` or `master`, then run one fresh analysis on the default branch to establish the current baseline.

---

## Fix With AI In The Local UI

The local scanner UI includes a `Fix with AI` tab in issue details.

Minimal OpenAI-compatible setup:

```sh
export OPENAI_API_KEY=your_api_key
export OLLANTA_AI_OPENAI_MODEL=gpt-4.1-mini
```

Local mock setup:

```sh
export OLLANTA_AI_ENABLE_MOCK=1
```

For multiple configured agents, use `OLLANTA_AI_AGENTS` with a JSON array. If you run the scanner through Docker Compose, export the AI-related environment variables before recreating `serve`.

## Docker Workflows

### Common commands

```sh
# Scan current directory and open UI at http://localhost:7777
docker compose up serve

# Scan a specific project
PROJECT_DIR=/path/to/myapp PROJECT_KEY=myapp docker compose up serve

# One-shot scan (no UI, just write report files)
docker compose run --rm scan-only

# Push a scan through Docker
OLLANTA_SERVER=http://your-server:8080 docker compose --profile push run --build --rm push
```

### Common overrides

| Variable | Default | Description |
|----------|---------|-------------|
| `PROJECT_DIR` | `.` | Host directory to scan |
| `PROJECT_KEY` | `project` | Project identifier |
| `PORT` | `7777` | Scanner UI port |
| `OLLANTA_SERVER` | `http://ollantaweb:8080` | API server URL for push mode |
| `OLLANTA_TOKEN` | `ollanta-dev-scanner-token` | Scanner token used by `push` |
| `OLLANTA_SERVER_WAIT` | `false` | Wait for the accepted scan job to finish |
| `PG_PASSWORD` | `ollanta_dev` | PostgreSQL password |
| `ZINC_USER` | `admin` | ZincSearch admin user |
| `ZINC_PASSWORD` | `ollanta_dev` | ZincSearch admin password |

See [docker-compose.yml](docker-compose.yml) for the full service configuration and the remaining environment defaults.

---

## Further Reading

- API reference: [docs/api.md](docs/api.md)
- Rules and rule authoring: [docs/rules.md](docs/rules.md)
- Quality gates: [docs/quality-gates.md](docs/quality-gates.md)
- Authentication: [docs/authentication.md](docs/authentication.md)
- Webhooks: [docs/webhooks.md](docs/webhooks.md)
- Issue tracking: [docs/issue-tracking.md](docs/issue-tracking.md)
- Kubernetes deployment: [docs/kubernetes.md](docs/kubernetes.md)
- Contributor workflow: [CONTRIBUTIONS.md](CONTRIBUTIONS.md)

---

## License

Apache-2.0 - see [LICENSE](LICENSE).
