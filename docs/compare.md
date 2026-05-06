# Ollanta vs SonarQube — Comparacao Tecnica

Comparacao arquitetural e operacional entre Ollanta e SonarQube Community/Enterprise.

## Arquitetura

| Aspecto | SonarQube | Ollanta |
|---------|-----------|---------|
| Linguagem | Java (JDK 21) | Go |
| Build | Gradle, Maven, Artifactory | Go modules + Makefile |
| Arquitetura | Monolitico com sub-processos (Web + CE + ES no mesmo JVM) | Hexagonal (ports & adapters) com binarios separados |
| Web server | Tomcat embedido via Servlet API | chi/v5 (Go) |
| Processamento | Compute Engine como processo separado dentro do mesmo JVM | Worker stateless separado (ollantaworker) |
| Plugin system | Sim (third-party via classloader) | Nao (regras built-in + custom rules YAML/tree-sitter) |
| Separacao de responsabilidades | Process-level (Web vs CE vs Search) | Module-level (domain/application/adapter rings) |
| CGo / FFI | N/A (Java puro) | CGo so em `ollantaparser`; `ollantaweb` compila com `CGO_ENABLED=0` |

## Search Engine

| Aspecto | SonarQube | Ollanta |
|---------|-----------|---------|
| Engine | Elasticsearch 7.x **embutido** no JVM | ZincSearch (externo, Go) ou PostgreSQL FTS (in-DB) |
| Indices | 4+ (issues, components, measures, rules) | 2 (issues, projects); expandindo para 4 |
| Index lifecycle | Rebuild completo no boot (6h+ com 200k projetos) | Diff indexing assincrono; boot instantaneo (~2s) |
| Sharding | Nao (single node embutido) | Multi-instancia ZincSearch com shared volume |
| Fallback | Nao (ES cai = app cai) | Sim (fallback para PG FTS com `X-Search-Backend` header) |
| Rich filtering | CWE, OWASP, PCI DSS, impacts, code variants | Basico (rule_key, type, severity, language, tags) |
| Auth-aware search | Sim (parent-child docs no ES) | Nao (auth na camada de aplicacao) |
| Licenca | Elastic License v2 (restritiva) | Apache 2.0 (ZincSearch) + PostgreSQL (gratuito) |

## Database

| Aspecto | SonarQube | Ollanta |
|---------|-----------|---------|
| Engine | PostgreSQL 15+ (primary); Oracle, SQL Server (enterprise) | PostgreSQL 17+ (only) |
| ORM | MyBatis (XML + annotations) | pgx/v5 (hand-written SQL) |
| Migrations | `sonar-db-migration` (custom engine) | Raw SQL files em `migrations/` |
| Particionamento | Nao (tabelas monolíticas) | RANGE por `created_at` (existente); HASH por projeto planejado |
| Live measures | Sim (`live_measures` + `project_measures`) | Sim (UPSERT atomico via `INSERT ... ON CONFLICT`) |
| Measure rollup | Nao (guarda cada scan) | Sim (`measure_daily_aggregates` com media/max/min diario) |
| Read replicas | Sim (enterprise) | Sim (via `pgxpool` separados, fallback nativo) |
| Bulk insert | MyBatis batch | PostgreSQL `COPY` protocol (50× mais rapido) |
| Batch UPSERT | Nao | Multi-row `unnest()` — 1 query para N metricas |
| Data lifecycle | Manual (admin precisa limpar) | TTL automatico: jobs 7d, scans 365d, measures 90d |
| Component tree | `components` table com UUID tree | Sim (UUID deterministico SHA256, qualifier TRK/DIR/FIL) |
| File hash | Checksum-based para duplicacao | SHA256 das primeiras 64 linhas para deteccao de rename |
| ETL-ready | Via API REST (lento, paginado) | Direto no PostgreSQL (BI/ETL via SQL) |

## Scanner

| Aspecto | SonarQube | Ollanta |
|---------|-----------|---------|
| Runtime | Java CLI + JRE 21 embutido | Binario Go nativo (zero dependencias) |
| Tamanho | ~300 MB (JRE + JARs) | ~25 MB (binario compilado) |
| Analise | Sensores via plugins (server-side language support) | Tree-sitter nativo (Go, JS, TS, Python, Rust) |
| Linguagens | 30+ (via plugins do servidor) | 5 (built-in com tree-sitter) |
| Custom rules | Nao (requer plugin Java) | Sim (YAML/JSON + tree-sitter queries) |
| Modo standalone | Nao (sempre precisa de servidor) | Sim (scan local com JSON/SARIF output) |
| Formato report | Protobuf binario zipado | JSON (com gzip compression) |
| Protocolo upload | `POST /api/ce/submit` (multipart) | `POST /api/v1/scans` (JSON body) |
| UI local | Nao | Sim (port 7777, React/Vite) |
| Test signals | Via plugins separados | Built-in (collect, run, doctor modes) |
| Mutation signals | Nao disponivel | Built-in (mutation score, changed-code mutants) |
| AI fix suggestions | Nao | Sim (local + server-side via LLM) |

## Processamento (Ingest Pipeline)

| Aspecto | SonarQube CE | Ollanta Ingest |
|---------|-------------|----------------|
| Processo | JVM separado via ProcessLauncher | Goroutine pool no ollantaworker |
| Pipeline steps | 58 passos (dependency injection) | 9 passos (error strategy: abort/skip/retry) |
| Workers | Configuravel (`ce.workerCount`) | Configuravel (`OLLANTA_WORKER_POOL`, default 4) |
| Claim atomico | Polling `ce_queue` (status=PENDING) | `SELECT ... FOR UPDATE SKIP LOCKED` |
| Lock por projeto | Exclusivo (um worker por projeto) | Exclusivo (um worker por projeto) |
| Round-trips/scan | N INSERTs + N queries | ~4 (COPY + batch UPSERT + bulk search) |
| Timeout por step | Nao (bloqueia indefinidamente) | Sim (abort/skip/retry com timeout) |
| File move detection | Sim (FileMoveDetectionStep) | Sim (FileHash cross-path) |
| Duplication detection | Sim (intra e cross-project) | Nao |
| Dependency analysis | Sim (ScaStep) | Nao |
| Backpressure | Nao (fila cresce ilimitada) | Sim (HTTP 429 + Retry-After) |
| Idempotencia | Nao (reenvio duplica) | Sim (idempotency key SHA-256) |

## API

| Aspecto | SonarQube | Ollanta |
|---------|-----------|---------|
| Endpoints | 300+ (WS framework custom) | ~80 (chi/v5 router) |
| Versionamento | `/api/ce`, `/api/issues`, `/api/measures` (sem versao) | `/api/v1/*` |
| Paginacao | Page-based (p, ps) | Limit/offset |
| Autenticacao | Built-in (JWT, SAML, OAuth, LDAP) | JWT + OAuth (GitHub, GitLab, Google) + scanner token |
| Batch operations | Sim (BulkChangeAction) | Tags bulk apenas |
| Admin API | Implicita (permission checks por action) | Explicita (`r.Group` + `RequirePermission`) |
| Background tasks | CE queue monitoring | `/admin/background-tasks` (scan, index, webhook) |
| Search API | Especifica por dominio | Unificada (`/search?q=&type=projects,issues,rules`) |

## Escalabilidade

| Aspecto | SonarQube | Ollanta |
|---------|-----------|---------|
| Horizontal web | Nao (ES embutido = single writer) | Sim (multiplas instancias, mesmo DB + ZincSearch) |
| Horizontal workers | Sim (multi-worker CE) | Sim (multi-worker com claim atomico) |
| Read replicas | Enterprise only | Sim (pgxpool separados, fallback nativo) |
| Sharding | Nao | Sim (ZincSearch multi-instancia + hash partitioning DB) |
| Boot time (0 projetos) | ~30s (ES init) | ~2s (Go binario) |
| Boot time (200k projetos) | ~6h (ES rebuild completo) | ~5s (indice em disco, sem rebuild) |
| Search down | App nao inicia | App sobe, fallback PG FTS |
| Worker crash | Job perdido (re-enfileirado manualmente) | Job re-claimed automaticamente (heartbeat) |

## Resiliencia

| Aspecto | SonarQube | Ollanta |
|---------|-----------|---------|
| Circuit breaker | Nao | Sim (PG, ZincSearch, Webhooks; Closed→Open→Half-Open) |
| Graceful degradation | Nao (ES cai = tudo cai) | Sim (search skip no ingest, PG FTS fallback) |
| Health checks | Basico (liveness/readiness) | Granular (ready/degraded/not_ready por componente) |
| Step timeouts | Nao | Sim (abort/skip/retry strategies) |
| Worker heartbeat | Nao (job lock e DB-mediated) | Sim (heartbeat a cada 10s, re-claim apos 30s) |
| Retry com backoff | Nao | Sim (exponencial: 1min, 5min, 30min, max 3 tentativas) |
| Backpressure | Nao | Sim (bounded queue, HTTP 429 + Retry-After) |

## Operacao

| Aspecto | SonarQube | Ollanta |
|---------|-----------|---------|
| Deploy | Java JAR/WAR + ES + DB | Binario Go + PostgreSQL (+ ZincSearch opcional) |
| Dependencias | JDK 21, ES 7.x, PostgreSQL | PostgreSQL 17+ |
| Configuracao | `sonar.properties` (Java .properties) | `config.toml` (TOML) + env vars com `${env.VAR}` |
| Proxy | `sonar.scanner.proxyHost`/`proxyPort` | `-proxy` flag + `HTTP_PROXY` env var |
| Skip scan | `sonar.scanner.skip=true` | `-skip` flag + `skip` config |
| Versionamento | `--version` | `--version` (via ldflags) |
| Cross-compile | Nao (Java, mas precisa JRE por plataforma) | Sim (`make release`: linux/windows/darwin, amd64/arm64) |
| Observabilidade | JMX + logs | Prometheus metrics + `slog` estruturado + tracing |
| Webhooks | Sim (assincrono, retry) | Sim (assincrono, retry exponencial, HMAC-SHA256) |
| Quality profiles | Sim (heranca, profile-as-code) | Sim (heranca max 3 niveis, YAML profile-as-code) |

## Quality Gate

| Aspecto | SonarQube | Ollanta |
|---------|-----------|---------|
| Condicoes | metric + operator (GT/LT) + error threshold | metric + operator (6 ops) + error threshold + warning threshold |
| Status | OK / ERROR | OK / WARN / ERROR |
| On new code | Sim | Sim (`on_new_code` flag) |
| Gate assignment | Por projeto | Por projeto |
| Gate CRUD | API + UI | API + UI (admin tab com CRUD completo) |
| Auto-condicoes | Sim (CAYC: 4 condicoes) | Sim (4 condicoes padrao ao criar gate) |
| Warning threshold | Nao (so error) | Sim (`warning_threshold` opcional) |
| Small changeset | Nao | Sim (pula `new_coverage`/`new_duplications`) |
| Gate changed webhook | Sim (`gate.changed` event) | Sim (`gate.changed` event) |
| Scanner exit code | Gate ERROR → exit 1 (generico) | Gate ERROR → exit 3 (semantico) |

## Pontos Fortes do Ollanta

1. **Boot instantaneo** (~2s vs 30s-6h do SonarQube) — sem rebuild de indice no startup
2. **Horizontally scalable** — multiplas instancias web e workers, mesmo DB + ZincSearch
3. **Binario unico** — scanner Go de 25MB sem JRE, sem plugins, sem bootstrap
4. **Standalone mode** — scan local sem servidor, JSON/SARIF output
5. **Test + mutation signals** — coleta built-in que o SonarQube nao tem
6. **Custom rules** — YAML/tree-sitter sem precisar escrever plugin Java
7. **Idempotencia** — push com idempotency key, sem duplicatas
8. **Backpressure** — 429 + Retry-After, queue controlada
9. **Warning threshold** — WARN status entre OK e ERROR, ausente no SonarQube
10. **ETL-ready** — PostgreSQL direto para BI/ETL, sem depender de API paginada
11. **COPY + batch UPSERT** — ~4 round-trips/scan vs N dezenas no SonarQube
12. **Goroutine pool** — workers Go leves e configuráveis, sem overhead de JVM/thread
13. **Data lifecycle automático** — TTL para scans, jobs, measures sem intervencao manual
14. **Worker heartbeat** — re-claim automatico de jobs em < 30s se worker crashar
15. **Live measures** — dashboard e overview instantaneos, sem subqueries complexas

## Pontos Fracos do Ollanta (vs SonarQube)

1. **Linguagens** — 5 vs 30+ (mas custom rules suprem parte da lacuna)
2. **Maturidade da API** — ~80 endpoints vs 300+
3. **Rich issue filtering** — basico vs CWE/OWASP/impacts
4. **Duplication detection** — ausente; cross-project duplication tambem
5. **Dependency analysis** — ausente (SCA)
6. **Plugin ecosystem** — inexistente (decisao arquitetural, nao lacuna)
7. **Enterprise features** — LDAP, SAML, portfolio management, governance reports (fora de escopo para MVP)
