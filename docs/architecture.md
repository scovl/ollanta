# Architecture

## Hexagonal Layout

Ollanta follows a **hexagonal (ports & adapters)** layout. The idea: the inner rings know nothing about the outer rings. HTTP, PostgreSQL, Meilisearch — all are plug-in details.

```mermaid
graph TD
    subgraph Outer["🔌 Adapters (outer ring)"]
        HTTP["primary/http\nchi HTTP handlers"]
        PG["secondary/postgres\npgx/v5"]
        SEARCH["secondary/search\nMeilisearch"]
        OAUTH["secondary/oauth\nGitHub · GitLab · Google"]
        WH["secondary/webhook\noutbound dispatcher"]
        PARSER["secondary/parser\nTree-sitter CGo"]
        RULES_BRIDGE["secondary/rules\nrule registry bridge"]
    end

    subgraph App["📦 Application (use cases)"]
        UC["ingest · analysis\nscan orchestration"]
    end

    subgraph Domain["🏛️ Domain (inner core — zero external deps)"]
        D["pure models\nport interfaces\ndomain services"]
    end

    HTTP --> UC
    UC --> D
    PG --> D
    SEARCH --> D
    OAUTH --> D
    WH --> D
    PARSER --> D
    RULES_BRIDGE --> D
```

---

## Module Map

Each Go module has a single responsibility. Arrows mean "depends on".

```mermaid
graph LR
    ollantacore["ollantacore\nshared types\nIssue · Metric · Rule"]

    ollantaparser["ollantaparser\nlanguage parsers\nGo AST · Tree-sitter"]
    ollantarules["ollantarules\nrule definitions\nsensors · thresholds"]
    ollantascanner["ollantascanner\nscan orchestration\nJSON + SARIF output"]
    ollantaengine["ollantaengine\nquality gates\nissue tracking\nmetric aggregation"]
    ollantastore["ollantastore\nPostgreSQL repos\nMeilisearch layer"]
    ollantaweb["ollantaweb\nREST API server\n(Docker entry point)"]
    adapter["adapter/\nHTTP · OAuth · Webhook\nTelemetry · Breaker"]

    ollantaparser --> ollantacore
    ollantarules  --> ollantacore
    ollantarules  --> ollantaparser
    ollantascanner --> ollantarules
    ollantascanner --> ollantaparser
    ollantascanner --> ollantacore
    ollantaengine  --> ollantacore
    ollantastore   --> ollantacore
    adapter --> ollantaengine
    adapter --> ollantastore
    adapter --> ollantascanner
    ollantaweb --> adapter
```

---

## Scan Pipeline

What happens from the moment you run `ollanta -project-dir .` to seeing results:

```mermaid
sequenceDiagram
    participant CLI as ollantascanner CLI
    participant Target as File Discovery
    participant Parser as ollantaparser
    participant Sensor as Rule Sensor
    participant Rules as ollantarules Registry
    participant Engine as ollantaengine
    participant Out as Output (JSON/SARIF)

    CLI->>Target: discover files by language
    Target-->>CLI: file list

    loop for each file
        CLI->>Parser: parse file → AST
        Parser-->>CLI: AST / tree-sitter tree
        CLI->>Sensor: run sensor (GoSensor or TreeSitterSensor)
        Sensor->>Rules: FindByLanguage(lang)
        Rules-->>Sensor: active rules
        Sensor-->>CLI: []Issue
    end

    CLI->>Engine: evaluate quality gate
    Engine-->>CLI: gate status (OK / WARN / ERROR)
    CLI->>Out: write report.json + report.sarif
```

---

## Rule System

Each rule is a struct with three fields:

```mermaid
classDiagram
    class Rule {
        +string MetaKey
        +RuleMeta Meta
        +CheckFunc Check
    }
    class RuleMeta {
        +string Key
        +string Name
        +string Description
        +string Language
        +IssueType Type
        +Severity DefaultSeverity
        +[]string Tags
        +[]ParamDef Params
    }
    class CheckFunc {
        <<function>>
        ctx AnalysisContext
        returns []Issue
    }
    Rule --> RuleMeta
    Rule --> CheckFunc
```

**How rules load at startup** — the `init()` pattern:

```mermaid
sequenceDiagram
    participant Binary as program starts
    participant Init as languages/golang/rules init()
    participant FS as embed.FS (*.json baked in)
    participant MR as MustRegister()
    participant GR as globalRegistry

    Binary->>Init: Go calls init() before main()
    Init->>FS: read all *.json files
    FS-->>Init: raw JSON bytes
    Init->>MR: MustRegister(MetaFS, "*.json", Rule1, Rule2, …)
    MR->>MR: for each Rule: find MetaKey in JSON, bind RuleMeta
    MR->>GR: Registry.Register(rule)
    Note over GR: panics if key missing or duplicate
```

---

## Issue Tracking

After each scan, `ollantaengine/tracking` compares results against the previous baseline:

```mermaid
stateDiagram-v2
    [*] --> new : issue appears for first time
    new --> unchanged : still present in next scan
    unchanged --> unchanged : still present
    unchanged --> closed : no longer found
    closed --> reopened : appears again
    reopened --> unchanged : persists
    reopened --> closed : gone again
```

Issues are matched first by **rule key + line hash** (content-based), then by **file path + line number** as a fallback. This means reformatting code without changing logic won't produce spurious closed/reopened transitions.
