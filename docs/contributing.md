# Contributing

This is the canonical contributor guide for Ollanta.

If you only need the short operational version, see [../CONTRIBUTIONS.md](../CONTRIBUTIONS.md).

## Why There Are Two Contribution Guides

Ollanta keeps two contributor documents on purpose, but they should not duplicate each other:

- `CONTRIBUTIONS.md` is the quick reference from the repository root.
- `docs/contributing.md` is the canonical guide with context, validation notes, and documentation expectations.

If both files start saying the same thing in the same amount of detail, maintenance drift wins. The root file should stay short; this file should stay authoritative.

## Project Shape

Ollanta is a multi-module Go workspace. The repository mixes:

- pure Go modules such as `application/`, `domain/`, `ollantastore/`, and `ollantaweb/`
- scanner-side modules that depend on tree-sitter through CGo
- an embedded TypeScript frontend for the local scanner UI
- Docker-managed services for centralized history, auth, search, and webhook delivery

Because of that, the right validation depends on what you changed.

## Local Setup

Minimum tooling:

- Go 1.21+
- a working C compiler for CGo-backed modules
- Docker and Docker Compose
- Node.js for `ollantascanner/server/static`
- `golangci-lint`

On Windows, the root `Makefile` exports the expected CGO environment for its own targets:

- `CGO_ENABLED=1`
- `C:\msys64\mingw64\bin` prepended to `PATH`

If you run `go` or `golangci-lint` directly against CGO-backed packages outside `make`, export the same values in your shell first.

## Validation By Area

### Scanner-side CGO modules

The root `Makefile` currently validates only these modules:

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

Important: those targets do not cover `application/`, `domain/`, `ollantastore/`, `ollantaweb/`, or `adapter/`.

### Server-side and hexagonal modules

For `application/`, `domain/`, `ollantastore/`, and `ollantaweb/`, run targeted commands from the repository root:

```sh
go build ./application/... ./domain/... ./ollantastore/... ./ollantaweb/...
go test ./application/... ./domain/... ./ollantastore/... ./ollantaweb/...
```

There is no root `make lint` target for these modules today. If you need lint coverage there, run `golangci-lint` only on the module you touched and treat it as module-specific validation rather than part of the root checklist.

### Adapter module

`adapter/` is outside the root `Makefile` as well and shares the CGO-sensitive parser boundary.

After exporting the CGO-capable shell environment, run:

```sh
go build ./adapter/...
go test ./adapter/...
golangci-lint run ./adapter/...
```

### Local scanner frontend

For changes under `ollantascanner/server/static`:

```sh
cd ollantascanner/server/static
npm test
npm run build
```

The scanner UI is compiled from TypeScript into `ollantascanner/server/static/dist/app.js` and then embedded into the scanner binary.

That means:

1. Editing `src/*.ts` is not enough; you must regenerate `dist/app.js`.
2. Docker users must recreate `serve` after that rebuild.
3. A browser refresh alone is not enough if the embedded assets were not rebuilt into the running binary.

### Docker rebuilds

- Recreate `serve` after scanner runtime or scanner UI changes:

```sh
docker compose up -d --build --force-recreate serve
```

- Rebuild `ollantaweb` after centralized server changes when validating through Docker:

```sh
docker compose --profile server build ollantaweb
```

- Bring up the full centralized stack when validating end-to-end behavior:

```sh
docker compose --profile server up -d
```

## AI Fix Workflow

The local scanner UI includes a `Fix with AI` tab in issue details.

Supported local configuration patterns:

- simple OpenAI-compatible setup through `OPENAI_API_KEY` and `OLLANTA_AI_OPENAI_MODEL`
- explicit multi-agent JSON configuration through `OLLANTA_AI_AGENTS`
- local mock development through `OLLANTA_AI_ENABLE_MOCK=1`

The apply step is intentionally guarded: if the target file changed after preview generation, Ollanta rejects the apply request and asks for a fresh preview.

## Documentation Expectations

Update documentation when you change:

- CLI flags
- validation or Docker workflows
- environment variables
- server routes or auth requirements
- scanner UI behavior that users rely on

Relevant docs include:

- [architecture.md](architecture.md)
- [api.md](api.md)
- [quality-gates.md](quality-gates.md)
- [rules.md](rules.md)
- [authentication.md](authentication.md)
- [webhooks.md](webhooks.md)

## Review Expectations

Good contributions in Ollanta usually have these properties:

- they solve the root cause instead of patching symptoms
- they preserve the hexagonal boundaries
- they do not duplicate types or data sources
- they leave the repo with the relevant validation passing for the touched area

If your change affects scanner UX, include the exact commands another contributor can run to verify the behavior.