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

## Custom Rule Packs

Custom Rule Packs are the easiest way to add project or organization-specific checks without writing Go code or rebuilding Ollanta. They are declarative JSON/YAML documents that extend the rule catalog once they are valid and published.

Supported custom engines:

| Engine | Use case | Runtime |
|--------|----------|---------|
| `text` | Regular expression matches over source text | Server validation and scanner execution |
| `go-ast` | Common Go AST patterns such as forbidden calls/imports | Server validation and scanner execution |
| `tree-sitter` | Syntax queries for parser-backed languages | Scanner/local validation and execution |

Example pack:

```yaml
version: 1
pack:
  name: Team Rules
  namespace: team
rules:
  - key: no-debug-print
    name: Debug prints should not be committed
    language: go
    type: code_smell
    severity: major
    engine: go-ast
    engine_config:
      pattern: forbidden_call
      target: fmt.Println
    message: Remove debug printing before committing.
    examples:
      - name: compliant
        code: |
          package main
          func main() { println("ok") }
        compliant: true
      - name: noncompliant
        code: |
          package main
          import "fmt"
          func main() { fmt.Println("debug") }
        compliant: false
```

Local scans load rule packs from `.ollanta/rules/*.yaml`, `.ollanta/rules/*.yml`, and `.ollanta/rules/*.json` before analysis. Server-managed scans fetch the published custom catalog from `ollantaweb` at scan start. A scan uses that immutable snapshot until it finishes, and reports include custom catalog/profile hashes for auditability.

Custom rules still run through Quality Profiles. A published rule must be activated in a compatible profile before the scanner executes it. With local profile-as-code, activate it by key:

```yaml
version: 1
profiles:
  - language: go
    name: Team Go
    rules:
      - key: team:no-debug-print
        severity: critical
```

In the server UI, use Rule Studio to create a draft, validate examples, publish it, and add it to a profile. Rule Studio can also generate an editable draft from an AI model and intent. Local Ollama models are discovered as local/no-key options, while OpenAI, Anthropic Claude, Kimi K2, and Qwen providers are configured server-side. The built-in OpenAI defaults track the GPT-5 family and use the Responses API for OpenAI's own endpoint. Anthropic Claude uses `ANTHROPIC_API_KEY` and defaults to `claude-opus-4-7`, `claude-sonnet-4-6`, and `claude-haiku-4-5`; Kimi uses `MOONSHOT_API_KEY` or `KIMI_API_KEY` and defaults to `kimi-k2.6`; Qwen uses `DASHSCOPE_API_KEY` or `QWEN_API_KEY` and defaults to `qwen3.6-max-preview`. Use each provider's `OLLANTA_AI_<PROVIDER>_MODELS`, `OLLANTA_AI_<PROVIDER>_MODEL`, or `OLLANTA_AI_<PROVIDER>_BASE_URL` variables to override the list or endpoint. AI output is never published automatically; drafts still need example validation and profile activation. Draft, invalid, disabled, and deprecated custom rules are not exposed through scanner/profile catalog endpoints.

To exercise the full lifecycle against a running server stack, run the smoke script with an admin password in the environment:

```powershell
$env:OLLANTA_ADMIN_PASSWORD = '<admin-password>'
.\scripts\smoke-custom-rules.ps1
```

The smoke test creates a custom rule pack, validates, previews, publishes, activates it in a profile, runs the scanner with server profiles, pushes the report, and verifies the issue through the server API.

## Adding a Native Analyzer Rule

Native analyzer rules are compiled into Ollanta. Use this path when a rule needs arbitrary Go logic, deeper semantic analysis, or behavior that cannot be expressed safely as a Custom Rule Pack. Adding one requires the rule logic, its metadata JSON, and registration in the language `embed.go` file.

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

Issue quality facets are derived from `type` and `tags`: `bug` maps to Reliability, `vulnerability` and `security_hotspot` map to Security, `code_smell` maps to Maintainability, and test-oriented tags such as `testability`, `coverage-gap`, `mutation`, or `survived-mutant` map to Testability.

**3. Register it** by adding the variable to the `init()` call in `embed.go`:

```go
ollantarules.MustRegister(MetaFS, "*.json",
    // existing rules...
    MyRule,
)
```

The JSON is embedded at compile time via `//go:embed *.json`. `MustRegister` panics at startup if the `MetaKey` is missing from the JSON or if the key is duplicated, giving compile-time-equivalent safety.

The JSON schema at `ollantarules/rule-meta.schema.json` provides editor validation and autocomplete for all files matching `ollantarules/languages/**/*.json`.
