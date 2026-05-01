# Quality Gates

Quality gates evaluate numeric metrics against configurable thresholds after every scan. Conditions support operators: `gt`, `lt`, `eq`, `gte`, `lte`.

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
| `tests` | Total unit tests. |
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

Missing optional test or mutation metrics do not fail a gate condition. They are reported as missing values and pass by default, which keeps legacy reports compatible.

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

Profiles define which rules are active for a project. Rules can be activated or deactivated per profile, and profiles can be imported from YAML.

| Method | Endpoint                                | Description |
|--------|-----------------------------------------|-------------|
| GET    | `/api/v1/profiles`                      | List quality profiles |
| POST   | `/api/v1/profiles`                      | Create quality profile |
| GET    | `/api/v1/profiles/{id}`                 | Get profile |
| POST   | `/api/v1/profiles/{id}/rules`           | Activate rule in profile |
| DELETE | `/api/v1/profiles/{id}/rules/{ruleKey}` | Deactivate rule |
| POST   | `/api/v1/profiles/{id}/import`          | Import profile from YAML |

## New-code Periods

New-code periods allow gates to evaluate only issues introduced since a given date or baseline scan.

| Method | Endpoint                          | Description |
|--------|-----------------------------------|-------------|
| GET    | `/api/v1/projects/{key}/new-code` | Get new-code period setting |
| PUT    | `/api/v1/projects/{key}/new-code` | Set new-code period |
