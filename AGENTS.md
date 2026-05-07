# AGENTS.md — Ollanta Operational Cheatsheet

## Module Map

10-module Go workspace (`go.work`). CGo status determines how you build:

| Module | CGo? | Notes |
|--------|------|-------|
| `domain/` | No | Pure stdlib, hexagonal inner ring |
| `application/` | No | Imports only `domain/` + `ollantacore/` |
| `ollantacore/` | No | Legacy shared types |
| `ollantastore/` | No | Postgres repos (pgx/v5) |
| `ollantaweb/` | No | REST server, **CGO_ENABLED=0** |
| `ollantaengine/` | No | Quality gates, metrics |
| `ollantaparser/` | **Yes** | True CGo module (go-tree-sitter C lib) |
| `ollantarules/` | Yes* | Rule registry, sensors |
| `ollantascanner/` | Yes* | CLI entry point |
| `adapter/` | Yes* | Parser/rules bridge, telemetry |

*\*Transitive CGo via `ollantaparser`.*

## Build / Test / Lint

**The root `Makefile` only covers 5 modules:** `ollantacore`, `ollantaparser`, `ollantarules`, `ollantascanner`, `ollantaengine`. It does **not** cover `domain/`, `application/`, `adapter/`, `ollantastore/`, or `ollantaweb/`.

### Scanner modules (CGo)
```sh
make build      # go build the 5 scanner modules
make test       # go test the 5 scanner modules
make lint       # golangci-lint per module (5 separate runs)
```

### Web/hexagonal modules (no CGo)
```sh
go build ./domain/... ./application/... ./ollantastore/... ./ollantaweb/...
go test ./domain/... ./application/... ./ollantastore/... ./ollantaweb/...
```

### Adapter module (CGo, needs Postgres for tests)
```sh
go test ./adapter/...
golangci-lint run ./adapter/...
```

### Frontend (ollantascanner/server/static)
```sh
cd ollantascanner/server/static
npm run build     # regenerates dist/app.js (embedded into scanner binary)
```

## Windows: CGo Requires MSYS2

The `Makefile` exports `CGO_ENABLED=1` and prepends `C:\msys64\mingw64\bin` to `PATH`. When running `go build`, `go test`, or `golangci-lint` directly on CGo-backed modules, set the same env first:

```powershell
$env:CGO_ENABLED = '1'
$env:PATH = 'C:\msys64\mingw64\bin;' + $env:PATH
```

## Linting

- **Never run `golangci-lint` at workspace root.** Each module has its own `go.mod` — lint runs per-module with `golangci-lint run ./ollantacore/...` etc.
- CI uses golangci-lint **v2.11.4**. Config at `.golangci.yml` (version `"2"` format).
- Only 5 modules are linted in CI: `ollantacore`, `ollantaparser`, `ollantarules`, `ollantascanner`, `ollantaengine`.

## Hexagonal Boundaries

- `domain/` — Go stdlib only. No external deps, no CGo.
- `application/` — imports only `domain/` and `ollantacore/`. Never imports adapters.
- `ollantaweb/` — **must NEVER transitively depend on `ollantaparser` or `ollantarules`.** Built with `CGO_ENABLED=0`. Adapter bridge (`adapter/secondary/rules/bridge.go`) converts legacy types ↔ domain types at the boundary.

## Adding a New Rule

3 files, no edits to existing files:
1. Rule logic: `ollantarules/languages/{lang}/rules/my_rule.go`
2. Metadata JSON: `ollantarules/languages/{lang}/rules/{lang}_{rule-key}.json`
3. Registration: add to `MustRegister()` in the language's `embed.go`
4. Also add the rule definition to `ollantacore/rulecatalog/catalog.go`

## Conventions

See `CLAUDE.md` for full conventions. Highlights:
- Interface naming: `I` prefix (`IProjectRepo`, `IAnalyzer`)
- Sentinel errors: `var ErrNotFound = errors.New("not found")`
- Compile-time interface checks: `var _ port.IAnalyzer = (*AnalyzerBridge)(nil)`
- Rule keys: `{lang}:{kebab-name}` — colons need URL encoding in HTTP routes
- Never discard errors (`_, _ = f()`). `*.Close` errors are the only exception.
- Use `slog` with structured context, not `log.Printf` / `fmt.Printf`

## Mandatory Validation (openspec/specs/integration-guardrails)

Before marking any implementation task as complete, verify:

- **UI dead component**: every render function is called in the render tree; every `getElementById` target exists in rendered HTML
- **Flag/config drift**: new CLI flags are added to `docker-compose.yml` where relevant; new config fields have TOML + ENV wiring
- **Wire-through**: new handlers are routed in `router.go`; new functions have ≥1 non-test caller
- **Cleanup completeness**: deleted directories have zero remaining imports; removed embed directives have no stale `"embed"` imports; docs updated for changed approaches

