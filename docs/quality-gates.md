# Quality Gates

Quality gates evaluate numeric metrics against configurable thresholds after every scan. Conditions support operators: `gt`, `lt`, `eq`, `gte`, `lte`.

Quality Gates are intentionally separate from Quality Profiles:

| Concept | Responsibility | Example |
|---------|----------------|---------|
| Quality Profile | Selects which rules run, their effective severity, and rule parameters. | Enable `go:no-large-functions` with `max_lines=30`; disable `go:todo-comment`. |
| Quality Gate | Evaluates the scan result after analysis using persisted metrics. | Fail when `bugs > 0`, `coverage < 80`, or `changed_mutation_score < 85`. |

The scanner enforces the effective Quality Profile before issues are emitted. The server evaluates the Quality Gate after the report is ingested. A profile can reduce, increase, or change issue severities; a gate decides whether the resulting scan is acceptable.

Gates can be managed via the [API](api.md) or defined in-process:

```go
conditions := []qualitygate.Condition{
    {MetricKey: "bugs",     Operator: qualitygate.OpGreaterThan, ErrorThreshold: 0,  Description: "Zero bugs"},
    {MetricKey: "coverage", Operator: qualitygate.OpLessThan,    ErrorThreshold: 80, Description: "Coverage ≥ 80%"},
}
status := qualitygate.Evaluate(conditions, measures)
if !status.Passed() {
    log.Fatal("Quality gate failed")
}
```

The server persists gates in PostgreSQL. Each scan stores its `gate_status` (`OK` / `WARN` / `ERROR`). The scanner CLI exits with code 1 when the gate fails.

## Supported Metric Keys

Quality gates can evaluate any numeric measure persisted for the scan. Common project-level keys include:

| Metric key | Meaning |
|------------|---------|
| `bugs` | Reliability findings. |
| `vulnerabilities` | Security vulnerabilities. |
| `code_smells` | Maintainability findings. |
| `coverage` | Coverage percentage, when supplied in the report. |
| `tests` | Total automated tests collected from available evidence. |
| `test_failures` | Failed tests. |
| `test_errors` | Test execution errors. |
| `test_skipped` | Skipped tests. |
| `test_duration_ms` | Test duration in milliseconds. |
| `mutation_score` | Mutation score percentage. |
| `mutants_total` | Total mutants. |
| `mutants_killed` | Killed mutants. |
| `mutants_survived` | Survived mutants. |
| `mutants_timeout` | Timed-out mutants. |
| `mutants_error` | Errored mutants. |
| `mutants_skipped` | Skipped mutants. |
| `changed_mutation_score` | Mutation score for changed code. |
| `changed_mutants_total` | Mutants generated for changed code. |
| `changed_mutants_killed` | Killed mutants on changed code. |
| `changed_mutants_survived` | Survived mutants on changed code. |

Missing optional test or mutation metrics do not fail a gate condition. They are reported as missing values and pass by default, which keeps legacy reports compatible.

Measured zero is different from unavailable. When test or mutation evidence is collected and the measured value is zero, Ollanta persists the zero-valued metric so gates can evaluate it. When evidence was not collected at all, Ollanta leaves optional metrics absent instead of creating synthetic zero values.

Example mutation conditions for teams that have already rolled out mutation reports:

```go
conditions := []qualitygate.Condition{
    {MetricKey: "mutation_score", Operator: qualitygate.OpLessThan, ErrorThreshold: 70, Description: "Overall mutation score >= 70%"},
    {MetricKey: "changed_mutation_score", Operator: qualitygate.OpLessThan, ErrorThreshold: 85, Description: "Changed-code mutation score >= 85%"},
    {MetricKey: "changed_mutants_survived", Operator: qualitygate.OpGreaterThan, ErrorThreshold: 0, Description: "No survived mutants on changed code"},
}
```

## API Endpoints

| Method | Endpoint                                 | Description |
|--------|------------------------------------------|-------------|
| GET    | `/api/v1/gates`                          | List quality gates |
| POST   | `/api/v1/gates`                          | Create quality gate |
| GET    | `/api/v1/gates/{id}`                     | Get gate details |
| PUT    | `/api/v1/gates/{id}`                     | Update gate |
| DELETE | `/api/v1/gates/{id}`                     | Delete gate |
| POST   | `/api/v1/gates/{id}/conditions`          | Add condition |
| DELETE | `/api/v1/gates/{id}/conditions/{condID}` | Remove condition |

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

| Method | Endpoint                                | Description |
|--------|-----------------------------------------|-------------|
| GET    | `/api/v1/profiles`                      | List quality profiles |
| POST   | `/api/v1/profiles`                      | Create quality profile |
| GET    | `/api/v1/profiles/{id}`                 | Get profile |
| GET    | `/api/v1/profiles/{id}/effective-rules` | Resolved rules after inheritance |
| GET    | `/api/v1/profiles/{id}/export`          | Export a profile-as-code document |
| GET    | `/api/v1/profiles/{id}/changelog`       | Paginated profile change history |
| POST   | `/api/v1/profiles/{id}/rules`           | Activate rule in profile |
| DELETE | `/api/v1/profiles/{id}/rules/{ruleKey}` | Deactivate rule |
| POST   | `/api/v1/profiles/{id}/import`          | Import profile from JSON or YAML |
| GET    | `/api/v1/projects/{key}/profiles`       | Project assignment/default profile state |
| POST   | `/api/v1/projects/{key}/profiles`       | Assign a profile to a project language |
| GET    | `/api/v1/projects/{key}/profiles/effective` | Effective scanner policy for all supported languages |

Every scanner report can include a `quality_profiles` array with `language`, `profile_name`, `source`, `active_rule_count`, `rules_hash`, and diagnostics. The server persists these snapshots during ingest. Legacy reports without this field remain accepted and are marked as profile metadata unavailable.

## New-code Periods

New-code periods allow gates to evaluate only issues introduced since a given date or baseline scan.

| Method | Endpoint                          | Description |
|--------|-----------------------------------|-------------|
| GET    | `/api/v1/projects/{key}/new-code` | Get new-code period setting |
| PUT    | `/api/v1/projects/{key}/new-code` | Set new-code period |
