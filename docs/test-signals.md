# Test Signal Discovery

Ollanta can optionally discover automated test evidence and existing test, coverage, or mutation reports during a scanner run. The feature is disabled by default, and the scanner does not execute repository test commands unless execution is explicitly enabled.

Mutation testing does not require evidence to be classified as `unit`. It requires an automated test suite capable of killing mutants. Ollanta records the suite kind and confidence so teams can distinguish unit, contract, component, integration, functional, and end-to-end evidence instead of pretending all mutation evidence comes from unit tests.

## Modes

- `collect`: discover modules and collect existing reports only.
- `run`: allow configured module commands to run, then collect reports.
- `doctor`: preview discovery output and diagnostics without executing commands.

Use `ollanta -with-tests -tests-mode=collect` for report-only collection. Use `-tests-mode=run` or `-tests-run=true` only when the project has explicitly approved commands.

## Configuration

```toml
[tests]
enabled = true
mode = "collect"
discover = true
run = false
max_report_age = "24h"
exclusions = ["node_modules/**", "vendor/**", "dist/**", "build/**"]
max_depth = 8
max_candidates = 200
max_report_bytes = 20971520
max_runtime = "10m"
command_policy = "explicit"   # requires explicit module command (see policy table below)
fail_on_timeout = false
allow_external_artifacts = false

[[tests.path_mapping]]
from = "/workspace/project"
to = "."

[[tests.modules]]
name = "domain"
root = "domain"
language = "go"
architecture_role = "domain"
test_policy = "required"
command = "go test ./... -coverprofile=coverage.out"
coverage_reports = ["coverage.out"]
test_reports = ["junit.xml"]
coverage_threshold = 85.0
new_coverage_threshold = 90.0
suite_kind = "unit"
evidence_confidence = "high"
allow_external_artifacts = false
owner = "platform"
team = "quality"
```

Explicit `[[tests.modules]]` entries override inferred language and architecture role. Set `test_policy = "ignored"` with `ignore_reason` for generated code, fixtures, vendored packages, or modules owned by another pipeline.

#### Command policies

| Policy | Allows configured commands | Allows discovered commands |
|--------|---------------------------|---------------------------|
| `explicit` | Only when `command` is non-empty | No |
| `never` | No | No |
| `configured_only` | Yes | No |
| `discovered` | Yes | Yes |
| `trusted_shell` | Yes | Yes |

`explicit` is the safest default: a configured module with an empty `command` field is never executed even when `run = true`. Use `configured_only` to allow commands from `[[tests.modules]]` and `[[mutations.modules]]` without explicitly filling every `command` field.

Supported suite kinds are `unit`, `integration`, `contract`, `component`, `functional`, `e2e`, and `unknown`. Supported confidence values are `high`, `medium`, `low`, and `not_applicable`. Availability is reported as `available`, `unavailable`, `partial`, or `stale` in normalized signals and health output.

## Discovery

The scanner looks for modules from:

- `go.work` and nested `go.mod` files.
- JavaScript package workspaces from `package.json` workspaces and `pnpm-workspace.yaml`.
- Nested `package.json`, `pom.xml`, `build.gradle`, `pyproject.toml`, `pytest.ini`, `tox.ini`, and `Cargo.toml` markers.
- Common monorepo markers such as Nx, Lerna, Turbo, and package workspace files.

Architecture roles are inferred from path names such as `domain`, `application`, `adapter`, `infrastructure`, `api`, `web`, `cmd`, `apps`, `services`, `libs`, and `packages`. Configuration always wins over inference.

## Reports And Freshness

Explicit report paths are checked before automatic report candidates. The scanner records provenance for each readable report candidate, including source mode, file size, age, and whether it is fresh or stale according to `max_report_age`.

Common default candidates include Go coverage profiles, LCOV, Cobertura XML, JaCoCo XML, coverage.py reports, JUnit XML, and common `test-results` paths. Reports larger than `max_report_bytes` are ignored with a diagnostic.

If no default candidate is found, Ollanta performs bounded fallback discovery. It walks module/report roots using `max_depth`, `max_candidates`, `max_report_bytes`, and exclusions, then sorts candidates deterministically before parsing. Dependency and cache directories remain excluded, while common artifact directories such as `build`, `target`, and `coverage` are searched for reports.

The parser layer normalizes:

- Go cover profiles (`coverage.out`, `cover.out`).
- LCOV (`lcov.info`).
- Cobertura-style XML line coverage (`coverage.xml`, `cobertura.xml`, or equivalent Cobertura shapes).
- JUnit XML test suites.
- Native Ollanta JSON.

JaCoCo XML and coverage.py JSON can be discovered as candidates, but they are not normalized as first-class formats yet. Unsupported candidates emit `report_format_unsupported` diagnostics instead of silently producing synthetic zero coverage.

Multiple reports are merged per module. Test cases with stable identifiers, or matching suite/class/name values, are deduplicated to avoid double-counting matrix jobs or repeated imports.

#### Suite kind classification from report paths

When a JUnit report path contains a recognizable directory segment, its suite `kind` is inferred automatically:

| Path contains | Inferred suite kind |
|---------------|-------------------|
| `integration/` | `integration` |
| `contract/` | `contract` |
| `component/` | `component` |
| `functional/` | `functional` |
| `e2e/` or `end-to-end/` | `e2e` |

Explicit `suite_kind` configuration always wins. Generic paths like `build/test-results/junit.xml` remain `unknown`. This lets CI pipelines with separate integration and contract test jobs produce correctly classified suites without manual per-module configuration.

## CI And Containers

Use `[[tests.path_mapping]]` when reports contain paths from a CI runner, container, devcontainer, or remote workspace that differ from the scanner project directory. Module `artifact_root` and `report_root` can point collection at CI artifacts while keeping module roots stable.

Ollanta first applies explicit path mappings, then tries project-relative paths, then safe suffix matching. Path mappings are boundary-aware, so a mapping for `/workspace/app` does not match `/workspace/application`. Ambiguous suffix matches and out-of-project paths are reported as diagnostics instead of being silently accepted. Generated, dependency, and cache paths are excluded from file-level coverage.

Configured report paths must stay inside the project unless `allow_external_artifacts = true` is set globally or for that module. Traversal and normalized paths that escape the project are diagnosed before parsing.

## Monorepos And Multi-Language Projects

Use one `[[tests.modules]]` entry per independently tested unit. A hexagonal Go service plus frontend can be modeled like this:

```toml
[[tests.modules]]
name = "domain"
root = "domain"
language = "go"
architecture_role = "domain"
command = "go test ./... -coverprofile=coverage.out"
coverage_reports = ["coverage.out"]
test_reports = ["junit.xml"]
coverage_threshold = 85.0

[[tests.modules]]
name = "application"
root = "application"
language = "go"
architecture_role = "application"
coverage_reports = ["coverage.out"]

[[tests.modules]]
name = "web-ui"
root = "presentation/web"
language = "typescript"
architecture_role = "web"
coverage_reports = ["coverage/lcov.info"]
test_reports = ["test-results/junit.xml"]

[[tests.modules]]
name = "payments-adapter"
root = "adapter/payments"
language = "go"
architecture_role = "adapter"
integration_required = true
test_reports = ["artifacts/integration-junit.xml"]
```

For CI matrix jobs, point each module at the artifact directory produced by that job:

```toml
[[tests.modules]]
name = "python-worker"
root = "services/worker"
language = "python"
architecture_role = "service"
artifact_root = "artifacts/python-worker"
coverage_reports = ["coverage.xml"]
test_reports = ["junit.xml"]
```

If a module is validated by another pipeline or should not count toward health, set `test_policy = "ignored"` and include an `ignore_reason`.

## Test Health And Quality Gates

Test health is derived from normalized module signals and is separate from existing testability code metrics. Missing optional data is reported as `partial` or `unavailable`; it does not fail scan processing or ingestion.

The health evaluator considers module role, coverage, failing tests, stale reports, mutation score when present, changed-code mutation score when present, suite kind, confidence, and integration requirements. Default expectations are stricter for `domain` and `application` modules, allow lower coverage for `adapter` and `infrastructure`, and require integration evidence for adapter/infrastructure roles unless configuration overrides them.

The project-level health score is **weighted by architecture role**:

| Role | Health weight |
|------|-------------|
| `domain` | 3 |
| `application` | 2 |
| `adapter`, `infrastructure`, `frontend`, `web`, `service`, `library`, `unknown` | 1 |

A failing `domain` module penalizes the project score 3x more than a peripheral `library` module. This prevents low-importance modules from masking critical domain health problems.

Mutation evidence without a unit-test report is valid partial evidence. Unit-test evidence is marked unavailable only when the configured policy or module role expects it and no suite or test report was collected.

The scanner report exposes project-level and module-level measures for later quality gates and UI surfaces: test counts, failures, skipped tests, duration, coverage, mutation totals, stale status, health score, status, and recommendations. A future gate can use those fields for policies such as "domain coverage must be at least 85%" or "no required module may be at_risk" without changing the report format again.

Recommendations explain the reason for a module priority, such as missing coverage, stale reports, low coverage for its architecture role, failing tests, missing integration evidence, or a suggested command/report configuration.

## Mutation Signals

Mutation signals are optional and disabled by default. The scanner first collects existing mutation reports; it does not run mutation tools unless `mode = "run"`, `run = true`, or `-mutations-run=true` is explicitly configured.

Use doctor mode to inspect supported tools, commands, and candidate report paths without executing anything:

```bash
ollanta -with-mutations -mutations-mode=doctor
```

Mutation discovery currently detects native Ollanta mutation reports, Stryker, PIT, mutmut, Cosmic Ray, and Infection markers. Configured `[[mutations.modules]]` entries override discovered modules by root, and ignored or disabled modules are reported as diagnostics without affecting mutation health.

```toml
[mutations]
enabled = true
mode = "collect"
discover = true
run = false
changed_only = true
max_runtime = "10m"
max_mutants = 500
max_report_age = "24h"
command_policy = "explicit"   # requires explicit module command (see policy table in [tests] section)
allow_external_artifacts = false

[[mutations.path_mapping]]
from = "/workspace/project"
to = "."

[[mutations.modules]]
name = "domain"
root = "domain"
language = "go"
architecture_role = "domain"
tool = "native"
native_report_paths = ["ollanta-mutations.json"]
report_paths = ["reports/mutation.json"]
threshold = 75.0
changed_code_threshold = 85.0
suite_kind = "contract"
evidence_confidence = "medium"
mutation_policy = "required"

[[mutations.modules]]
name = "generated fixtures"
root = "testdata/generated"
mutation_policy = "ignored"
ignore_reason = "Generated code is validated elsewhere."
```

Report collection records provenance, size, age, and freshness. Stale or unsupported reports are diagnostics and partial health signals; they do not fail ingestion by default. If mutation execution is enabled, commands run with `max_runtime`; timeout is partial unless `fail_on_timeout = true`.

Command execution records shell mode, command policy, working directory, duration, exit code, timeout status, partial status, and stdout/stderr truncation flags. `command_policy = "explicit"` is the default — it requires a configured module with a non-empty `command` field. Use `configured_only` to allow any configured module command, `discovered` to allow discovered defaults, `trusted_shell` when shell features are intentionally required, and `never` for report-only pipelines.

Mutation statuses are normalized into these internal categories: `killed`, `survived`, `no-coverage`, `timeout`, `skipped`, `equivalent`, `ignored`, `non-viable`, `runtime-error`, and `parser-error`. `no-coverage` is actionable and is not counted as killed. Scores are computed from testable mutants while preserving skipped, ignored, equivalent, non-viable, timeout, and error counts as separate evidence.

Normalized mutation data contributes project measures such as `mutation_score`, `mutants_total`, `mutants_killed`, `mutants_survived`, and changed-code measures such as `changed_mutation_score` and `changed_mutants_survived`. Changed-code mutation is prioritized in health recommendations when available.

### Changed-code mutation scope

When `changed_only = true`, the scanner resolves changed files via `git diff --name-only --diff-filter=ACMR` against the pull request base branch or `HEAD~1` as fallback. These files are passed to the tool-specific command builder, producing concrete scope-limited commands:

| Tool | With `changed_only` (no diff) | With concrete changed files |
|------|-----------------------------|----------------------------|
| **Stryker** | `npx stryker run --mutate` (best_effort) | `npx stryker run --mutate=src/a.ts,src/b.ts` (enforced) |
| **PIT** | `mvn ... mutationCoverage` (advisory) | Same (advisory — no scope flag) |
| **mutmut** | `mutmut run` (best_effort) | Same (enforced) |
| **Cosmic Ray** | `cosmic-ray exec .cosmic-ray.toml` (best_effort) | Same (enforced) |
| **Infection** | `vendor/bin/infection --git-diff-filter=AM` (best_effort) | `vendor/bin/infection --filter=src/App.php --git-diff-filter=AM` (enforced) |

Three enforcement levels, exposed as `changed_only_enforcement` in the mutation summary:

- **`enforced`** — concrete file list was passed to the tool
- **`best_effort`** — tool was asked to limit scope but without a precise file list
- **`advisory`** — no scope restriction was passed

The `max_mutants` option also influences the command: Stryker receives `--concurrency=<value>`, PIT receives `-DmaxMutationsPerClass=<value>`, and Infection receives `--threads=<value>`.

Ollanta parses native mutation JSON plus common Stryker JSON and PIT XML reports. Unsupported tools can use the native JSON shape:

```json
{
  "mutation": {
    "tool": "custom",
    "score": 80,
    "total": 10,
    "killed": 8,
    "survived": 2,
    "changed_total": 4,
    "changed_killed": 3,
    "changed_survived": 1,
    "survived_mutants": [
      {
        "id": "src/app.go:42:conditionals-boundary",
        "status": "survived",
        "mutator": "conditionals-boundary",
        "file": "src/app.go",
        "line": 42,
        "replacement": ">="
      }
    ]
  }
}
```
## Native Ollanta Test Signals

Unsupported or proprietary tools can emit a native Ollanta test-signal JSON file and reference it from `native_reports`:

```toml
[[tests.modules]]
name = "custom-runtime"
root = "services/custom-runtime"
native_reports = ["artifacts/ollanta-tests.json"]
```

Native reports are included as normalized test-signal candidates so teams can bridge custom tooling without waiting for a dedicated parser.

Example payload:

```json
{
	"coverage": {
		"lines_to_cover": 120,
		"covered_lines": 108,
		"coverage": 90
	},
	"suites": [
		{
			"name": "contract",
			"kind": "integration",
			"tests": 12,
			"passed": 12
		}
	],
	"mutation": {
		"score": 76,
		"total": 50,
		"killed": 38,
		"survived": 12
	}
}
```

## Rollout Guidance

Start with report-only collection in CI: generate coverage, JUnit, and mutation reports with the tools the project already trusts, then run Ollanta with `mode = "collect"` and `run = false`.

After diagnostics are clean and path mappings are stable, enable explicit commands only for modules where the working directory, timeout, and command policy are reviewed. Keep `fail_on_timeout = false` while tuning runtimes so scans report partial evidence instead of breaking the pipeline unexpectedly.

Once teams trust the data, add quality gates for measured evidence: begin with changed-code mutation and survived-mutant counts, then raise overall coverage or mutation thresholds module by module.

## Native Schema Reference

### `ollanta-tests.json` (v1)

The native Ollanta test-signal JSON format (version 1). Accepts optional top-level `version`, `coverage`, `suites`, and `mutation` objects. Any field may be omitted. The `version` field declares the schema revision and enables forward-compatible parsing.

```json
{
  "version": 1,
  "coverage": {
    "lines_to_cover": 120,
    "covered_lines": 108,
    "coverage": 90
  },
  "suites": [
    {
      "name": "unit",
      "kind": "unit",
      "tests": 12,
      "passed": 11,
      "failures": 1,
      "errors": 0,
      "skipped": 0,
      "duration_ms": 3400
    }
  ],
  "mutation": {
    "tool": "custom",
    "score": 80,
    "total": 10,
    "killed": 8,
    "survived": 2,
    "no_coverage": 0,
    "timeout": 0,
    "skipped": 0,
    "errors": 0,
    "changed_total": 4,
    "changed_killed": 3,
    "changed_survived": 1
  }
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `coverage.lines_to_cover` | int | no | Total lines tracked |
| `coverage.covered_lines` | int | no | Lines hit at least once |
| `coverage.coverage` | float | no | Coverage percentage (0-100) |
| `suites[].name` | string | yes | Suite identifier |
| `suites[].kind` | string | no | `unit`, `integration`, `contract`, `component`, `functional`, `e2e` |
| `suites[].tests` | int | no | Total test cases |
| `suites[].passed` | int | no | Passed test cases |
| `suites[].failures` | int | no | Failed test cases |
| `suites[].errors` | int | no | Errored test cases |
| `suites[].skipped` | int | no | Skipped test cases |
| `suites[].duration_ms` | int | no | Suite wall-clock in milliseconds |
| `mutation.tool` | string | no | Tool identifier displayed in reports |
| `mutation.score` | float | no | Killed / (Killed + Survived) * 100 |
| `mutation.total` | int | no | Total mutants generated |
| `mutation.killed` | int | no | Mutants killed by tests |
| `mutation.survived` | int | no | Mutants not killed (includes `no_coverage`) |
| `mutation.no_coverage` | int | no | Mutants with no test coverage |
| `mutation.timeout` | int | no | Mutants that timed out during execution |
| `mutation.skipped` | int | no | Mutants skipped by the tool |
| `mutation.errors` | int | no | Mutants with runtime or parser errors |
| `mutation.changed_*` | int/float | no | Subset restricted to changed code (when available) |

### `ollanta-mutations.json` (v1)

The native mutation report format (version 1). Top-level `mutation` object or a flat shape are both accepted. The `version` field is optional but recommended for forward compatibility.

```json
{
  "version": 1,
  "mutation": {
    "tool": "custom",
    "score": 80,
    "total": 10,
    "killed": 8,
    "survived": 2,
    "changed_total": 4,
    "changed_killed": 3,
    "changed_survived": 1,
    "survived_mutants": [
      {
        "id": "src/app.go:42:conditionals-boundary",
        "status": "survived",
        "mutator": "conditionals-boundary",
        "file": "src/app.go",
        "line": 42,
        "replacement": ">=",
        "description": "Replaced > with >="
      }
    ]
  }
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `mutation.tool` | string | no | Tool identifier |
| `mutation.score` | float | no | Killed / (Killed + Survived) * 100 |
| `mutation.total` | int | no | Total mutants |
| `mutation.killed` | int | no | Killed mutants |
| `mutation.survived` | int | no | Survived mutants |
| `mutation.changed_total` | int | no | Mutants in changed code |
| `mutation.changed_killed` | int | no | Killed mutants in changed code |
| `mutation.changed_survived` | int | no | Survived mutants in changed code |
| `mutation.survived_mutants[].id` | string | no | Stable mutant identifier (e.g., `file:line:mutator`) |
| `mutation.survived_mutants[].status` | string | no | Normalized: `survived`, `no-coverage`, `timeout` |
| `mutation.survived_mutants[].mutator` | string | no | Mutation operator name |
| `mutation.survived_mutants[].file` | string | yes | Source file path relative to module root |
| `mutation.survived_mutants[].line` | int | yes | Line number |
| `mutation.survived_mutants[].replacement` | string | no | The replacement that survived |
| `mutation.survived_mutants[].description` | string | no | Human-readable mutation description |

### Query survived mutants

`GET /api/v1/scans/{id}/survived-mutants` returns the survived mutant list extracted from the scan's test signals.

```json
{
  "scan_id": 42,
  "total": 3,
  "mutants": [
    {
      "id": "src/app.go:42:conditionals-boundary",
      "status": "survived",
      "mutator": "conditionals-boundary",
      "file": "src/app.go",
      "line": 42,
      "description": "Replaced > with >=",
      "module": "domain",
      "changed_code": true
    }
  ]
}
```
