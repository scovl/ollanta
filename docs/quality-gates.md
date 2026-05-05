# Quality Gates

Quality gates evaluate numeric metrics against configurable conditions after every scan. Each condition defines a metric, an operator, a threshold, and an optional warning threshold. The gate produces a single status: `OK`, `WARN`, or `ERROR`.

Quality Gates are intentionally separate from Quality Profiles:

| Concept | Responsibility | Example |
|---------|----------------|---------|
| Quality Profile | Selects which rules run, their effective severity, and rule parameters. | Enable `go:no-large-functions` with `max_lines=30`; disable `go:todo-comment`. |
| Quality Gate | Evaluates the scan result after analysis using persisted metrics and new-code comparisons. | Fail when `bugs > 0`, `coverage < 80`, or `new_bugs > 5`. |

The scanner enforces the effective Quality Profile before issues are emitted. The server evaluates the Quality Gate during ingest using the gate assigned to the project (from the database), comparing current measures against the configured conditions.

## Gate Lifecycle

1. An admin creates a gate via the **Quality Gate** tab (UI) or `POST /api/v1/quality-gates`.
2. The gate is populated with conditions — metrics, operators, error thresholds, and optional warning thresholds.
3. The gate can be assigned to one or more projects via `POST /api/v1/projects/{key}/quality-gate`.
4. On each push, the ingest pipeline loads the project's assigned gate (or the global default), evaluates all conditions against the current measures and new-code measures, and stores the result in the scan.
5. The scanner receives the gate status and exits accordingly.

**Built-in gate**: The "Ollanta Default" gate comes with 4 conditions: `bugs > 0`, `vulnerabilities > 0`, `new_bugs > 0` (on new code), `new_vulnerabilities > 0` (on new code). New gates are automatically created with these same 4 conditions.

## Gate Status

| Status | Meaning | Scanner behavior |
|--------|---------|-----------------|
| `OK` | All conditions pass | Exits 0 |
| `WARN` | At least one warning threshold violated, but no error threshold violated | Exits 0 (does not block CI) |
| `ERROR` | At least one error threshold violated | Exits 3 |
| `NONE` | No gate is configured for the project | Exits 0 |

## Conditions

Each condition supports:

| Field | Type | Description |
|-------|------|-------------|
| `metric` | string | Metric key to evaluate (e.g. `bugs`, `coverage`, `new_bugs`) |
| `operator` | string | `GT`, `LT`, `GTE`, `LTE`, `EQ`, `NE` |
| `threshold` | number | Error threshold — violation produces ERROR status |
| `warning_threshold` | number (optional) | Warning threshold — violation produces WARN status (only if error threshold is not violated) |
| `on_new_code` | boolean | When `true`, evaluates only new/changed code instead of the full project |

### Warning thresholds

When a condition has both `threshold` (error) and `warning_threshold` set:

- Actual value violates `threshold` → condition status = `ERROR`
- Actual value violates `warning_threshold` but NOT `threshold` → condition status = `WARN`
- Neither violated → `OK`

Gate final status: `ERROR` if any condition is `ERROR`, otherwise `WARN` if any condition is `WARN`, otherwise `OK`.

### Small changeset

When a scan changes fewer lines than `small_changeset_lines` (default 20), conditions on `new_coverage` and `new_duplications` are automatically skipped. This prevents false negatives on small pull requests where coverage and duplication metrics have low statistical significance.

## New-Code Measures

Conditions with `on_new_code: true` compare against measures computed from the difference between the current scan and the baseline scan (determined by the new-code period). For cumulative metrics (bugs, vulnerabilities, code smells): `new = current - baseline`. For relative metrics (coverage, duplication): the current value is used directly.

The scanner CLI displays the gate result on push, including which conditions failed and their actual values vs thresholds.

## Supported Metric Keys

Gate conditions can reference any numeric measure available after scan analysis:

| Metric key | Type | Meaning |
|------------|------|---------|
| `bugs` | cumulative | Reliability findings |
| `vulnerabilities` | cumulative | Security vulnerabilities |
| `code_smells` | cumulative | Maintainability findings |
| `coverage` | relative | Coverage percentage |
| `new_bugs` | cumulative | Bugs introduced since baseline |
| `new_vulnerabilities` | cumulative | Vulnerabilities introduced since baseline |
| `new_code_smells` | cumulative | Code smells introduced since baseline |
| `new_coverage` | relative | Coverage on new/changed code |
| `duplicated_lines_density` | relative | Duplication percentage |
| `new_duplicated_lines_density` | relative | Duplication on new/changed code |
| `tests` | optional | Total automated tests |
| `test_failures` | optional | Failed tests |
| `test_errors` | optional | Test execution errors |
| `test_skipped` | optional | Skipped tests |
| `test_duration_ms` | optional | Test duration in milliseconds |
| `mutation_score` | optional | Mutation score percentage |
| `mutants_total` | optional | Total mutants |
| `mutants_killed` | optional | Killed mutants |
| `mutants_survived` | optional | Survived mutants |
| `mutants_timeout` | optional | Timed-out mutants |
| `mutants_error` | optional | Errored mutants |
| `mutants_skipped` | optional | Skipped mutants |
| `changed_mutation_score` | optional | Mutation score for changed code |
| `changed_mutants_total` | optional | Mutants generated for changed code |
| `changed_mutants_killed` | optional | Killed mutants on changed code |
| `changed_mutants_survived` | optional | Survived mutants on changed code |

Missing optional test or mutation metrics do not fail a gate condition. They are reported as missing values and pass by default. Measured zero is different from unavailable — when evidence exists and the value is zero, the zero-valued metric is persisted so gates can evaluate it.

## Gate Assignment

Each project can have exactly one quality gate assigned. Resolution order:

1. Project-assigned gate (via `POST /api/v1/projects/{key}/quality-gate`)
2. Global default gate (the gate marked `is_default = true`)
3. Built-in "Ollanta Default" gate (always exists)

The gate is evaluated server-side during scan ingest. The result is persisted as a snapshot on the scan record for audit trail.

## UI Management

The **Quality Gate** tab in the project detail page provides:

- **Create gate**: Name only — auto-populated with 4 default conditions
- **Conditions**: Expand each gate to view, add, or remove conditions
- **Assign to project**: One-click association
- **Copy**: Duplicate a gate with all conditions
- **Set as default**: Promote a gate to the global default
- **Delete**: Remove a non-builtin gate

Operators are displayed descriptively: "is greater than", "is less than", "equals", etc. Percentage metrics show the `%` suffix.

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/quality-gates` | List all gates |
| `POST` | `/api/v1/quality-gates` | Create gate (auto-adds 4 conditions) |
| `GET` | `/api/v1/quality-gates/{id}` | Get gate with conditions |
| `PUT` | `/api/v1/quality-gates/{id}` | Update gate |
| `DELETE` | `/api/v1/quality-gates/{id}` | Delete gate |
| `POST` | `/api/v1/quality-gates/{id}/conditions` | Add condition |
| `PUT` | `/api/v1/quality-gates/{id}/conditions/{cid}` | Update condition |
| `DELETE` | `/api/v1/quality-gates/{id}/conditions/{cid}` | Remove condition |
| `POST` | `/api/v1/quality-gates/{id}/copy` | Copy gate with all conditions |
| `POST` | `/api/v1/quality-gates/{id}/set-default` | Set as default |
| `POST` | `/api/v1/projects/{key}/quality-gate` | Assign gate to project |

## Quality Profiles

Profiles define which rules are active for a project. Rules can be activated or deactivated per profile, severity and params can be overridden, and profiles can be imported/exported as JSON or YAML profile-as-code.

Built-in `Ollanta Way` profiles are synchronized at `ollantaweb` startup from a CGo-free rule catalog. This keeps server-side validation available without importing tree-sitter or analyzer packages into the web process. TypeScript and Rust currently expose parser-only profile state because the scanner can parse those languages but does not ship bundled rules yet.

When the scanner runs with `-profile-source server`, it fetches `GET /api/v1/projects/{key}/profiles/effective` and enforces the returned active rules. With `-profile-source local`, it reads a profile-as-code file. With `auto`, it prefers a local file when configured, then server profiles when a server URL is configured, then built-in defaults. Unless `-profile-strict` is set, local/server load failures fall back to built-in defaults and appear as diagnostics in the report.

Profile-as-code example:

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

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/profiles` | List quality profiles |
| `POST` | `/api/v1/profiles` | Create quality profile |
| `GET` | `/api/v1/profiles/{id}` | Get profile |
| `GET` | `/api/v1/profiles/{id}/effective-rules` | Resolved rules after inheritance |
| `GET` | `/api/v1/profiles/{id}/export` | Export a profile-as-code document |
| `GET` | `/api/v1/profiles/{id}/changelog` | Paginated profile change history |
| `POST` | `/api/v1/profiles/{id}/rules` | Activate rule in profile |
| `DELETE` | `/api/v1/profiles/{id}/rules/{ruleKey}` | Deactivate rule |
| `POST` | `/api/v1/profiles/{id}/import` | Import profile from JSON or YAML |
| `GET` | `/api/v1/projects/{key}/profiles` | Project assignment/default profile state |
| `POST` | `/api/v1/projects/{key}/profiles` | Assign a profile to a project language |
| `GET` | `/api/v1/projects/{key}/profiles/effective` | Effective scanner policy for all supported languages |

Every scanner report can include a `quality_profiles` array with `language`, `profile_name`, `source`, `active_rule_count`, `rules_hash`, and diagnostics. The server persists these snapshots during ingest. Legacy reports without this field remain accepted and are marked as profile metadata unavailable.

## New-code Periods

New-code periods allow gates to evaluate only issues introduced since a given date or baseline scan.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/projects/{key}/new-code` | Get new-code period setting |
| `PUT` | `/api/v1/projects/{key}/new-code` | Set new-code period |
