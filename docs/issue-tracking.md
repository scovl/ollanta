# Issue Tracking

The `ollantaengine/tracking` package compares current scan results against a previous baseline to classify each issue.

## Issue States

| State       | Meaning |
|-------------|---------|
| `new`       | Not present in the previous scan |
| `unchanged` | Present in both scans at the same location |
| `closed`    | Present in the previous scan but not in the current one |
| `reopened`  | Was closed in a prior scan but appeared again |

## Matching Strategy

Issues are matched across scans using:
1. **Primary**: rule key + line hash (content-based, survives reformatting)
2. **Fallback**: file path + line number (position-based)

This means that reformatting a file without changing its logic will not cause issues to be marked as closed and reopened.
