# Contributing to Ollanta

This is the short contributor checklist. Use it when you already know the codebase and need the right commands fast.

For the canonical guide with rationale, validation notes, and documentation expectations, see [docs/contributing.md](docs/contributing.md).

## Why There Are Two Contribution Guides

- `CONTRIBUTIONS.md`: quick operational reference from the repository root
- `docs/contributing.md`: canonical contributor guide with context and validation guidance

## Core Rules

- Keep changes focused and avoid unrelated refactors.
- Preserve the hexagonal boundaries and keep types in their canonical packages.
- Do not duplicate rule metadata or shared structs.
- Do not silently ignore errors.
- Update documentation when workflows, flags, environment variables, or user-facing behavior change.

## Validation Matrix

### 1. Scanner-side CGO modules

The root `Makefile` currently covers only these modules:

- `ollantacore`
- `ollantaparser`
- `ollantarules`
- `ollantascanner`
- `ollantaengine`

Use:

```sh
make build
make test
make lint
make fmt
```

These targets do not validate or format `application/`, `domain/`, `ollantastore/`, `ollantaweb/`, or `adapter/`.

### 2. Server-side and hexagonal modules

For `application/`, `domain/`, `ollantastore/`, and `ollantaweb/`, run targeted Go commands from the repository root:

```sh
go build ./application/... ./domain/... ./ollantastore/... ./ollantaweb/...
go test ./application/... ./domain/... ./ollantastore/... ./ollantaweb/...
```

There is no root `make lint` target for these modules today. If you need lint coverage there, run `golangci-lint` only on the module you touched and treat it as module-specific validation.

### 3. Adapter module

`adapter/` is also outside the root `Makefile`. Validate it directly.

On Windows, if you run `go` or `golangci-lint` directly against CGO-backed packages, export the same environment that the `Makefile` uses first:

```powershell
$env:CGO_ENABLED = '1'
$env:PATH = 'C:\msys64\mingw64\bin;' + $env:PATH
```

Then run:

```sh
go build ./adapter/...
go test ./adapter/...
golangci-lint run ./adapter/...
```

### 4. Local scanner frontend

For changes under `ollantascanner/server/static`:

```sh
cd ollantascanner/server/static
npm test
npm run build
```

### 5. Docker rebuilds

- Recreate `serve` after scanner UI or scanner runtime changes:

```sh
docker compose up -d --build --force-recreate serve
```

- Rebuild `ollantaweb` after centralized server changes when validating through Docker:

```sh
docker compose --profile server build ollantaweb
```

- Bring up the full server stack when validating end-to-end behavior:

```sh
docker compose --profile server up -d
```

## Pull Request Checklist

- Run the relevant validation for the area you changed.
- Update docs when behavior, workflows, or configuration changed.
- Call out security implications explicitly when applicable.
- Keep the scope focused.

## Commit Guidance

Use conventional commit prefixes:

- `feat:` for new functionality
- `fix:` for bug fixes
- `docs:` for documentation-only changes
- `test:` for tests
- `chore:` for maintenance work

Recommended branch format:

- `username/brief-description`

## Common Mistakes

- Assuming `make build`, `make test`, `make lint`, or `make fmt` cover every module in the workspace
- Forgetting to rebuild `ollantascanner/server/static/dist/app.js` after frontend changes
- Recreating the browser session without rebuilding the embedded scanner assets
- Running direct CGO-backed commands on Windows without the MSYS2/MinGW toolchain on `PATH`