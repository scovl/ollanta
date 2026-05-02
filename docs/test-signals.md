# Test Signal Discovery

Ollanta can optionally discover unit-test modules and existing test or coverage reports during a scanner run. The feature is disabled by default, and the scanner does not execute repository test commands unless execution is explicitly enabled.

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
command_policy = "explicit"

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
owner = "platform"
team = "quality"
```

Explicit `[[tests.modules]]` entries override inferred language and architecture role. Set `test_policy = "ignored"` with `ignore_reason` for generated code, fixtures, vendored packages, or modules owned by another pipeline.

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

The first parser slice normalizes:

- Go cover profiles (`coverage.out`, `cover.out`).
- LCOV (`lcov.info`).
- Cobertura-style XML line coverage.
- JUnit XML test suites.
- Native Ollanta JSON.

Multiple reports are merged per module. Test cases with stable identifiers, or matching suite/class/name values, are deduplicated to avoid double-counting matrix jobs or repeated imports.

## CI And Containers

Use `[[tests.path_mapping]]` when reports contain paths from a CI runner, container, devcontainer, or remote workspace that differ from the scanner project directory. Module `artifact_root` and `report_root` can point collection at CI artifacts while keeping module roots stable.

Ollanta first applies explicit path mappings, then tries project-relative paths, then safe suffix matching. Ambiguous suffix matches and out-of-project paths are reported as diagnostics instead of being silently accepted. Generated, dependency, and cache paths are excluded from file-level coverage.

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

The health evaluator considers module role, coverage, failing tests, stale reports, mutation score when present, and integration requirements. Default expectations are stricter for `domain` and `application` modules, allow lower coverage for `adapter` and `infrastructure`, and require integration evidence for adapter/infrastructure roles unless configuration overrides them.

The scanner report exposes project-level and module-level measures for later quality gates and UI surfaces: test counts, failures, skipped tests, duration, coverage, mutation totals, stale status, health score, status, and recommendations. A future gate can use those fields for policies such as "domain coverage must be at least 85%" or "no required module may be at_risk" without changing the report format again.

Recommendations explain the reason for a module priority, such as missing coverage, stale reports, low coverage for its architecture role, failing tests, missing integration evidence, or a suggested command/report configuration.

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
