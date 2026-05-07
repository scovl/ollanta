# Ollanta — Revisão Geral do Projeto

## O que é

Ollanta é uma **plataforma de análise estática multilinguagem** escrita em Go. Ela varre código-fonte, detecta bugs, code smells e vulnerabilidades, computa métricas e avalia quality gates configuráveis.

Dois componentes principais:

- **Scanner** (`ollantascanner`) — CLI local que descobre arquivos, aplica regras, produz relatórios JSON/SARIF. Opcionalmente serve uma UI web local na porta 7777.
- **Server** (`ollantaweb`) — Servidor REST (porta 8080) que recebe relatórios de scan, rastreia issues ao longo do tempo e avalia quality gates. Acompanhado de PostgreSQL, ZincSearch e workers assíncronos.

## Stack

- **Go 1.21+** com tree-sitter (CGo) para parser de JS, TS, Python, Rust
- **Go nativo** (`go/ast`) para análise de código Go
- **PostgreSQL 17** + **ZincSearch** como storage
- **Docker Compose** para orquestração (perfis: scanner, server, push, observability)
- **chi/v5** como router HTTP
- **Frontend** em TypeScript/Vite no scanner local

## Arquitetura

**Hexagonal** (ports & adapters) com 3 anéis:

| Anel | Módulos | Dependências |
|------|---------|-------------|
| **Inner** | `domain/` | Só stdlib Go |
| **Middle** | `application/` | Só `domain/` |
| **Outer** | `adapter/`, `ollantaweb/`, `ollantastore/` | Tudo, mas setas apontam para dentro |

## Módulos (10 no total, cada um com `go.mod` próprio)

| Módulo | Função | CGo |
|--------|--------|-----|
| `domain/` | Modelos puros, interfaces de porta, serviços | Não |
| `application/` | Casos de uso: scan, ingest, análise | Não |
| `adapter/` | HTTP, OAuth, Postgres, parser, telemetria, webhook | Sim* |
| `ollantacore/` | Tipos legados compartilhados | Não |
| `ollantaparser/` | Bindings C do tree-sitter — **único CGo verdadeiro** | **Sim** |
| `ollantarules/` | Registro de regras, sensores Go/tree-sitter | Sim* |
| `ollantascanner/` | CLI, descoberta de arquivos, executor paralelo | Sim* |
| `ollantaengine/` | Quality gates, rastreamento de issues, sumarização | Não |
| `ollantastore/` | Repositórios PostgreSQL (pgx/v5), busca (ZincSearch/PG FTS) | Não |
| `ollantaweb/` | Servidor REST, ingestão, auth (chi/v5) | Não |

_\*CGo transitivo via `ollantaparser`_

## Fronteira CGo

- Só `ollantaparser` tem CGo direto. O `domain/` usa `any` para tipos tree-sitter para permanecer CGo-free.
- `ollantaweb` **nunca pode** importar `ollantaparser` ou `ollantarules` transitivamente, pois seu Dockerfile compila com `CGO_ENABLED=0`.

## Padrão Adapter Bridge

`adapter/secondary/rules/bridge.go` converte entre tipos legados (`ollantacore/domain.Issue`) e hexagonais (`domain/model.Issue`). Nunca misturar os tipos diretamente.

## Regras

Cada regra tem **3 arquivos**, sem editar arquivos existentes:

1. Lógica: `ollantarules/languages/{lang}/rules/my_rule.go`
2. Metadados JSON: `ollantarules/languages/{lang}/rules/{lang}_{rule-key}.json`
3. Registro: Adicionar ao `MustRegister()` no `embed.go` da linguagem

**OBSOLETO:** o `ollantaweb/api/rules_data/` foi removido. Os metadados agora vivem em `ollantacore/rulecatalog/`, um pacote CGo-free compartilhado entre scanner e servidor.

## Convecões Notáveis

| Convenção | Exemplo |
|-----------|---------|
| Interfaces prefixadas com `I` | `IProjectRepo`, `IAnalyzer` |
| Construtores com `New` | `NewRegistry()`, `NewIngestUseCase()` |
| Chaves de regra `{lang}:{kebab-name}` | `go:no-large-functions`, `py:broad-except` |
| Tags JSON em snake_case | `json:"rule_key"` |
| `context.Context` sempre como primeiro argumento | `func(ctx context.Context, ...)` |
| Erros sentinela | `var ErrNotFound = errors.New("not found")` |
| Verificações compile-time de interface | `var _ port.IAnalyzer = (*AnalyzerBridge)(nil)` |

## CI/CD

4 jobs paralelos em push/PR para `main`:
- `test-scanner` (CGo) — módulos com CGo
- `test-web` (sem CGo) — domain, application, ollantaweb, ollantastore
- `test-adapter` (CGo + Postgres)
- `lint` — golangci-lint v2 por módulo (nunca rodar na raiz)

## Pontos de Atenção

1. **Cópia de JSONs de regras** — RESOLVIDO. `ollantaweb/api/rules_data/` foi removido. Os metadados foram consolidados em `ollantacore/rulecatalog/`, um módulo sem CGo. Não há mais duplicação manual.
2. **Erros não podem ser silenciados** — `_, _ = f()` é proibido. Sempre tratar ou propagar.
3. **Respostas HTTP consistentes** — Sempre `Content-Type: application/json`, erros no formato `{"error": "mensagem"}`.
4. **Router Chi** — Usar `r.Route` com `r.Group` aninhado para separar GETs públicos de writes admin.
5. **Clean Architecture** — Anel interno (domain) importa só stdlib. Anel médio (application) importa só domain. Adaptadores ficam no anel externo.
6. **Log estruturado com `slog`** — `Info` para eventos operacionais, `Debug` para rastreabilidade, `Error` para falhas. Sempre com contexto estruturado (`slog.With`). Remover `log.Printf` antes de commitar.
