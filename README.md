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

Ollanta follows a **hexagonal (ports & adapters)** layout — inner modules have no external dependencies; adapters plug in at the edges. See [docs/architecture.md](docs/architecture.md) for the full module layout.

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

Full REST API reference at [docs/api.md](docs/api.md). All `/api/v1` routes require a `Bearer` token or API token (`olt_…`) in the `Authorization` header.

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

17 built-in rules across Go, JavaScript, and Python. See [docs/rules.md](docs/rules.md) for the full reference and instructions on adding new rules.

---

## Quality Gates

Quality gates evaluate numeric metrics against configurable thresholds after every scan. The scanner CLI exits with code 1 when the gate fails. See [docs/quality-gates.md](docs/quality-gates.md).

## Authentication

Three mechanisms: local (JWT), OAuth (GitHub, GitLab, Google), and API tokens (`olt_…`). See [docs/authentication.md](docs/authentication.md).

## Webhooks

Projects can register outbound webhooks that fire on scan events, with HMAC-SHA256 signature verification and automatic retry. See [docs/webhooks.md](docs/webhooks.md).

## Server API

Full REST API reference (50+ endpoints) at [docs/api.md](docs/api.md).

---

## Issue Tracking

Each scan is compared against a previous baseline to classify issues as **new**, **unchanged**, **closed**, or **reopened**. See [docs/issue-tracking.md](docs/issue-tracking.md).

---

## License

MIT
