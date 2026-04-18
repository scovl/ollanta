# Rules Reference

## Go (8 rules)

| Rule key                       | Severity | Description |
|--------------------------------|----------|-------------|
| `go:no-large-functions`        | Major    | Functions exceeding `max_lines` (default: 40) |
| `go:no-naked-returns`          | Critical | Naked `return` in functions with named returns longer than `min_lines` |
| `go:naming-conventions`        | Minor    | Exported names must use MixedCaps per Effective Go |
| `go:cognitive-complexity`      | Critical | Functions with high cognitive complexity |
| `go:too-many-parameters`       | Major    | Functions with too many parameters |
| `go:function-nesting-depth`    | Major    | Code nested too deeply |
| `go:magic-number`              | Minor    | Numeric literals that should be named constants |
| `go:todo-comment`              | Info     | TODO, FIXME, HACK and XXX comments |

## JavaScript (4 rules)

| Rule key                  | Severity | Description |
|---------------------------|----------|-------------|
| `js:no-large-functions`   | Major    | Functions exceeding `max_lines` (default: 40) |
| `js:no-console-log`       | Minor    | `console.log` left in production code |
| `js:eqeqeq`               | Major    | Use `===`/`!==` instead of `==`/`!=` |
| `js:too-many-parameters`  | Major    | Functions with too many parameters |

## Python (5 rules)

| Rule key                       | Severity | Description |
|--------------------------------|----------|-------------|
| `py:no-large-functions`        | Major    | Functions exceeding `max_lines` (default: 40) |
| `py:broad-except`              | Major    | Catching broad exceptions like `Exception` |
| `py:mutable-default-argument`  | Major    | Mutable default argument values (list, dict, set) |
| `py:comparison-to-none`        | Minor    | Use `is`/`is not` for None comparisons |
| `py:too-many-parameters`       | Major    | Functions with too many parameters |

---

## Adding a New Rule

Adding a rule requires exactly two files — no edits to existing files.

**1. Create the rule logic** in the appropriate language package:

```go
// ollantarules/languages/golang/rules/my_rule.go
package rules

import ollantarules "github.com/scovl/ollanta/ollantarules"

var MyRule = ollantarules.Rule{
    MetaKey: "go:my-rule",
    Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
        // analysis logic here
        return nil
    },
}
```

**2. Create the metadata JSON** in the same directory:

```json
// ollantarules/languages/golang/rules/go_my-rule.json
{
  "key": "go:my-rule",
  "name": "My Rule",
  "description": "What this rule detects and why it matters.",
  "language": "go",
  "type": "code_smell",
  "severity": "major",
  "tags": ["design"]
}
```

**3. Register it** by adding the variable to the `init()` call in `embed.go`:

```go
ollantarules.MustRegister(MetaFS, "*.json",
    // existing rules...
    MyRule,
)
```

The JSON is embedded at compile time via `//go:embed *.json`. `MustRegister` panics at startup if the `MetaKey` is missing from the JSON or if the key is duplicated, giving compile-time-equivalent safety.

The JSON schema at `ollantarules/rule-meta.schema.json` provides editor validation and autocomplete for all files matching `ollantarules/languages/**/*.json`.
