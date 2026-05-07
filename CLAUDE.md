# Ollanta â€” Project Guardrails

Ollanta is a multi-language static analysis platform in Go. It scans source code, reports quality issues (bugs, code smells, vulnerabilities), computes metrics, and evaluates configurable quality gates.

**Two main components:**
- **Scanner** (`ollantascanner`) â€” CLI that discovers files, parses, applies rules, produces JSON/SARIF reports. Optional local web UI on port 7777.
- **Server** (`ollantaweb`) â€” receives scan reports, tracks issues across scans, stores history, evaluates quality gates, REST API on port 8080.

**Supported languages:** Go (native `go/ast`), JavaScript, TypeScript, Python, Rust (tree-sitter).

## Architecture

Hexagonal architecture (ports & adapters) with three rings:

| Ring | Modules | Deps allowed |
|------|---------|--------------|
| **Inner** | `domain/` | Go stdlib only |
| **Middle** | `application/` | `domain/` only |
| **Outer** | `adapter/`, `ollantaweb/`, `ollantastore/` | Everything |

**Legacy modules** (`ollantacore/`, `ollantaparser/`, `ollantarules/`, `ollantascanner/`, `ollantaengine/`) coexist during migration. New code should target the hexagonal types (`domain/model`, `domain/port`).

### CGo Boundary

Only `ollantaparser` has CGo (tree-sitter C library). The domain layer uses `any` for tree-sitter types to stay CGo-free. `ollantaweb` must NEVER import `ollantaparser` or `ollantarules` transitively â€” its Dockerfile builds with `CGO_ENABLED=0`.

### Adapter Bridge Pattern

`adapter/secondary/rules/bridge.go` converts between legacy types (`ollantacore/domain.Issue`) and hexagonal types (`domain/model.Issue`). Always use the bridge â€” never mix types directly.

## Module Layout

| Module | Purpose | CGo |
|--------|---------|-----|
| `domain/` | Pure models, port interfaces, domain services | No |
| `application/` | Use cases: scan, ingest, analysis | No |
| `adapter/` | HTTP, OAuth, Postgres, Parser, Rules bridge, Telemetry, Webhook | Yes* |
| `ollantacore/` | Legacy shared types: Issue, Rule, Component, Measure | No |
| `ollantaparser/` | Tree-sitter C bindings â€” **only true CGo module** | **Yes** |
| `ollantarules/` | Rule registry, Go/tree-sitter sensors, metadata | Yes* |
| `ollantascanner/` | CLI entry point, file discovery, parallel executor | Yes* |
| `ollantaengine/` | Quality gates, issue tracking, metric summarization | No |
| `ollantastore/` | PostgreSQL repos (pgx/v5), search (ZincSearch/Postgres FTS) | No |
| `ollantaweb/` | REST server, ingestion, auth, webhooks (chi/v5) | No |

_*Transitive CGo via `ollantaparser`._

## Developer Setup

### Prerequisites
- Go 1.21+, CGo toolchain (gcc/clang or MSYS2 MinGW on Windows)
- Docker & Docker Compose
- Node.js (for scanner frontend build)

### Commands

| Command | Description |
|---------|-------------|
| `make build` | Build all scanner modules |
| `make test` | Test all scanner modules |
| `make lint` | Lint per module (5 modules separately) |
| `make fmt` | Format source code |
| `make run` | Run scanner (overridable: `PROJECT_DIR`, `PROJECT_KEY`, `PORT`) |
| `make clean` | Clean build artifacts |

**Docker Compose profiles:**
- `docker compose --profile scanner up local-ui` â€” scanner local UI on 7777
- `docker compose --profile server up` â€” postgres + zincsearch + ollantaweb (8080)
- `docker compose run --rm push` â€” scan + push results to server

**Scanner push authentication:**

The push service sends results to `ollantaweb` via `POST /api/v1/scans`, which requires auth. A pre-shared scanner token avoids needing a user account:

| Variable | Where | Default (dev) |
|----------|-------|---------------|
| `OLLANTA_SCANNER_TOKEN` | `ollantaweb` (server side) | `ollanta-dev-scanner-token` |
| `OLLANTA_TOKEN` | `push` service (scanner side) | `ollanta-dev-scanner-token` |

For production, set both to the same strong random secret in `.env`. If `OLLANTA_SCANNER_TOKEN` is empty, scanner push falls back to regular JWT/API token auth.

### After Code Changes

**Scanner/rules (CGo modules):** `make build` â†’ `make test`

**Server (`ollantaweb`):** changes take effect on rebuild: `docker compose --profile server build ollantaweb`

**Scanner frontend:** `cd ollantascanner/server/static && npm run build`

### CI Pipeline

4 parallel jobs on push/PR to `main`:
- `test-scanner` (CGo) â€” ollantacore, ollantaparser, ollantarules, ollantascanner, ollantaengine
- `test-web` (no CGo) â€” ollantaweb, ollantastore, domain, application
- `test-adapter` (CGo + Postgres service) â€” adapter/
- `lint` â€” golangci-lint v2 per module

**IMPORTANT:** Never run `golangci-lint` at workspace root. Each module has its own `go.mod` â€” lint must run per-module.

## Guardrails

### 1. No Duplication of Data or Types

**Single source of truth.** Data files (rule JSONs, configs) live in exactly ONE location. If multiple modules need the same data, use `go:embed` from the canonical source or expose it via an API â€” never copy files across modules.

**No duplicated structs.** If two packages need the same struct, extract it to a shared package (`ollantacore/domain` or `domain/model`). Never define equivalent structs in multiple packages.

_Rationale: We had rule JSON files copied to 3 locations and identical `ruleDetail` structs in 2 packages. This causes silent drift._

### 2. Domain Purity

`domain/model.Rule` carries only what business logic needs: key, name, severity, type, language, params. Fields used purely for display live in the layer that displays them.

**Where each field belongs:**

| Field | Where it lives |
|-------|---------------|
| `key`, `name`, `severity`, `type`, `language`, `params` | `domain/model.Rule` |
| `rationale`, `noncompliant_code`, `compliant_code` | `ollantarules.RuleMeta` (source of truth) + handler DTO for the API response |
| Display labels, icons, colors | Handler or frontend only |

`ollantarules.RuleMeta` is the canonical store for rule documentation. The API handler reads directly from the embedded JSONs via `RuleMeta` â€” it does not go through `domain/model.Rule`. This keeps the domain pure without adding complexity to the rule-authoring workflow (still 3 files, no extra steps).

### 3. Error Handling

**Never silently discard errors.** The `_, _ = someFunc()` pattern is forbidden. Always handle or propagate errors.

```go
// BAD
data, _ := fs.ReadFile(embedFS, path)

// GOOD
data, err := fs.ReadFile(embedFS, path)
if err != nil {
    log.Printf("failed to read %s: %v", path, err)
    continue
}
```

The linter config excludes `*.Close` return values â€” that is the only accepted exception.

### 4. Hexagonal Boundary Enforcement

- **Inner ring** (`domain/`) imports only Go stdlib. No external deps, no CGo.
- **Middle ring** (`application/`) imports only `domain/`. Never imports adapters.
- **Outer ring** imports anything, but arrows always point inward.
- `ollantaweb` must NEVER transitively depend on CGo. Check with `go list -deps`.
- Use port interfaces in use cases, never concrete adapter types.

### 5. No Over-Engineering

- Don't add abstractions for one-time operations
- Don't create interfaces with a single implementation unless it's a hexagonal port
- Don't add error handling for impossible states
- Don't add generics or complex type parameters where a simple concrete type suffices
- Don't refactor existing working code unless directly related to the task

### 6. No Unnecessary Boilerplate

- Don't add docstrings to unexported functions
- Don't add comments that restate the code
- Don't create helper packages for one function
- Don't wrap errors that already have sufficient context

**Log com propÃ³sito.** Use `slog` com nÃ­vel adequado:

| NÃ­vel | Quando usar |
|-------|-------------|
| `slog.Info` | Eventos operacionais relevantes: scan iniciado/concluÃ­do, gate avaliado, webhook disparado |
| `slog.Debug` | Rastreabilidade detalhada: arquivo processado, regra aplicada, issue encontrada |
| `slog.Error` | Falhas: arquivo que nÃ£o pÃ´de ser parseado, erro de I/O, falha de webhook |

Sempre adicione contexto estruturado: `slog.With("file", path, "rule", key)` em vez de interpolaÃ§Ã£o em string.

Remova `log.Printf` / `fmt.Printf` ad-hoc antes do commit â€” substitua por `slog` com nÃ­vel e campos adequados.

### 7. Consistent HTTP Responses

All API handlers must return consistent JSON responses:
- Set `Content-Type: application/json` before writing any response
- Error responses use `{"error": "message"}` format
- Never mix `text/plain` Content-Type with JSON body

### 8. Chi Router Patterns

When a resource has both public GET and admin-only write operations, use `r.Route` with nested `r.Group`:

```go
r.Route("/resource", func(r chi.Router) {
    r.Get("/", handler.List)       // public (within auth)
    r.Get("/{id}", handler.Get)
    r.Group(func(r chi.Router) {
        r.Use(adminOnly)
        r.Post("/", handler.Create)
        r.Put("/{id}", handler.Update)
        r.Delete("/{id}", handler.Delete)
    })
})
```

Never place GET handlers outside `r.Route` when sub-routes also exist for the same path â€” this causes 405 errors.

## Conventions

| Convention | Example |
|------------|---------|
| Interface naming: `I` prefix | `IProjectRepo`, `IAnalyzer` |
| Constructor naming: `New` prefix | `NewRegistry()`, `NewIngestUseCase()` |
| Test naming: `Test{Type}_{Scenario}` | `TestTreeSitterSensor_JS_LargeFunction` |
| Rule keys: `{lang}:{kebab-name}` | `go:no-large-functions`, `py:broad-except` |
| JSON tags: lowercase snake_case | `json:"rule_key"` |
| Optional fields: `omitempty` | `json:"resolution,omitempty"` |
| Secret fields: `json:"-"` | `PasswordHash`, `Secret`, `TokenHash` |
| Context as first arg | `func(ctx context.Context, ...)` |
| Pointer receivers for stateful structs | `(r *Registry) Register(...)` |
| Compile-time interface checks | `var _ port.IAnalyzer = (*AnalyzerBridge)(nil)` |
| Sentinel errors | `var ErrNotFound = errors.New("not found")` |
| Package-level doc comments | `// Package ingest ...` |

## Adding a New Rule

3 files, no edits to existing files:

1. **Rule logic:** `ollantarules/languages/{lang}/rules/my_rule.go`
2. **Metadata JSON:** `ollantarules/languages/{lang}/rules/{lang}_{rule-key}.json`
3. **Registration:** Add to `MustRegister()` call in the language's `embed.go`

`MustRegister` panics at startup if MetaKey is missing from JSON or key is duplicated.

## Known Gotchas

### Scanner Push Returns 401

If `docker compose run --rm push` fails with `server returned 401`, ensure `OLLANTA_SCANNER_TOKEN` is set on `ollantaweb` and `OLLANTA_TOKEN` matches it on the push service. The dev defaults align automatically.

### URL Encoding in Rule Keys

Rule keys contain colons (`go:no-large-functions`). When passing through URLs:
- Frontend: `encodeURIComponent(ruleKey)` encodes `:` â†’ `%3A`
- Backend: Always `url.PathUnescape()` wildcard route params before lookup

### Rule Metadata Shared via `rulecatalog` (CGo-Free)

`ollantarules` is the canonical source of rule implementations. Both it and `ollantaweb` read rule metadata from `ollantacore/rulecatalog` — a CGo-free Go package. When adding or updating a rule:
1. Rule logic: `ollantarules/languages/{lang}/rules/my_rule.go`
2. Metadata: add to `ollantacore/rulecatalog/catalog.go` (key, name, language, type, severity, tags, params, description, rationale, code examples)
3. Registration: add to `MustRegister()` in the language's `embed.go`

## Commits & PRs

- Branch naming: `username/brief-description`
- Conventional commits: `feat:`, `fix:`, `chore:`, `test:`, `docs:`
- Run `make lint` and `make test` before pushing
- Flag security implications in PR description

## Key Anti-Patterns to Avoid

| Anti-pattern | Why it's bad |
|-------------|-------------|
| Importing `ollantaparser` from `domain/` or `application/` | Breaks hexagonal boundary, introduces CGo |
| Copying data files across modules | Silent drift, maintenance burden |
| Duplicating struct definitions | Types diverge silently |
| `_, _ = f()` â€” discarding errors | Bugs hide, debugging is harder |
| Putting SQL/HTTP/CGo in inner rings | Domain must stay pure |
| Running `golangci-lint` at workspace root | Each module has own `go.mod` |
| Mixing `model.Issue` and `domain.Issue` without bridge | Legacy and hexagonal types are separate |
| `log.Printf` debug statements left in code | Noise in production logs |
