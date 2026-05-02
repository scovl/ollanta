# Arquitetura do Ollanta

## O que Ã© o Ollanta?

O Ollanta Ã© uma plataforma de anÃ¡lise de cÃ³digo estÃ¡tico multi-linguagem, projetada para ser rÃ¡pida, extensÃ­vel e fÃ¡cil de usar. Ele lÃª seu cÃ³digo-fonte, aplica um conjunto de regras para detectar problemas (bugs, code smells, vulnerabilidades), e gera relatÃ³rios detalhados. O Ollanta Ã© inspirado em ferramentas como SonarQube, OpenStaticAnalyzer e o Semgrep, mas foi construÃ­do do zero com uma arquitetura moderna e modular.

Ele Ã© capaz de ler seu cÃ³digo, entender sua estrutura, e apontar esses problemas automaticamente sem executar nada, apenas analisando o texto do cÃ³digo (por isso chamo de *anÃ¡lise estÃ¡tica*).

---

## Parte 1: Overview

### Os dois lados do Ollanta

O Ollanta tem duas metades que trabalham juntas:

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "secondaryColor": "#dbeafe",
    "tertiaryColor": "#f0fdf4",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fffbeb",
    "clusterBorder": "#fbbf24"
  }
}}%%
graph LR
    subgraph SCAN ["  ðŸ”  Scanner  "]
        A(["ðŸ“ Seu cÃ³digo"]):::src
        B["ðŸ—‚ï¸ Descobrir\narquivos"]:::step
        C["ðŸŒ³ Parsear\ncada arquivo"]:::step
        D["ðŸ“ Aplicar\nregras"]:::step
        E["ðŸ“„ Gerar\nrelatÃ³rio"]:::step
        A --> B --> C --> D --> E
    end

    E -- "ðŸ“¤ envia relatÃ³rio" --> F

    subgraph SRV ["  ðŸ¢  Servidor  "]
        F["ðŸ“¥ Receber\nrelatÃ³rio"]:::step
        G["ðŸ”„ Comparar com\nscan anterior"]:::step
        H["ðŸš¦ Avaliar\nquality gate"]:::step
        I["ðŸ’¾ Guardar\nno banco"]:::step
        J(["ðŸŒ API"]):::out
        F --> G --> H --> I --> J
    end

    classDef src  fill:#dbeafe,stroke:#3b82f6,stroke-width:2px,rx:12,color:#1e3a5f
    classDef step fill:#fef9c3,stroke:#d97706,stroke-width:2px,color:#1c1917
    classDef out  fill:#d1fae5,stroke:#059669,stroke-width:2px,rx:12,color:#064e3b

    style SCAN fill:#fffbeb,stroke:#fbbf24,stroke-width:2px,stroke-dasharray:6 3,rx:16
    style SRV  fill:#f0f9ff,stroke:#7dd3fc,stroke-width:2px,stroke-dasharray:6 3,rx:16
```

**O Scanner** analisa o seu cÃ³digo-fonte localmente e produz um relatÃ³rio com os problemas encontrados. Pode rodar sozinho no terminal, gerar um arquivo JSON/SARIF, ou abrir uma interface web local para visualizar os resultados.

**O Servidor** recebe relatÃ³rios de mÃºltiplos projetos, armazena o histÃ³rico de scans, rastreia a evoluÃ§Ã£o das issues ao longo do tempo e avalia quality gates.

### Modos de uso na prÃ¡tica

| SituaÃ§Ã£o | Comando | O que acontece |
|----------|---------|----------------|
| "Quero ver os problemas do meu cÃ³digo agora" | `ollanta -project-dir . -local-ui` | Scanner roda, abre UI local na porta 7777 |
| "Quero um relatÃ³rio para o CI" | `ollanta -project-dir . -format sarif` | Scanner gera `.ollanta/report.sarif` |
| "Quero histÃ³rico centralizado" | `ollanta -project-dir . -server http://host:8080` | Scanner envia relatÃ³rio ao servidor |
| "Quero acessar resultados via API" | `curl http://host:8080/api/v1/issues` | Servidor expÃµe dados via REST |

---

## Parte 2: Como o Scanner funciona

Quando vocÃª roda o scanner, acontecem 4 etapas em sequÃªncia. Vamos percorrer cada uma:

### Etapa 1: Descoberta de Arquivos

Primeiro, o scanner precisa saber *quais* arquivos analisar. Ele caminha recursivamente pelo diretÃ³rio do projeto, olha a extensÃ£o de cada arquivo, e decide a linguagem:

```
.go     â†’ Go
.js     â†’ JavaScript
.mjs    â†’ JavaScript
.ts     â†’ TypeScript
.tsx    â†’ TypeScript
.py     â†’ Python
.rs     â†’ Rust
```

Os seguintes diretÃ³rios sÃ£o sempre ignorados, independente de configuraÃ§Ã£o:

```
vendor/    node_modules/    .git/    testdata/    _build/    .ollanta/
```

Para excluir arquivos adicionais, use o flag `-exclusions` com padrÃµes glob separados por vÃ­rgula:

```
ollanta -project-dir . -exclusions "*_test.go,generated/**"
```

> **CÃ³digo relevante:** `application/scan/discovery.go`

---

### Etapa 2: Parsing

CÃ³digo-fonte Ã© sÃ³ texto. Para entender sua estrutura, precisamos transformÃ¡-lo em uma **Ã¡rvore sintÃ¡tica**, uma representaÃ§Ã£o que sabe onde comeÃ§a cada funÃ§Ã£o, cada `if`, cada variÃ¡vel. O Ollanta usa **duas estratÃ©gias de parsing** diferentes:

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px"
  }
}}%%
graph TD
    File(["ðŸ“„ arquivo.go / arquivo.js"]):::src

    File --> Check{{"ðŸ”€ Qual linguagem?"}}:::decision

    Check -->|"Go"| GoParser["ðŸ¹ go/parser\n(stdlib nativa, sem CGo)\nâ†’ ast.File"]:::gonode
    Check -->|"JS Â· TS Â· Python Â· Rust"| TSParser["ðŸŒ³ tree-sitter\n(biblioteca em C, via CGo)\nâ†’ ParsedFile com Tree"]:::tsnode

    GoParser --> Rules(["âœ… Pronto para\naplicar regras"]):::out
    TSParser --> Rules

    classDef src      fill:#dbeafe,stroke:#3b82f6,stroke-width:2px,color:#1e3a5f
    classDef decision fill:#fef3c7,stroke:#f59e0b,stroke-width:2px,color:#92400e
    classDef gonode   fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b
    classDef tsnode   fill:#ede9fe,stroke:#7c3aed,stroke-width:2px,color:#3b0764
    classDef out      fill:#fce7f3,stroke:#db2777,stroke-width:2px,color:#831843
```

**Por que dois parsers?** O Go tem um parser excelente na prÃ³pria standard library (`go/parser`). Para as outras linguagens, usamos o [tree-sitter](https://tree-sitter.github.io/), um parser incremental muito rÃ¡pido que suporta dezenas de linguagens via gramÃ¡ticas plugÃ¡veis.

> **Detalhe tÃ©cnico importante:** O tree-sitter Ã© escrito em C/Rust, entÃ£o o mÃ³dulo `ollantaparser` Ã© o **Ãºnico** que precisa de CGo (compilador C). Todos os outros mÃ³dulos do Ollanta funcionam sem CGo, o que simplifica builds e deploys.

> **CÃ³digo relevante:** `ollantaparser/` (tree-sitter) e `ollantarules/languages/golang/sensor/` (Go nativo)

---

### Etapa 3: ExecuÃ§Ã£o de Regras

Com a Ã¡rvore sintÃ¡tica pronta, o scanner aplica **regras**, cada regra sabe detectar um tipo especÃ­fico de problema. Por exemplo:

- *"Esta funÃ§Ã£o tem mais de 40 linhas"* â†’ regra `go:no-large-functions`
- *"Este `==` deveria ser `===`"* â†’ regra `js:eqeqeq`
- *"EstÃ¡ usando `except Exception:` genÃ©rico"* â†’ regra `py:broad-except`

A execuÃ§Ã£o Ã© **paralela**: o scanner distribui os arquivos por um pool de workers (2Ã— o nÃºmero de CPUs) e cada worker processa um arquivo de cada vez. Se um arquivo causar um panic, o worker se recupera e continua com o prÃ³ximo. Ou seja, um arquivo problemÃ¡tico nÃ£o derruba o scan inteiro.

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fffbeb",
    "clusterBorder": "#fbbf24"
  }
}}%%
graph LR
    subgraph POOL ["  âš™ï¸  Pool de Workers â€” NumCPU Ã— 2  "]
        W1["ðŸ¹ Worker 1\nhandler.go"]:::worker
        W2["ðŸ Worker 2\nutils.py"]:::worker
        W3["ðŸŸ¨ Worker 3\nindex.js"]:::worker
        W4["ðŸ¦€ Worker N\nmain.rs"]:::worker
    end

    W1 --> Agg
    W2 --> Agg
    W3 --> Agg
    W4 --> Agg

    Agg["ðŸ—ƒï¸ Coletor\nde Issues"]:::agg
    Agg --> Report(["ðŸ“‹ RelatÃ³rio\nFinal"]):::out

    classDef worker fill:#dbeafe,stroke:#3b82f6,stroke-width:2px,color:#1e3a5f
    classDef agg    fill:#fef9c3,stroke:#d97706,stroke-width:2px,color:#1c1917
    classDef out    fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b

    style POOL fill:#fffbeb,stroke:#fbbf24,stroke-width:2px,stroke-dasharray:6 3
```

> **CÃ³digo relevante:** `application/scan/executor.go`

### Etapa 4: RelatÃ³rio

ApÃ³s todas as regras rodarem, o scanner consolida tudo em um relatÃ³rio que contÃ©m:

1. **Metadados** â€” chave do projeto, data, duraÃ§Ã£o do scan e escopo de branch ou pull request
2. **MÃ©tricas** â€” arquivos, linhas, bugs, code smells, vulnerabilidades e, opcionalmente, cobertura, testes e mutaÃ§Ã£o
3. **Issues** â€” cada problema encontrado, com arquivo, linha, regra, severidade, mensagem, tags, linguagem e domÃ­nio de qualidade derivado

O relatÃ³rio Ã© salvo em dois formatos:
- **JSON** (`.ollanta/report.json`) â€” para consumo pela API/servidor

Exemplo:

```json
{
  "metadata": {
    "project_key": "MeuProjeto",
    "analysis_date": "2024-07-01T12:00:00Z",
    "scope_type": "branch",
    "branch": "main"
  },
  "measures": {
    "files": 10,
    "lines": 1000,
    "bugs": 3,
    "code_smells": 15,
    "vulnerabilities": 0,
    "coverage": 82.5,
    "tests": 120,
    "test_failures": 0,
    "mutation_score": 74.2,
    "mutants_survived": 8
  },
  "issues": [
    {
      "rule_key": "go:no-large-functions",
      "component_path": "handler.go",
      "line": 42,
      "type": "code_smell",
      "severity": "major",
      "quality_domain": "maintainability",
      "language": "go",
      "tags": ["size", "maintainability"],
      "message": "FunÃ§Ã£o 'handleRequest' tem 120 linhas, excedendo o limite de 40."
    }
  ]
}
```

- **SARIF** (`.ollanta/report.sarif`) â€” formato padrÃ£o da indÃºstria, integra com GitHub, VS Code, etc.

Exemplo:

```json
{
  "version": "2.1.0",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "Ollanta Scanner",
          "rules": [
            {
              "id": "go:no-large-functions",
              "name": "FunÃ§Ã£o muito longa",
              "shortDescription": { "text": "FunÃ§Ã£o tem mais de 40 linhas" },
              "fullDescription": { "text": "FunÃ§Ã£o 'handleRequest' tem 120 linhas, excedendo o limite recomendado." },
              "defaultConfiguration": { "level": "error" }
            },
            ...
          ]
        }
      },
      "results": [
        {
          "ruleId": "go:no-large-functions",
          "message": { "text": "FunÃ§Ã£o 'handleRequest' tem 120 linhas, excedendo o limite de 40." },
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": { "uri": "handler.go" },
                "region": { "startLine": 42 }
              }
            }
          ]
        },
        ...
      ]
    }
  ]
}
```

---

## Parte 3: Como o Servidor funciona

O servidor (`ollantaweb`) Ã© onde a mÃ¡gica de **acompanhamento ao longo do tempo** acontece. Enquanto o scanner Ã© "stateless" (roda e esquece), o servidor mantÃ©m o histÃ³rico completo.

Quando o scanner envia um relatÃ³rio para o servidor via `POST /api/v1/scans`, um pipeline de 8 passos Ã© executado. Cada passo tem timeout individual e estratÃ©gia de erro (abort ou skip):

1. **Registrar o projeto** â€” cria o projeto no banco se ainda nÃ£o existir (abort on fail).
2. **Buscar scan anterior** â€” carrega o Ãºltimo scan do mesmo projeto para tracking (skip on fail).
3. **Comparar issues** â€” aplica o algoritmo de tracking para determinar quais issues sÃ£o novas, quais continuam abertas, quais foram corrigidas e quais reapareceram (abort on fail).
4. **Avaliar quality gate** â€” verifica se o projeto satisfaz todas as condiÃ§Ãµes configuradas (skip on fail).
5. **Inserir scan** â€” persiste o registro do scan com mÃ©tricas e status do gate (abort on fail).
6. **Inserir issues** â€” bulk insert de todas as issues via COPY protocol (abort on fail).
7. **Inserir mÃ©tricas** â€” bulk insert das medidas agregadas (abort on fail).
8. **Indexar para busca** â€” enfileira indexaÃ§Ã£o async no backend de busca (skip on fail).

Webhooks sÃ£o disparados pelo handler HTTP apÃ³s o pipeline retornar, nÃ£o dentro do pipeline em si.

A resposta retorna `gate_status` (OK ou ERROR), contagem de issues novas e fechadas. Vamos aprofundar os passos mais interessantes:

### Como o tracking de issues funcionam

Este Ã© um dos conceitos mais importantes do Ollanta. Sem tracking, cada scan seria independente, vocÃª nÃ£o saberia se um bug Ã© novo ou se jÃ¡ existia antes.

**O problema:** entre dois scans, o cÃ³digo muda. Linhas sÃ£o adicionadas e removidas. Uma issue que estava na linha 42 pode agora estar na linha 47. Como saber que Ã© a *mesma* issue?

**A soluÃ§Ã£o: LineHash.** Para cada issue, o Ollanta calcula o SHA-256 do conteÃºdo da linha onde o problema estÃ¡ (ignorando espaÃ§os). Esse hash Ã© estÃ¡vel, nÃ£o importa se a linha mudou de nÃºmero, o *conteÃºdo* continua o mesmo.

A combinaÃ§Ã£o `(rule_key, line_hash)` funciona como a "impressÃ£o digital" de uma issue.

**O algoritmo de matching opera em 2 camadas:**

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fffbeb",
    "clusterBorder": "#fbbf24"
  }
}}%%
flowchart TD
    CI(["ðŸ“‹ Issue no scan atual\n(rule_key + arquivo + line_hash)"]):::src

    L1{{"ðŸ” Camada 1\nCorrespondÃªncia exata?\n(rule_key + arquivo + line_hash)\nnos issues ABERTOS anteriores"}}:::decision
    L2{{"ðŸ” Camada 2\nCorrespondÃªncia solta?\n(rule_key + line_hash)\nem qualquer arquivo anterior"}}:::decision
    WC{{"â“ Estava FECHADA\nanteriormente?"}}:::decision

    Unchanged(["â™»ï¸ Unchanged\nproblema continua"]):::keep
    Moved(["ðŸ”€ Moved\nmesmo conteÃºdo, local diferente"]):::keep
    Reopened(["ðŸ”„ Reopened\nvoltou!"]):::warn
    New(["ðŸ†• New\nnunca visto antes"]):::new_

    Unmatched(["ðŸ“‹ Issue ABERTA anterior\nsem correspondÃªncia"]):::prev
    Closed(["âœ… Closed\nfoi corrigido!"]):::fixed

    CI --> L1
    L1 -->|"âœ… Sim"| Unchanged
    L1 -->|"âŒ NÃ£o"| L2
    L2 -->|"âœ… Sim"| Moved
    L2 -->|"âŒ NÃ£o"| WC
    WC -->|"Sim"| Reopened
    WC -->|"NÃ£o"| New

    Unmatched --> Closed

    classDef src      fill:#dbeafe,stroke:#3b82f6,stroke-width:2px,color:#1e3a5f
    classDef decision fill:#fef3c7,stroke:#f59e0b,stroke-width:2px,color:#92400e
    classDef keep     fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b
    classDef warn     fill:#fef9c3,stroke:#d97706,stroke-width:2px,color:#1c1917
    classDef new_     fill:#ede9fe,stroke:#7c3aed,stroke-width:2px,color:#3b0764
    classDef prev     fill:#f3f4f6,stroke:#6b7280,stroke-width:2px,color:#1f2937
    classDef fixed    fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b
```

**Exemplo concreto:**

| Scan anterior (issues abertas) | Scan atual | Resultado |
|-------------------------------|------------|-----------|
| `go:cognitive-complexity` em `handler.go` hash `a1b2` | Mesma combinaÃ§Ã£o presente | **Unchanged** â€” problema continua |
| `go:magic-number` em `config.go` hash `c3d4` | CombinaÃ§Ã£o nÃ£o encontrada | **Closed** â€” foi corrigido! |
| â€” | `js:eqeqeq` em `app.js` hash `e5f6` (nova) | **New** â€” problema novo |
| `py:broad-except` em `main.py` hash `g7h8` (estava fechada) | Mesma combinaÃ§Ã£o reaparece | **Reopened** â€” voltou |

> **CÃ³digo relevante:** `ollantaengine/tracking/tracker.go` e `domain/service/tracking.go`

### Como o Quality Gate funciona

O quality gate Ã© um conjunto de condiÃ§Ãµes que o projeto precisa satisfazer. Pense nele como um "semÃ¡foro": se passa, estÃ¡ verde (OK); se nÃ£o passa, estÃ¡ vermelho (ERROR).

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fef9c3",
    "clusterBorder": "#d97706"
  }
}}%%
graph LR
    Measures["ðŸ“Š MÃ©tricas do scan\nbugs: 3\nvulnerabilities: 0\ncode_smells: 15"]:::metrics

    Measures --> Gate

    subgraph Gate ["  ðŸš¦  Quality Gate  "]
        C1["bugs > 0?\n3 > 0 â†’ âŒ FAIL"]:::fail
        C2["vulnerabilities > 0?\n0 > 0 â†’ âœ… PASS"]:::pass
    end

    Gate --> Result{{"â“ Alguma condiÃ§Ã£o\nfalhou?"}}:::decision

    Result -->|"Sim"| ERROR(["ðŸ”´ ERROR\nProjeto nÃ£o passa"]):::err
    Result -->|"NÃ£o"| OK(["ðŸŸ¢ OK\nProjeto aprovado"]):::ok

    classDef metrics  fill:#dbeafe,stroke:#3b82f6,stroke-width:2px,color:#1e3a5f
    classDef fail     fill:#fee2e2,stroke:#ef4444,stroke-width:2px,color:#7f1d1d
    classDef pass     fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b
    classDef decision fill:#fef3c7,stroke:#f59e0b,stroke-width:2px,color:#92400e
    classDef err      fill:#fca5a5,stroke:#dc2626,stroke-width:2px,color:#7f1d1d
    classDef ok       fill:#6ee7b7,stroke:#059669,stroke-width:2px,color:#064e3b

    style Gate fill:#fffbeb,stroke:#fbbf24,stroke-width:2px,stroke-dasharray:6 3
```

**CondiÃ§Ãµes padrÃ£o:**

| MÃ©trica | CondiÃ§Ã£o | Significado |
|---------|----------|-------------|
| `bugs` | > 0 â†’ ERROR | Nenhum bug Ã© tolerado |
| `vulnerabilities` | > 0 â†’ ERROR | Nenhuma vulnerabilidade Ã© tolerada |

VocÃª pode criar gates personalizados com condiÃ§Ãµes adicionais (cobertura mÃ­nima, duplicaÃ§Ã£o mÃ¡xima, etc.) e atÃ© avaliar **apenas o cÃ³digo novo** Ãºtil para equipes que herdam projetos legados e querem garantir que cÃ³digo novo nÃ£o introduza problemas.

> **CÃ³digo relevante:** `ollantaengine/qualitygate/gate.go`

---

## Parte 4: O Sistema de Regras â€” como se adiciona uma regra

O sistema de regras Ã© projetado para ser extensÃ­vel. Adicionar uma nova regra envolve trÃªs coisas:

### 1. A lÃ³gica de detecÃ§Ã£o (Go code)

Cada regra Ã© uma funÃ§Ã£o que recebe um contexto de anÃ¡lise e retorna issues encontradas:

```go
var MagicNumber = ollantarules.Rule{
    MetaKey: "go:magic-number",
    Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
        // Percorre a AST procurando literais numÃ©ricos
        // fora de declaraÃ§Ãµes const/var
        // Se encontrar â†’ cria issue
    },
}
```

### 2. Os metadados (JSON embarcado)

Cada regra tem metadados que descrevem seu nome, severidade, tipo e parÃ¢metros configurÃ¡veis:

```json
{
  "key": "go:magic-number",
  "name": "NÃºmeros mÃ¡gicos devem ser extraÃ­dos para constantes",
  "language": "go",
  "type": "code_smell",
  "severity": "minor",
  "tags": ["readability"],
  "params": []
}
```

### 3. O registro no init()

Na inicializaÃ§Ã£o do programa, todas as regras sÃ£o registradas num registry global:

```go
func init() {
    ollantarules.MustRegister(MetaFS, "*.json",
        MagicNumber, TodoComment, CognitiveComplexity, ...)
}
```

O `MustRegister` lÃª os JSONs de metadata (embarcados no binÃ¡rio via `go:embed`), vincula cada JSON com sua `CheckFunc` pelo `MetaKey`, e registra tudo no registry global. Na hora do scan, o sensor consulta esse registry para saber quais regras rodar.

### Como as regras detectam problemas, os dois sensores

Existem dois "sensores" que sabem executar regras, um para cada tipo de parser:

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fffbeb",
    "clusterBorder": "#fbbf24"
  }
}}%%
graph TB
    subgraph GOSENSOR ["  ðŸ¹  GoSensor (para Go)  "]
        GA(["go/parser.ParseFile()"]):::gosrc
        GAST["ast.File\n(Ã¡rvore Go nativa)"]:::gonode
        GI["ast.Inspect(node)\nPercorre a Ã¡rvore\nprocurando padrÃµes"]:::gonode
        GA --> GAST --> GI
    end

    subgraph TSSENSOR ["  ðŸŒ³  TreeSitterSensor (para JS, TS, Python, Rust)  "]
        TA(["tree-sitter.Parse()"]):::tssrc
        TAST["ParsedFile\n(Ã¡rvore tree-sitter)"]:::tsnode
        TQ["QueryRunner.Run(query)\nExecuta S-expressions\ncontra a Ã¡rvore"]:::tsnode
        TA --> TAST --> TQ
    end

    GI --> Out(["âœ… Issues encontradas"]):::out
    TQ --> Out

    classDef gosrc  fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b
    classDef gonode fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b
    classDef tssrc  fill:#ede9fe,stroke:#7c3aed,stroke-width:2px,color:#3b0764
    classDef tsnode fill:#ede9fe,stroke:#7c3aed,stroke-width:2px,color:#3b0764
    classDef out    fill:#fce7f3,stroke:#db2777,stroke-width:2px,color:#831843

    style GOSENSOR fill:#f0fdf4,stroke:#6ee7b7,stroke-width:2px,stroke-dasharray:6 3
    style TSSENSOR fill:#f5f3ff,stroke:#c4b5fd,stroke-width:2px,stroke-dasharray:6 3
```

**S-expressions** sÃ£o o mecanismo de query do tree-sitter. Funcionam como "CSS selectors para cÃ³digo", isto Ã©, permitem selecionar padrÃµes na Ã¡rvore sintÃ¡tica de forma declarativa. Por exemplo, para encontrar todas as chamadas de `console.log` em JavaScript:

```scheme
;; "Encontre todas as chamadas de console.log"
(call_expression
  function: (member_expression
    property: (property_identifier) @prop
    (#eq? @prop "log")))
```

---

## Parte 5: A OrganizaÃ§Ã£o Interna â€” Arquitetura Hexagonal

AtÃ© aqui explico *o que* o Ollanta faz. Agora vamos entender *como* o cÃ³digo Ã© organizado.

### O problema que a arquitetura resolve

Imagine que amanhÃ£ precisamos trocar o PostgreSQL por MySQL, ou o ZincSearch por Elasticsearch. Se o cÃ³digo de negÃ³cio (tracking de issues, quality gates) estiver misturado com cÃ³digo de banco de dados, a se torna um pesadelo.

A **Arquitetura Hexagonal** resolve isso com uma regra simples: **a camada de negÃ³cio nunca sabe qual banco de dados, API, ou framework estÃ¡ sendo usado**. Ele sÃ³ conhece *interfaces* (ports).

### Os trÃªs anÃ©is

Pense em trÃªs cÃ­rculos concÃªntricos, como uma cebola:

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fffbeb",
    "clusterBorder": "#fbbf24"
  }
}}%%
graph TB
    subgraph EXTERNO ["  ðŸ”µ  Anel EXTERNO â€” Adaptadores  "]
        subgraph MEDIO ["  ðŸŸ¡  Anel MÃ‰DIO â€” AplicaÃ§Ã£o  "]
            subgraph INTERNO ["  ðŸŸ¢  Anel INTERNO â€” DomÃ­nio  "]
                D["ðŸ“¦ Modelos: Issue, Project, Scan, Rule\nðŸ”Œ Ports: IProjectRepo, IScanRepo, IIssueRepo\nâš™ï¸ ServiÃ§os: Track(), Evaluate()\nâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€\nZero deps externas. SÃ³ stdlib do Go."]:::inner
            end
            A["ðŸ—‚ï¸ ScanUseCase: descobre â†’ parseia â†’ analisa â†’ relata\nðŸ“¥ IngestUseCase: persiste â†’ rastreia â†’ avalia â†’ indexa\nâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€\nConhece a ORDEM, nÃ£o o COMO.\nChama interfaces, nunca implementaÃ§Ãµes."]:::middle
        end
        E["ðŸ˜ PostgreSQL (pgx/v5) â†’ IProjectRepo, IScanRepo\nðŸ”Ž ZincSearch (HTTP) â†’ ISearcher, IIndexer\nðŸŒ chi/v5 (HTTP Router) â†’ chama os UseCases\nðŸŒ³ tree-sitter (CGo) â†’ IParser\nðŸ”‘ OAuth (GitHub/GitLab) â†’ IOAuthProvider\nâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€\nCÃ³digo 'sujo': SQL, HTTP, CGo.\nPode ser trocado sem mexer nos anÃ©is internos."]:::outer
    end

    E --> A --> D

    classDef inner  fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b
    classDef middle fill:#fef9c3,stroke:#d97706,stroke-width:2px,color:#1c1917
    classDef outer  fill:#dbeafe,stroke:#3b82f6,stroke-width:2px,color:#1e3a5f

    style INTERNO fill:#f0fdf4,stroke:#6ee7b7,stroke-width:2px,stroke-dasharray:6 3
    style MEDIO   fill:#fefce8,stroke:#fbbf24,stroke-width:2px,stroke-dasharray:6 3
    style EXTERNO fill:#eff6ff,stroke:#93c5fd,stroke-width:2px,stroke-dasharray:6 3
```

**A regra de ouro:** as setas sempre apontam para dentro. O anel externo conhece o mÃ©dio, o mÃ©dio conhece o interno, mas **nunca** o contrÃ¡rio.

### Como isso se mapeia nos mÃ³dulos Go

O Ollanta tem 10 mÃ³dulos Go. Eles se dividem em dois grupos: o **nÃºcleo hexagonal** (novo, onde o cÃ³digo estÃ¡ sendo migrado) e os **mÃ³dulos legados** (funcionais, mas sendo gradualmente absorvidos):

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fffbeb",
    "clusterBorder": "#fbbf24"
  }
}}%%
graph TB
    subgraph NOVO ["  ðŸ†•  NÃºcleo Hexagonal (futuro)  "]
        domain["ðŸŸ¢ domain/\nModelos + Ports + ServiÃ§os puros\nZero dependÃªncias externas"]:::inner
        application["ðŸŸ¡ application/\nCasos de uso\nDepende apenas de domain/"]:::middle
        adapter["ðŸ”µ adapter/\nHTTP, OAuth, Postgres, Parser\nImplementa os ports"]:::outer
        domain --> application --> adapter
    end

    subgraph LEGADO ["  ðŸ—„ï¸  MÃ³dulos Legados (funcional, sendo migrado)  "]
        ollantacore["ollantacore/\nTipos compartilhados"]:::leg
        ollantaparser["ollantaparser/\nTree-sitter (CGo)"]:::leg
        ollantarules["ollantarules/\nRegras e sensores"]:::leg
        ollantascanner["ollantascanner/\nCLI e orquestraÃ§Ã£o"]:::leg
        ollantaengine["ollantaengine/\nQuality gates, tracking"]:::leg
        ollantastore["ollantastore/\nRepos PostgreSQL, busca"]:::leg
        ollantaweb["ollantaweb/\nServidor REST"]:::leg
    end

    ollantacore --> ollantarules
    ollantacore --> ollantascanner
    ollantacore --> ollantaengine
    ollantaparser --> ollantarules
    ollantarules --> ollantascanner
    ollantaengine --> ollantaweb
    ollantastore --> ollantaweb

    classDef inner  fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b
    classDef middle fill:#fef9c3,stroke:#d97706,stroke-width:2px,color:#1c1917
    classDef outer  fill:#dbeafe,stroke:#3b82f6,stroke-width:2px,color:#1e3a5f
    classDef leg    fill:#f3f4f6,stroke:#6b7280,stroke-width:2px,color:#1f2937

    style NOVO    fill:#f0fdf4,stroke:#6ee7b7,stroke-width:2px,stroke-dasharray:6 3
    style LEGADO  fill:#f9fafb,stroke:#d1d5db,stroke-width:2px,stroke-dasharray:6 3
```

| MÃ³dulo | Anel | CGo? | O que faz |
|--------|------|------|-----------|
| `domain` | ðŸŸ¢ Interno | NÃ£o | Modelos puros, interfaces de port, serviÃ§os sem I/O |
| `application` | ðŸŸ¡ MÃ©dio | NÃ£o | Orquestra casos de uso chamando ports |
| `adapter` | ðŸ”µ Externo | Sim* | Implementa ports com tecnologias concretas |
| `ollantacore` | Legado | NÃ£o | Tipos compartilhados (`Issue`, `Rule`, `Component`) |
| `ollantaparser` | Legado | **Sim** | Ãšnico mÃ³dulo com CGo (tree-sitter) |
| `ollantarules` | Legado | Sim* | Registry de regras + sensores Go/tree-sitter |
| `ollantascanner` | Legado | Sim* | CLI, descoberta de arquivos, execuÃ§Ã£o paralela |
| `ollantaengine` | Legado | NÃ£o | Quality gates, tracking, new-code periods |
| `ollantastore` | Legado | NÃ£o | PostgreSQL (pgx/v5), ZincSearch, Postgres FTS |
| `ollantaweb` | Legado | NÃ£o | Servidor HTTP, ingestÃ£o, auth, webhooks |

_*CGo via transitividade de `ollantaparser`._

### Os principais ports (interfaces)

Ports sÃ£o as "tomadas" que conectam o domÃ­nio ao mundo externo. Aqui estÃ£o os mais importantes:

```go
// "Onde guardar e buscar projetos?"
IProjectRepo { Upsert, Create, GetByKey, GetByID, List, Delete }

// "Onde guardar e buscar scans?"
IScanRepo { Create, Update, GetByID, GetLatest, ListByProject }

// "Onde guardar e buscar issues?"
IIssueRepo { BulkInsert, Query, Facets, CountByProject, Transition }

// "Como buscar texto livre?"
ISearcher { SearchIssues, SearchProjects }
IIndexer  { IndexIssues, IndexProject, ConfigureIndexes, ReindexAll }

// "Como analisar cÃ³digo?"
IAnalyzer { Key, Name, Language, Check(ctx) }

// "Como autenticar com serviÃ§os externos?"
IOAuthProvider { AuthURL, Exchange }
```

O domÃ­nio sÃ³ conhece essas interfaces. Quem as implementa (PostgreSQL? MongoDB? ZincSearch? Elasticsearch?) Ã© decidido no anel externo.

---

## Parte 6: Conceitos AvanÃ§ados do Engine

### New Code Period â€” "o que Ã© cÃ³digo novo?"

Quando uma equipe herda um projeto legado com 500 issues, nÃ£o faz sentido exigir que todas sejam corrigidas de uma vez. O conceito de **new code period** permite focar apenas no cÃ³digo novo: "a partir de quando estamos medindo?". O Ollanta suporta 5 estratÃ©gias para definir esse baseline:

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fffbeb",
    "clusterBorder": "#fbbf24"
  }
}}%%
graph LR
    Strategy{{"ðŸ“… Qual baseline\nusar?"}}:::decision

    Strategy --> Auto
    Strategy --> PV
    Strategy --> Days
    Strategy --> Specific
    Strategy --> Branch

    Auto["ðŸ¤– auto\nDetecta tags semver (v1.2.3)\nSe nÃ£o encontrar: Ãºltimos 30 dias"]:::opt
    PV["ðŸ“¦ previous_version\nO penÃºltimo scan do projeto"]:::opt
    Days["ðŸ—“ï¸ number_of_days\nScans nos Ãºltimos N dias\n(padrÃ£o: 30)"]:::opt
    Specific["ðŸŽ¯ specific_analysis\nUm scan ID exato"]:::opt
    Branch["ðŸŒ¿ reference_branch\nÃšltimo scan de uma branch\nespecÃ­fica (ex: main)"]:::opt

    classDef decision fill:#fef3c7,stroke:#f59e0b,stroke-width:2px,color:#92400e
    classDef opt      fill:#fef9c3,stroke:#d97706,stroke-width:2px,color:#1c1917
```

> **CÃ³digo relevante:** `ollantaengine/newcode/resolver.go`

### Summarizer â€” mÃ©tricas de baixo para cima

O Ollanta organiza o projeto em uma **Ã¡rvore de componentes**: o projeto contÃ©m mÃ³dulos, que contÃªm pacotes, que contÃªm arquivos. MÃ©tricas sÃ£o calculadas nos arquivos (folhas), mas precisamos saber o total do projeto. O **summarizer** propaga mÃ©tricas de baixo para cima:

```mermaid
graph TB
    subgraph "Antes da propagaÃ§Ã£o"
        P1["ðŸ“ Projeto â€” bugs: ???"]
        P1 --> M1["ðŸ“‚ src/ â€” bugs: ???"]
        P1 --> M2["ðŸ“‚ lib/ â€” bugs: ???"]
        M1 --> F1["ðŸ“„ handler.go â€” bugs: 2"]
        M1 --> F2["ðŸ“„ utils.go â€” bugs: 1"]
        M2 --> F3["ðŸ“„ parser.py â€” bugs: 3"]
    end

    subgraph "Depois do CumSum"
        P2["ðŸ“ Projeto â€” bugs: 6 âœ“"]
        P2 --> M3["ðŸ“‚ src/ â€” bugs: 3"]
        P2 --> M4["ðŸ“‚ lib/ â€” bugs: 3"]
        M3 --> F4["ðŸ“„ handler.go â€” bugs: 2"]
        M3 --> F5["ðŸ“„ utils.go â€” bugs: 1"]
        M4 --> F6["ðŸ“„ parser.py â€” bugs: 3"]
    end

    style P2 fill:#264653,color:#fff
    style M3 fill:#2a9d8f,color:#fff
    style M4 fill:#2a9d8f,color:#fff
```

Dois algoritmos:
- **CumSum** â€” soma: o total de bugs do projeto Ã© a soma dos bugs de todos os arquivos
- **CumAvg** â€” mÃ©dia ponderada: a complexidade mÃ©dia do projeto leva em conta o tamanho de cada arquivo

> **CÃ³digo relevante:** `ollantaengine/summarizer/cumsum.go`

---

## Parte 7: PersistÃªncia e Busca

### PostgreSQL â€” o banco principal

Todas as informaÃ§Ãµes do Ollanta sÃ£o guardadas no PostgreSQL 17. Aqui estÃ¡ o modelo de dados simplificado:

```mermaid
erDiagram
    projects ||--o{ scans : "tem muitos"
    scans ||--o{ issues : "contÃ©m"
    scans ||--o{ measures : "contÃ©m"
    projects ||--o{ webhooks : "configura"
    users ||--o{ tokens : "possui"
    users }o--o{ groups : "pertence a"
    quality_gates ||--o{ gate_conditions : "avalia"

    projects {
        bigserial id PK
        text key "ex: myapp (Ãºnico)"
        text name
        text description
    }

    scans {
        bigserial id PK
        bigint project_id FK
        text version "ex: 1.2.3"
        text branch "ex: main"
        timestamptz analysis_date
        int elapsed_ms
    }

    issues {
        bigserial id PK
        bigint scan_id FK
        text rule_key "ex: go:cognitive-complexity"
        text engine_id "ex: ollanta-scanner"
        text component_path "ex: pkg/handler.go"
        int line
        text message
        text type "bug, code_smell, ..."
        text severity "blocker, critical, ..."
        text status "open, closed, ..."
        text line_hash "SHA-256 da linha"
        jsonb secondary_locations
    }

    measures {
        bigserial id PK
        bigint scan_id FK
        text metric_key "ex: bugs, ncloc"
        float8 value
    }

    users {
        bigserial id PK
        text login "Ãºnico"
        text email "Ãºnico"
        text password_hash "bcrypt"
        text provider "github, gitlab, ..."
    }
```

**OtimizaÃ§Ãµes que valem saber:**

| TÃ©cnica | Onde | Por quÃª |
|---------|------|---------|
| **Tabela particionada** | `issues` (por `created_at`) | Scans antigos podem ser limpos sem reindexar tudo. Queries em scans recentes sÃ£o rÃ¡pidas |
| **COPY protocol** | InserÃ§Ã£o de issues e measures | AtÃ© 50Ã— mais rÃ¡pido que `INSERT` para milhares de linhas |
| **Pool de conexÃµes** | pgx pool (max 25, idle 5min) | Reutiliza conexÃµes TCP ao banco |
| **Advisory locks** | CoordenaÃ§Ã£o de indexaÃ§Ã£o | Evita que duas rÃ©plicas indexem o mesmo scan |

### Busca full-text â€” duas opÃ§Ãµes

O Ollanta precisa buscar issues por texto livre ("todas as issues com 'null pointer'"). Para isso, oferece dois backends intercambiÃ¡veis:

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fffbeb",
    "clusterBorder": "#fbbf24"
  }
}}%%
graph TB
    Config{{"âš™ï¸ OLLANTA_SEARCH_BACKEND\n= ?"}}:::decision

    Config -->|"zincsearch (padrÃ£o)"| Zinc["ðŸ”Ž ZincSearch\nAPI compatÃ­vel com Elasticsearch\nHTTP + Basic Auth\nMelhor para volumes grandes"]:::zinc
    Config -->|"postgres"| PGFTS["ðŸ˜ Postgres Full-Text Search\ntsvector + ts_rank + GIN index\nZero infraestrutura extra\nBom para deploys simples"]:::pg

    Zinc --> Port(["ðŸ”Œ ISearcher + IIndexer\n(mesma interface)"]):::out
    PGFTS --> Port

    classDef decision fill:#fef3c7,stroke:#f59e0b,stroke-width:2px,color:#92400e
    classDef zinc     fill:#ede9fe,stroke:#7c3aed,stroke-width:2px,color:#3b0764
    classDef pg       fill:#dbeafe,stroke:#3b82f6,stroke-width:2px,color:#1e3a5f
    classDef out      fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b
```

A troca entre backends Ã© uma variÃ¡vel de ambiente. O cÃ³digo de negÃ³cio nÃ£o sabe (nem precisa saber) qual estÃ¡ sendo usado.

> **CÃ³digo relevante:** `ollantastore/search/port.go`, `ollantastore/search/factory.go`

### CoordenaÃ§Ã£o de indexaÃ§Ã£o

ApÃ³s cada ingestÃ£o, o `ollantaweb` enfileira um job de indexaÃ§Ã£o em um worker in-process anexado Ã  rÃ©plica em execuÃ§Ã£o. A mesma rÃ©plica que recebeu o scan drena a fila, lÃª as issues no PostgreSQL e atualiza o backend de busca ativo.

| Worker | Como funciona | Quando usar |
|--------|---------------|-------------|
| **memory** | Canal Go com buffer + worker em background com retry | Todas as implantaÃ§Ãµes atuais |

No desenho atual, o fluxo de indexaÃ§Ã£o Ã©:
1. A API persiste scan, issues e mÃ©tricas no PostgreSQL
2. O pipeline de ingestÃ£o enfileira um job local na rÃ©plica que recebeu a requisiÃ§Ã£o
3. O worker in-process lÃª as issues do scan e atualiza o Ã­ndice de busca

Isso reduz a complexidade operacional: nÃ£o hÃ¡ coordenador extra, tabela distribuÃ­da de jobs ou caminho com PostgreSQL `LISTEN/NOTIFY`. Se um pod for interrompido antes de drenar sua fila, execute `POST /admin/reindex` para reconstruir o Ã­ndice a partir do PostgreSQL.

> **CÃ³digo relevante:** `ollantaweb/ingest/worker.go`

---

## Parte 8: AutenticaÃ§Ã£o e AutorizaÃ§Ã£o

### Quatro formas de se autenticar

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fffbeb",
    "clusterBorder": "#fbbf24"
  }
}}%%
graph TB
    subgraph PWD ["  ðŸ”‘  1. Login com senha  "]
        L1(["POST /api/v1/auth/login"]):::req
        L2["ðŸ” bcrypt(password, hash)"]:::step
        L3(["âœ… JWT 15min\n+ Refresh Token 30 dias"]):::ok
        L1 --> L2 --> L3
    end

    subgraph OAUTH ["  ðŸŒ  2. OAuth â€” GitHub Â· GitLab Â· Google  "]
        O1(["GET /api/v1/auth/github"]):::req
        O2["â†ª Redirect â†’ GitHub authorize"]:::step
        O3["Callback com code"]:::step
        O4["Troca code por access_token"]:::step
        O5["Busca perfil do usuÃ¡rio"]:::step
        O6(["âœ… Cria/atualiza usuÃ¡rio\n+ retorna JWT"]):::ok
        O1 --> O2 --> O3 --> O4 --> O5 --> O6
    end

    subgraph TOKEN ["  ðŸª™  3. API Token  "]
        T1(["Authorization: Bearer olt_abc123â€¦"]):::req
        T2["ðŸ” Busca hash do token no banco"]:::step
        T3(["âœ… Identifica o usuÃ¡rio"]):::ok
        T1 --> T2 --> T3
    end

    subgraph SCANNER ["  ðŸ”§  4. Scanner Token (prÃ©-compartilhado)  "]
        S1(["Authorization: Bearer <scanner-token>"]):::req
        S2["ðŸ”‘ Token == OLLANTA_SCANNER_TOKEN?"]:::step
        S3(["âœ… Aceita como usuÃ¡rio 'scanner'
Sem consulta ao banco"]):::ok
        S1 --> S2 --> S3
    end

    classDef req  fill:#dbeafe,stroke:#3b82f6,stroke-width:2px,color:#1e3a5f
    classDef step fill:#fef9c3,stroke:#d97706,stroke-width:2px,color:#1c1917
    classDef ok   fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b

    style PWD     fill:#fffbeb,stroke:#fbbf24,stroke-width:2px,stroke-dasharray:6 3
    style OAUTH   fill:#eff6ff,stroke:#93c5fd,stroke-width:2px,stroke-dasharray:6 3
    style TOKEN   fill:#f5f3ff,stroke:#c4b5fd,stroke-width:2px,stroke-dasharray:6 3
    style SCANNER fill:#f0fdf4,stroke:#6ee7b7,stroke-width:2px,stroke-dasharray:6 3

    PWD ~~~ OAUTH
    OAUTH ~~~ TOKEN
    TOKEN ~~~ SCANNER
```

| Tipo de token | Prefixo | DuraÃ§Ã£o | Uso tÃ­pico |
|---------------|---------|---------|------------|
| **Access Token** | (JWT) | 15 minutos | NavegaÃ§Ã£o na UI, chamadas de API |
| **Refresh Token** | `ort_` | 30 dias | Renovar o access token sem re-login |
| **API Token** | `olt_` | Sem expiraÃ§Ã£o | AutomaÃ§Ã£o, CI/CD, scanner |
| **Scanner Token** | *(configurÃ¡vel)* | Sem expiraÃ§Ã£o | Push de relatÃ³rios sem conta de usuÃ¡rio |

**Scanner Token (prÃ©-compartilhado):** O Ollanta aceita um token prÃ©-compartilhado para o push de scans via CI/CD, evitando a necessidade de criar uma conta de usuÃ¡rio. Configure `OLLANTA_SCANNER_TOKEN` no servidor e `OLLANTA_TOKEN` no scanner com o mesmo valor. Se o Bearer token coincidir, a requisiÃ§Ã£o Ã© aceita como um usuÃ¡rio sintÃ©tico `scanner` â€” sem consulta ao banco.

### PermissÃµes

O Ollanta tem dois nÃ­veis de permissÃµes:

**Globais** (valem para tudo):
- `admin` â€” pode tudo
- `manage_users` â€” criar/editar/deletar usuÃ¡rios
- `manage_groups` â€” gerenciar grupos

**Por projeto** (valem para um projeto especÃ­fico):
- `project_admin` â€” configurar gates, profiles, webhooks
- `can_scan` â€” enviar relatÃ³rios de scan
- `can_view` â€” ver resultados
- `can_comment` â€” transicionar issues (confirmar, fechar, reabrir)

PermissÃµes podem ser atribuÃ­das diretamente a usuÃ¡rios ou a grupos.

---

## Parte 9: Infraestrutura e Deploy

### Docker Compose â€” ambiente completo

```mermaid
graph TB
    subgraph "Perfil: scanner (UI local)"
      localUI["ðŸ” local-ui
        Scanner + UI local
        Porta 7777
        Monta seu cÃ³digo como volume"]
    end

    subgraph "Perfil: server (stack completo)"
        postgres["ðŸ˜ PostgreSQL 17
        Porta 5432
        Volume persistente"]

        zinc["ðŸ”Ž ZincSearch
        Porta 4080
        Busca full-text"]

        web["ðŸŒ ollantaweb
        Porta 8080
        API REST + Frontend"]

        web --> postgres
        web --> zinc
    end

    subgraph "Perfil: push (scanner â†’ servidor)"
        push["ðŸ“¤ push
        Escaneia e envia
        para ollantaweb"]
        push -->|"POST report"| web
    end

    style postgres fill:#336791,color:#fff
    style zinc fill:#e9c46a,color:#000
    style web fill:#264653,color:#fff
    style localUI fill:#2a9d8f,color:#fff
```

**Comandos prÃ¡ticos:**

```bash
# SÃ³ quer escanear e ver os resultados localmente?
docker compose --profile scanner up local-ui

# Quer o servidor completo com banco, busca e API?
docker compose --profile server up -d

# Quer escanear e enviar pro servidor?
PROJECT_DIR=/path/to/code docker compose run --rm push
```

### VariÃ¡veis de ambiente

As mais importantes para configurar o servidor:

| VariÃ¡vel | Default | O que faz |
|----------|---------|-----------|
| `OLLANTA_DATABASE_URL` | *(obrigatÃ³ria)* | Connection string do PostgreSQL |
| `OLLANTA_ADDR` | `:8080` | EndereÃ§o onde o servidor escuta |
| `OLLANTA_SEARCH_BACKEND` | `zincsearch` | Backend de busca (`zincsearch` ou `postgres`) |
| `OLLANTA_SCANNER_TOKEN` | *(vazio)* | Token compartilhado aceito para pushes do scanner em `POST /api/v1/scans` |
| `OLLANTA_JWT_SECRET` | *(auto-gerado)* | Segredo para assinar JWTs |
| `OLLANTA_JWT_EXPIRY` | `15m` | DuraÃ§Ã£o do access token |
| `OLLANTA_ZINCSEARCH_URL` | `http://localhost:4080` | URL do ZincSearch |
| `OLLANTA_LOG_LEVEL` | `info` | NÃ­vel de log |
| `OLLANTA_SCANNER_TOKEN` | *(vazio)* | Token prÃ©-compartilhado para push do scanner. Se vazio, push requer JWT ou API token |

OAuth (opcional â€” configure para habilitar login social):

| VariÃ¡vel | Para quÃª |
|----------|----------|
| `OLLANTA_GITHUB_CLIENT_ID` / `SECRET` | Login via GitHub |
| `OLLANTA_GITLAB_CLIENT_ID` / `SECRET` | Login via GitLab |
| `OLLANTA_GOOGLE_CLIENT_ID` / `SECRET` | Login via Google |
| `OLLANTA_OAUTH_REDIRECT_BASE` | URL base para callbacks (ex: `https://ollanta.exemplo.com`) |

### Health checks

| Endpoint | O que verifica | Quando retorna 200 |
|----------|---------------|-------------------|
| `GET /healthz` | O processo estÃ¡ vivo? | Sempre (liveness) |
| `GET /readyz` | Postgres e busca estÃ£o acessÃ­veis? | Quando tudo estÃ¡ pronto (readiness) |
| `GET /metrics` | MÃ©tricas Prometheus | Sempre |

### Build multi-stage (Docker)

O Ollanta usa build em dois estÃ¡gios para manter a imagem final pequena e segura:

```mermaid
flowchart LR
    subgraph "EstÃ¡gio 1: CompilaÃ§Ã£o"
        A["golang:1.21-bookworm
        (imagem grande com compilador)"] --> B["Compila o binÃ¡rio
        CGO_ENABLED=1
        static linking"]
    end

    subgraph "EstÃ¡gio 2: ProduÃ§Ã£o"
        C["distroless/static
        (imagem mÃ­nima, ~2MB)"] --> D["Copia apenas o binÃ¡rio
        Roda como nonroot"]
    end

    B -->|"binÃ¡rio estÃ¡tico"| C

    style A fill:#264653,color:#fff
    style C fill:#2d6a4f,color:#fff
```

Resultado: imagem final de ~20MB, sem shell, sem ferramentas â€” superfÃ­cie de ataque mÃ­nima.

---

## Parte 10: CI/CD

O pipeline roda no GitHub Actions com 5 jobs paralelos:

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "primaryColor": "#fef9c3",
    "primaryTextColor": "#1c1917",
    "primaryBorderColor": "#d97706",
    "lineColor": "#92400e",
    "edgeLabelBackground": "#fffbeb",
    "fontFamily": "ui-monospace, monospace",
    "fontSize": "14px",
    "clusterBkg": "#fffbeb",
    "clusterBorder": "#fbbf24"
  }
}}%%
flowchart TB
    Push(["ðŸ“¬ Push ou PR\npara main"]):::src

    Push --> L & TS & TW & TA & DB

    L["ðŸ” Lint\ngolangci-lint v2\n(5 mÃ³dulos)"]:::job
    TS["ðŸ§ª Test Scanner\nCGO_ENABLED=1\nollantacore, parser,\nrules, scanner, engine"]:::cgo
    TW["ðŸ§ª Test Web\nCGO_ENABLED=0\nollantastore, ollantaweb"]:::job
    TA["ðŸ§ª Test Adapter\nCGO_ENABLED=1\nadapter/"]:::cgo
    DB["ðŸ³ Docker Build\nSmoke test dos\nDockerfiles"]:::docker

    classDef src    fill:#dbeafe,stroke:#3b82f6,stroke-width:2px,color:#1e3a5f
    classDef job    fill:#fef9c3,stroke:#d97706,stroke-width:2px,color:#1c1917
    classDef cgo    fill:#ede9fe,stroke:#7c3aed,stroke-width:2px,color:#3b0764
    classDef docker fill:#d1fae5,stroke:#059669,stroke-width:2px,color:#064e3b
```

**Por que separar os testes?** MÃ³dulos com CGo (`ollantaparser` e dependentes) precisam de um compilador C instalado. MÃ³dulos sem CGo (`ollantaweb`, `ollantastore`) sÃ£o mais rÃ¡pidos de compilar e testar. Separar permite paralelizar e falhar rÃ¡pido.

**Linters ativos:** errcheck, staticcheck, govet, ineffassign, misspell, revive

---

## Parte 11: API REST â€” ReferÃªncia RÃ¡pida

### Endpoints pÃºblicos (sem autenticaÃ§Ã£o)

| MÃ©todo | Rota | DescriÃ§Ã£o |
|--------|------|-----------|
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/readyz` | Readiness probe |
| `GET` | `/metrics` | MÃ©tricas Prometheus |
| `POST` | `/api/v1/auth/login` | Login com senha â†’ JWT |
| `POST` | `/api/v1/auth/refresh` | Renovar JWT com refresh token |
| `GET` | `/api/v1/auth/github` | Iniciar login via GitHub |
| `GET` | `/api/v1/projects/{key}/badge` | Badge SVG do quality gate |

### Endpoints autenticados

**Projetos e Scans:**

| MÃ©todo | Rota | PermissÃ£o | DescriÃ§Ã£o |
|--------|------|-----------|-----------|
| `POST` | `/api/v1/projects` | `can_scan` | Criar/atualizar projeto |
| `GET` | `/api/v1/projects` | `can_view` | Listar projetos |
| `GET` | `/api/v1/projects/{key}` | `can_view` | Detalhes do projeto |
| `POST` | `/api/v1/scans` | `can_scan` | Enviar relatÃ³rio (ingestÃ£o) |
| `GET` | `/api/v1/projects/{key}/scans` | `can_view` | HistÃ³rico de scans |

**Issues e Busca:**

| MÃ©todo | Rota | PermissÃ£o | DescriÃ§Ã£o |
|--------|------|-----------|-----------|
| `GET` | `/api/v1/issues` | `can_view` | Buscar issues com filtros e facets |
| `POST` | `/api/v1/issues/{id}/transition` | `can_comment` | Mudar status (confirmar, fechar, reabrir) |
| `GET` | `/api/v1/issues/{id}/changelog` | `can_view` | HistÃ³rico de transiÃ§Ãµes |
| `GET` | `/api/v1/search` | `can_view` | Busca full-text |

**AdministraÃ§Ã£o:**

| MÃ©todo | Rota | PermissÃ£o | DescriÃ§Ã£o |
|--------|------|-----------|-----------|
| `POST/GET` | `/api/v1/users` | `manage_users` | Gerenciar usuÃ¡rios |
| `POST/GET` | `/api/v1/groups` | `manage_groups` | Gerenciar grupos |
| `POST/GET` | `/api/v1/gates` | `project_admin` | Configurar quality gates |
| `POST/GET` | `/api/v1/profiles` | `project_admin` | Configurar quality profiles |
| `PUT` | `/api/v1/projects/{key}/new-code` | `project_admin` | Configurar new code period |
| `POST/GET` | `/api/v1/projects/{key}/webhooks` | `project_admin` | Gerenciar webhooks |
| `POST` | `/api/v1/admin/reindex` | `admin` | Reindexar busca |
| `GET` | `/api/v1/system/info` | `admin` | InformaÃ§Ãµes do sistema (versÃ£o, stats) |

---

## Parte 12: Webhooks â€” notificaÃ§Ãµes automÃ¡ticas

Webhooks permitem que sistemas externos sejam notificados quando algo acontece no Ollanta.

### Eventos disponÃ­veis

| Evento | Quando dispara |
|--------|---------------|
| `scan.completed` | Um scan foi processado com sucesso |
| `gate.changed` | O status do quality gate mudou (OK â†’ ERROR ou vice-versa) |
| `project.created` | Um novo projeto foi criado |
| `project.deleted` | Um projeto foi deletado |

### Como funciona a entrega

```mermaid
sequenceDiagram
    participant P as Pipeline de IngestÃ£o
    participant D as Dispatcher
    participant W as Seu Servidor (webhook URL)

    P->>D: "scan.completed no projeto X"
    D->>D: Busca webhooks registrados para esse evento
    D->>D: Assina payload com HMAC-SHA256(secret)
    D->>W: POST url + headers de seguranÃ§a

    alt Sucesso (2xx)
        W-->>D: 200 OK âœ“
    else Falha
        W-->>D: 500 / timeout
        Note over D: Retry em 1min, 5min, 30min
        D->>W: Tentativa 2...
        alt Falha apÃ³s 3 tentativas
            D->>D: Registra como dead-letter
        end
    end
```

**Headers enviados:**
- `X-Ollanta-Event: scan.completed` â€” qual evento
- `X-Ollanta-Signature: sha256=abc123...` â€” HMAC para vocÃª validar autenticidade
- `X-Ollanta-Delivery: uuid` â€” ID Ãºnico da entrega

---

## Parte 13: MigraÃ§Ãµes do Banco

O schema evolui via migraÃ§Ãµes numeradas, aplicadas em ordem:

| # | O que cria | Por quÃª |
|---|-----------|---------|
| 001 | `projects` | Registrar projetos analisados |
| 002 | `scans` | HistÃ³rico de cada execuÃ§Ã£o de scan |
| 003 | `issues` (particionada) | Issues encontradas, com Ã­ndices para busca rÃ¡pida |
| 004 | `measures` | MÃ©tricas numÃ©ricas por scan |
| 005 | `users` | Contas de usuÃ¡rio |
| 006 | `groups` + `group_members` | Grupos para permissÃµes coletivas |
| 007 | `permissions` | PermissÃµes globais e por projeto |
| 008 | `tokens` | API tokens (prefixo `olt_`) |
| 009 | `sessions` | Refresh tokens (prefixo `ort_`) |
| 010 | Seed admin | Cria usuÃ¡rio admin/admin |
| 011 | `quality_profiles` + `profile_rules` | Perfis de regras por linguagem |
| 012 | `quality_gates` + `gate_conditions` | Gates com condiÃ§Ãµes configurÃ¡veis |
| 013 | `new_code_periods` | ConfiguraÃ§Ã£o de baseline por projeto |
| 014 | `webhooks` + `webhook_deliveries` | Webhooks e log de entregas |
| 015 | Ajustes menores | CorreÃ§Ãµes de schema |
| 016 | Coluna `resolution` em `issues` | Motivo de fechamento (fixed, false_positive, won't_fix) |
| 017 | `engine_id` + `secondary_locations` | Suporte multi-engine e contexto expandido |
| 018 | `changelog` | HistÃ³rico de transiÃ§Ãµes de issues |
| 019 | Fix `quality_profiles` | Remove perfis Java/C# (nÃ£o suportados), adiciona perfil Rust |

---

## GlossÃ¡rio

| Termo | O que significa |
|-------|----------------|
| **Issue** | Um problema encontrado no cÃ³digo (bug, vulnerabilidade, code smell, hotspot) |
| **Scan** | Uma execuÃ§Ã£o completa de anÃ¡lise sobre um projeto |
| **Component** | Um nÃ³ na hierarquia: projeto â†’ mÃ³dulo â†’ pacote â†’ arquivo |
| **Rule** | Uma regra que sabe detectar um tipo de problema (ex: "funÃ§Ã£o grande demais") |
| **Measure** | Um valor numÃ©rico de mÃ©trica para um componente (ex: ncloc = 1500) |
| **Quality Gate** | Conjunto de condiÃ§Ãµes que determinam se o projeto "passa" ou "falha" |
| **Quality Profile** | Conjunto de regras ativas para uma linguagem (ex: "Sonar Way Go") |
| **New Code Period** | Ponto de referÃªncia que define o que Ã© "cÃ³digo novo" |
| **LineHash** | SHA-256 do conteÃºdo de uma linha â€” identidade estÃ¡vel de uma issue |
| **Tracking** | Algoritmo que correlaciona issues entre scans usando (rule_key + line_hash) |
| **Sensor** | Componente que executa regras: GoSensor (Go nativo) ou TreeSitterSensor |
| **IngestÃ£o** | Pipeline que recebe um relatÃ³rio e persiste scans, issues e mÃ©tricas |
| **Port** | Interface que isola o domÃ­nio de implementaÃ§Ãµes concretas (hexagonal) |
| **Adapter** | ImplementaÃ§Ã£o concreta de um port (ex: PostgreSQL implementa IProjectRepo) |
| **Index Worker** | Worker in-process que indexa as issues do scan apÃ³s a ingestÃ£o |
| **CumSum** | PropagaÃ§Ã£o de mÃ©tricas das folhas para a raiz da Ã¡rvore de componentes |
| **SARIF** | Static Analysis Results Interchange Format â€” formato padrÃ£o da indÃºstria |
