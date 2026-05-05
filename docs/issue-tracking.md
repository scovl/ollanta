# Issue Tracking

The tracking service compares current scan results against the previous scan in the same branch or pull request scope to classify each issue.

## Issue States

| State       | Meaning |
|-------------|---------|
| `new`       | Not present in the previous scan |
| `unchanged` | Present in both scans at the same location |
| `closed`    | Present in the previous scan but not in the current one |
| `reopened`  | Was closed in a prior scan but appeared again |
| `unknown`   | Legacy or backfilled state is not available yet |

The current-scan lifecycle is stored on each issue as `tracking_state`. It powers the Issues facet sidebar and can be filtered through `GET /api/v1/issues?tracking_state=...`.

Legacy projects can be backfilled with:

```sh
curl -X POST http://localhost:8080/api/v1/projects/<key>/issues/backfill-tracking-state \
	-H "Authorization: Bearer <token>"
```

## Matching Strategy

Issues are matched across scans using:
1. **Primary**: rule key + line hash (content-based, survives reformatting)
2. **Fallback**: file path + line number (position-based)

This means that reformatting a file without changing its logic will not cause issues to be marked as closed and reopened.

## Quality Facets

Issues are also classified into software quality domains for navigation and reporting:

| Quality domain | Source |
|----------------|--------|
| `security` | `vulnerability` and `security_hotspot` issue types |
| `reliability` | `bug` issue type |
| `maintainability` | `code_smell` issue type |
| `testability` | Test-related tags such as `testability`, `coverage-gap`, `mutation`, `survived-mutant`, `failing-test`, and `flaky-test` |

Security categories are derived from normalized tags such as `owasp-*`, `cwe-*`, `injection`, `auth`, `crypto`, and `secrets`.

## Tag Governance

Tags are now managed as a governed vocabulary instead of only free-form labels on projects, issues, and rules. The catalog stores the tag key, display name, description, color, owner, status, aliases, and usage counts while preserving the existing `tags` arrays used by scans and filters.

Scan ingestion automatically discovers unknown issue tags and records them as `discovered` catalog entries. Built-in rules contribute the tags declared in their rule metadata (for example `readability`, `complexity`, `convention`); custom rules contribute the tags declared on the rule itself. Security category tags such as `owasp-*`, `cwe-*`, `injection`, `auth`, `crypto`, and `secrets` are also discovered so teams can filter and govern them later.

The Tags area in the web UI provides:

- Catalog browsing with color swatches, descriptions, owners, status, and usage.
- Per-tag pages with aliases, audit history, and counts across issues, projects, rules, custom rules, and saved filters.
- Bulk tag edit preview/apply flows for issues, projects, rules, and custom rules.
- Saved issue filters that can use governed tag criteria and resolve aliases when applied.

Deprecated tags cannot be newly assigned. Use aliases or merge operations to keep old tag names resolving to the canonical catalog entry while steering new work to the approved vocabulary.
