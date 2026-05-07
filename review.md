# Review arquitetural e operacional do Ollanta

## Escopo

Este review avalia o estado atual do Ollanta com foco em quatro perguntas principais:

- O sistema consegue escalar horizontalmente?
- Ele e resiliente a falhas, reinicios e concorrencia?
- Ele e observavel o suficiente para operar em producao?
- Ele e efemero/cloud-native, isto e, seus processos podem ser recriados sem perda de estado essencial?

Tambem foram revisados pontos positivos, decisoes ruins ou frageis, over-engineering, dividas tecnicas e divergencias entre documentacao e codigo.

A analise foi feita por leitura estatica do repositorio, especialmente `docker-compose.yml`, `README.md`, `docs/`, `ollantaweb/`, `application/`, `ollantastore/`, `adapter/secondary/telemetry/` e os reviews historicos `over.md` e `over02.md`. Nao foram executados testes de carga, chaos tests, builds completos nem validacao em Kubernetes real.

## Veredito executivo

O Ollanta esta bem encaminhado para uma arquitetura operacional escalavel e efemera. O ponto mais importante e que o runtime atual ja separa papeis que deveriam escalar de forma independente: API (`ollantaweb`), compute worker (`ollantaworker`), search indexer (`ollantaindexer`) e webhook worker (`ollantawebhookworker`). Alem disso, scan, indexacao e webhook delivery usam jobs duraveis em PostgreSQL com claim concorrente via `FOR UPDATE SKIP LOCKED`, o que e uma base correta para multiplas replicas.

Ainda assim, eu nao classificaria o Ollanta como pronto para escala grande ou operacao critica sem ajustes. O desenho esta correto para pequeno/medio porte, mas faltam controles importantes de producao: idempotencia no intake, backpressure/rate limiting real, recuperacao automatica de jobs stale, coordenacao de migrations, configuracao de pool de banco por ambiente, readiness mais honesto dos workers e atualizacao da documentacao que ficou atras do codigo.

Resumo curto: **o Ollanta pode escalar horizontalmente em sua forma atual, mas com teto operacional claro no PostgreSQL, no ZincSearch single-node e em alguns caminhos nao idempotentes. Ele e razoavelmente resiliente, bem observavel para uma primeira versao e majoritariamente efemero nos processos stateless. A maturidade cloud-native existe na direcao, mas ainda nao esta completa.**

## Matriz de avaliacao

| Criterio | Avaliacao | Veredito |
| --- | --- | --- |
| Escalabilidade horizontal | Boa base, incompleta | API e workers escalam por replicas; PostgreSQL, pool fixo, ZincSearch single-node e falta de rate limiting limitam o teto. |
| Resiliencia | Media/boa | Jobs duraveis, retries e graceful shutdown ajudam; falta idempotencia, transacao fim-a-fim e auto-recovery de stale jobs. |
| Observabilidade | Boa base | Logs estruturados, metricas Prometheus, traces OTLP e admin endpoints existem; faltam alertas, metricas de DB e readiness mais real. |
| Efemeridade | Boa para app, parcial para stack | API/workers sao stateless e containerizados; Postgres/ZincSearch sao stateful por natureza e migrations no startup acoplam pods ao DB. |
| Simplicidade/manutenibilidade | Media | Hexagonal architecture e separacao de roles ajudam; ha boilerplate, codigo residual e docs divergentes. |

## Arquitetura atual observada

O Ollanta tem dois grandes blocos:

- **Scanner**: CLI/local UI que analisa codigo, gera JSON/SARIF e opcionalmente envia o resultado para o servidor.
- **Server**: API central que recebe scans, persiste historico, calcula quality gate, expoe dashboards, busca e webhooks.

No servidor, o desenho operacional atual esta dividido em quatro binarios principais, todos produzidos pelo Dockerfile de `ollantaweb`:

- `ollantaweb`: API HTTP, ingest assincorno, queries, auth, dashboards e endpoints administrativos.
- `ollantaworker`: consome `scan_jobs`, executa o processamento de ingestao e cria jobs derivados.
- `ollantaindexer`: consome `index_jobs` e atualiza o backend de busca.
- `ollantawebhookworker`: consome `webhook_jobs` e entrega webhooks com retry.

Essa separacao e uma decisao arquitetural boa. Ela permite escalar API, processamento, indexacao e integracoes externas com perfis diferentes de CPU, memoria, conexoes e tolerancia a latencia.

## Escalabilidade horizontal

### Pontos positivos

O caminho principal de ingestao e assincorno. `POST /api/v1/scans` persiste um `scan_job` e retorna `202 Accepted`, em vez de manter a requisicao presa ate o processamento completo. Isso protege a API de scans pesados e torna natural escalar workers separadamente.

As tabelas `scan_jobs`, `index_jobs` e `webhook_jobs` usam jobs duraveis em PostgreSQL. Os repositorios fazem claim concorrente com `FOR UPDATE SKIP LOCKED`, o que permite multiplos workers sem coordenador externo. Esse e um padrao simples, robusto e adequado para a escala inicial do produto.

O `docker-compose.yml` ja reflete os papeis separados: `ollantaweb`, `ollantaworker`, `ollantaindexer` e `ollantawebhookworker`. A documentacao de Kubernetes tambem aponta para HPA, probes, PDB e security context, ainda que precise de atualizacao em alguns trechos.

O servidor e majoritariamente stateless. O estado relevante fica no PostgreSQL e, para busca, no ZincSearch ou no fallback Postgres FTS. Isso e pre-requisito para replicas horizontais.

### Limitacoes e riscos

O principal gargalo de escala passa a ser o PostgreSQL. O pool de conexoes em `ollantastore/postgres/db.go` esta hardcoded com `MaxConns = 25` e `MinConns = 5`. Com varios pods e quatro tipos de processo, isso pode esgotar o banco rapidamente. Exemplo: 4 replicas de API, 4 workers, 2 indexers e 2 webhook workers ja podem reservar ou disputar centenas de conexoes dependendo da configuracao real.

Nao ha idempotency key no endpoint de ingestao. Se o scanner ou uma esteira CI reenviar o mesmo report por timeout, retry de rede ou duplicidade operacional, o sistema pode criar jobs e scans duplicados. Em pequena escala isso e toleravel; em producao vira custo, ruido historico e risco de webhooks duplicados.

Nao ha rate limiting ou backpressure HTTP claro no caminho atual de `POST /api/v1/scans`. Existem estruturas `IngestQueue` em memoria com `ErrQueueFull`, mas o handler atual grava `scan_jobs` diretamente. Na pratica, o limite de entrada e o banco, o tamanho maximo do body e os recursos do processo. Isso funciona enquanto a carga e baixa, mas em burst grande a pressao cai no PostgreSQL.

O ZincSearch aparece como single replica com volume. Como o Postgres e source of truth, perder o indice nao perde dados, mas a busca pode degradar ou exigir reindexacao. Para escala horizontal real, o backend de busca precisa de uma estrategia clara: operar ZincSearch como dependencia stateful com backup/restore/HA, usar Postgres FTS conscientemente para volumes moderados, ou adotar um search backend com operacao mais madura.

Alguns endpoints listam dados e paginam em memoria. Por exemplo, listagens de scans por projeto e background tasks carregam conjuntos internos limitados e depois paginam. Isso e aceitavel no inicio, mas vira problema com historico longo.

### Veredito de escala

O Ollanta escala horizontalmente no plano de aplicacao e workers, desde que o PostgreSQL aguente. Para escala pequena/media, o desenho e bom. Para escala grande, faltam idempotencia, rate limiting, tuning de pool, paginação no banco, estrategia de busca e limites operacionais documentados.

## Resiliencia

### Pontos positivos

Jobs duraveis sao a maior conquista de resiliencia. Um restart de API nao perde scans aceitos. Um restart de worker nao perde indexacoes ou webhooks ja persistidos. Isso elimina a critica antiga de depender de fila local em memoria para trabalho importante.

Os workers usam retry/backoff em indexacao e webhook delivery. Webhooks tambem mantem informacoes como status code e corpo de resposta parcial, o que ajuda no diagnostico.

Os servidores HTTP tem timeouts configurados e graceful shutdown com `signal.NotifyContext` e `srv.Shutdown`. Isso reduz risco de conexoes penduradas durante deploy.

O scanner tem um ponto positivo local: executa analise paralela e isola panics por arquivo com `recover`, evitando que um arquivo problematico derrube a analise inteira.

### Limitacoes e riscos

A ingestao nao parece ser uma transacao unica fim-a-fim. O fluxo cria scan, insere issues, measures, snapshot, index jobs e webhooks em etapas. Se uma etapa abortar depois de persistir parte do estado, pode haver scan sem todos os dados derivados ou efeitos secundarios ausentes. Algumas etapas usam estrategia `Skip`, o que e pragmatico, mas precisa de compensacao ou reconciliacao quando o efeito e importante.

Jobs `running` que ficam presos viram `stale` de forma derivada para visibilidade, mas nao ha requeue automatico no caminho analisado. A API administrativa permite retry/requeue/cancel, o que e bom para operacao manual, mas resiliencia automatica exige um reconciler ou janitor.

Nao ha deduplicacao evidente para `index_jobs` por `scan_id` nem para `webhook_jobs` por evento/destino. Se a etapa de emissao for repetida, efeitos duplicados podem ocorrer.

As migrations rodam no startup dos binarios e nao foi observado advisory lock. Em ambiente com muitas replicas subindo ao mesmo tempo, DDL concorrente pode gerar falhas, locks longos ou comportamento dificil de diagnosticar. O ideal e separar migrations como job de deploy ou usar lock coordenado.

O JWT secret aleatorio quando `OLLANTA_JWT_SECRET` nao e definido e perigoso para multi-replica: tokens emitidos por uma replica podem nao validar em outra, e restarts invalidam sessoes. A documentacao alerta, mas o default e hostil para producao se alguem esquecer a variavel.

O CORS permissivo (`*`) e uma decisao fragil para producao. Pode ser aceitavel em dev, mas deveria ser configuravel e restritivo por default em deployments reais.

### Veredito de resiliencia

O Ollanta tem boa resiliencia estrutural por causa dos jobs duraveis e papeis separados, mas ainda depende demais de operacao manual para recuperar stale jobs e nao garante idempotencia/atomicidade nos pontos mais sensiveis. A resiliencia e boa para uma plataforma em amadurecimento, ainda nao para ambiente critico sem endurecimento.

## Observabilidade

### Pontos positivos

Ha uma base de observabilidade acima da media para um projeto nesse estagio:

- Logs estruturados com `slog`.
- Metricas Prometheus em formato texto.
- Tracing OpenTelemetry via OTLP HTTP.
- Propagacao de trace context em jobs duraveis (`trace_parent`, `trace_state`).
- Admin server nos workers com `/healthz`, `/readyz` e `/metrics`.
- Stack local com Prometheus, Jaeger, Loki e Promtail.
- Metricas especificas para requests HTTP, ingestao, filas, index jobs, webhooks e background tasks.

A presenca de endpoints administrativos para background tasks tambem e muito positiva. Ela transforma filas internas em operacao visivel, com estados normalizados e acoes de retry/requeue/cancel.

### Limitacoes e riscos

A readiness dos workers parece superficial quando `StartAdminServer` recebe `readyCheck` nulo. O processo estar vivo nao significa que consegue falar com PostgreSQL, ZincSearch ou destinos externos. Isso pode fazer Kubernetes manter um pod em service quando ele deveria estar fora de rota ou ser reiniciado.

A readiness da API trata falha no search como degradacao com HTTP 200. Isso pode ser intencional, porque o sistema ainda consulta o Postgres, mas precisa ficar claro em alertas e dashboards. Caso contrario, busca quebrada pode passar despercebida.

Faltam metricas importantes para producao: latencia de queries de banco, erros por tipo/etapa da ingestao, duracao por step do pipeline, tempo em fila por job, idade do job mais antigo, contagem de stale jobs por tipo e taxa de retry/dead-letter com labels mais detalhados.

As metricas de background task summary parecem atualizadas quando o endpoint de summary e chamado, nao por coleta periodica propria. Isso reduz confianca para alertas se ninguem consultar a API.

O tracing vira no-op se o endpoint OTLP esta vazio ou inalcançavel no startup. Isso evita falha dura, mas pode mascarar perda de observabilidade se nao houver alerta de tracing desabilitado.

Ainda existem alguns usos de `log.Printf` em areas de aplicacao/scanner. O projeto tem guardrail para preferir `slog`, entao isso e divida de consistencia operacional.

### Veredito de observabilidade

A base e boa e ja permite operar localmente com visibilidade razoavel. Para producao, faltam alertas/SLOs e algumas metricas de profundidade. O Ollanta e observavel, mas ainda nao e plenamente operavel em incidentes complexos.

## Efemeridade e operacao cloud-native

### Pontos positivos

Os processos de aplicacao sao majoritariamente efemeros. API e workers nao dependem de disco local para estado essencial; o estado fica no PostgreSQL e no backend de busca. Isso permite recriar containers sem perda direta de scans aceitos.

A imagem de servidor usa build `CGO_ENABLED=0` e runtime distroless/static nonroot. Isso e excelente para seguranca, reproducibilidade e operacao em Kubernetes.

Os papeis separados combinam bem com deployments efemeros: cada workload pode ter requests/limits, autoscaling e politica de restart propria.

O scanner tambem e naturalmente efemero quando usado como job/CI: monta o projeto, roda analise, gera report e termina.

### Limitacoes e riscos

PostgreSQL e ZincSearch sao stateful por natureza. Isso nao e um problema, mas significa que a efemeridade vale para os processos de aplicacao, nao para a stack inteira. Backups, restore, retention, migracoes e operacao de volumes precisam estar claros.

Rodar migrations no startup dos binarios mistura ciclo de vida de deploy com ciclo de vida da aplicacao. Em cloud-native maduro, migrations costumam rodar como job controlado antes do rollout, com lock e observabilidade propria.

O scanner precisa montar `/project`, e isso e esperado. Mas em ambientes com workspace grande, arquivos gerados e report com snapshot de codigo podem gerar payloads grandes. O limite atual de body da API e 10 MB, enquanto alguns exemplos de infra falam em limites maiores. Essa divergencia deve ser resolvida.

### Veredito de efemeridade

API e workers estao bem proximos do ideal efemero. O ponto fraco nao e estado local nos processos, mas sim a operacao das dependencias stateful e o acoplamento de migrations ao startup.

## Pontos positivos fortes

1. **Separacao real de papeis operacionais.** A existencia de `ollantaweb`, `ollantaworker`, `ollantaindexer` e `ollantawebhookworker` e uma decisao correta e importante.

2. **Jobs duraveis em PostgreSQL.** `scan_jobs`, `index_jobs` e `webhook_jobs` reduzem perda de trabalho em restart e permitem concorrencia horizontal simples.

3. **Uso de `FOR UPDATE SKIP LOCKED`.** E um mecanismo adequado para work queues em Postgres sem introduzir Kafka/RabbitMQ cedo demais.

4. **Search como projecao.** Postgres permanece source of truth; ZincSearch pode ser reconstruido. Isso reduz acoplamento com o search backend.

5. **Observabilidade desde cedo.** Logs, metricas, tracing e admin endpoints ja estao no desenho.

6. **Imagem de servidor segura e simples.** Distroless nonroot com `CGO_ENABLED=0` para `ollantaweb` e coerente com o guardrail de nao puxar tree-sitter para o servidor.

7. **Hexagonal architecture bem intencionada.** A separacao `domain`, `application`, `adapter` ajuda a manter o core menos contaminado por HTTP, SQL e CGO.

8. **Scanner resiliente localmente.** Paralelismo e isolamento de panic por arquivo sao boas escolhas para ferramenta de analise estatica.

## Mas decisoes ou decisoes frageis

### 1. JWT secret aleatorio como fallback

Gerar secret aleatorio se `OLLANTA_JWT_SECRET` estiver vazio e conveniente para dev, mas perigoso para producao. Em multi-replica, cada pod pode assinar tokens diferentes. Em restart, sessoes podem quebrar. O ideal e falhar startup em ambiente production-like quando o secret nao estiver definido, ou exigir explicitamente `OLLANTA_ALLOW_RANDOM_JWT_SECRET=true` para dev.

### 2. Pool de PostgreSQL hardcoded

Pool fixo e uma decisao fragil para escala horizontal. O numero certo depende de replicas, workers, tamanho do banco, PgBouncer, CPU e workload. Isso deve ser configuravel por env vars e documentado com exemplos.

### 3. Migrations no startup de todos os processos

E simples, mas arriscado. Com varias replicas subindo, DDL concorrente pode criar incidentes. Melhor separar migrations como job de deploy ou usar advisory lock claro.

### 4. Falta de idempotencia no intake

O servidor aceita scans e cria jobs sem chave de deduplicacao. Em sistemas distribuidos, retries acontecem. Sem idempotencia, retries viram historico duplicado, index jobs duplicados e webhook duplicado.

### 5. Backpressure ausente no endpoint principal

A arquitetura tem uma ideia de fila em memoria com erro de queue full, mas o endpoint atual usa job table diretamente. Isso e melhor para durabilidade, mas ainda precisa de limite operacional: rate limit por token/projeto, limite de jobs pendentes, ou resposta 429 quando o sistema esta saturado.

### 6. Stale jobs dependem de acao manual

Expor stale jobs na API e bom, mas nao basta para resiliencia automatica. Um reconciler deveria requeuear ou falhar jobs running antigos conforme politica configuravel.

### 7. CORS permissivo por default

`Access-Control-Allow-Origin: *` pode ser aceitavel em desenvolvimento, mas deveria ser configuravel e seguro para producao.

### 8. Documentacao divergente do codigo

Algumas partes dos docs descrevem um estado antigo. Isso e perigoso porque operadores podem tomar decisoes erradas baseadas em documentacao obsoleta.

## Over-engineering e divida tecnica

### Circuit breaker nao usado

`ollantaweb/breaker` implementa um circuit breaker generico, mas a analise nao encontrou uso no runtime principal. Isso e candidato a over-engineering: uma abstracao correta em tese, mas sem integracao real. Se webhooks ou search precisam de breaker, ele deve ser conectado. Se nao, deve ser removido para reduzir superficie mental.

### Filas em memoria residuais

Existem estruturas `IngestQueue` em mais de um pacote, mas o caminho atual de ingestao usa `scan_jobs` duraveis. Isso parece sobra de desenho anterior. Codigo residual confunde leitura, especialmente porque sugere backpressure que a API atual nao aplica.

### Pipeline/wrappers de compatibilidade

Ha wrappers no pacote `ollantaweb/ingest` que delegam para `application/ingest`. Alguns parecem existir para compatibilidade historica. Isso pode ser aceitavel durante migracao, mas deve ter prazo: ou vira API clara, ou some.

### `RouterDeps` muito grande

O router recebe um conjunto grande de dependencias. Nao e um problema grave, mas e sinal de que a composicao HTTP esta crescendo. Se continuar, vale agrupar por contexto funcional ou introduzir composition roots menores.

### Duplicacao de metadados de regras — RESOLVIDO

A duplicacao de JSONs de regras entre `ollantarules` e `ollantaweb/api/rules_data` era uma divida conhecida por causa da fronteira CGO. Ela foi resolvida com a criacao do pacote `ollantacore/rulecatalog/` — um modulo CGo-free usado tanto pelo scanner quanto pelo servidor, eliminando a copia manual.

### Hexagonal architecture com custo de boilerplate

A arquitetura hexagonal faz sentido porque o projeto tem scanner, servidor, storage, search, rules e parser com fronteiras reais. O risco e exagerar interfaces e adapters para casos de uso simples. A regra pratica deveria ser: interfaces sao valiosas nos ports de dominio/aplicacao; fora disso, evitar abstrair so por simetria.

## Divergencias entre documentacao e codigo

1. **Indexacao em Kubernetes.** `docs/kubernetes.md` ainda descreve comportamento antigo de indexacao local/in-process em replicas de `ollantaweb`. O codigo atual tem `index_jobs` duraveis e `ollantaindexer`, entao essa secao deve ser atualizada.

2. **Transacao unica de ingestao.** Partes da documentacao arquitetural sugerem persistencia em uma unica transacao. O codigo atual mostra pipeline multi-step com estrategias de abort/skip. Se a decisao atual e multi-step, documentar compensacoes e invariantes. Se a intencao e transacao unica, falta implementar.

3. **Advisory locks.** A documentacao menciona advisory locks para coordenacao, mas nao foi observado uso efetivo em migrations ou indexacao no levantamento. Atualizar docs ou implementar.

4. **Reviews antigos.** `over.md` e `over02.md` sao historicos e parcialmente obsoletos. Eles sao uteis para entender a evolucao, mas nao devem ser lidos como diagnostico atual sem a nota de status.

5. **Limites de payload.** A API limita body em 10 MB, enquanto exemplos de infra podem sugerir limites maiores. Como reports podem carregar code snapshots, isso precisa estar alinhado.

## Recomendacoes priorizadas

### P0 - Antes de producao multi-replica

1. Tornar `OLLANTA_JWT_SECRET` obrigatorio em ambiente production-like, ou exigir opt-in explicito para secret aleatorio.
2. Configurar pool de PostgreSQL por env vars (`max_conns`, `min_conns`, lifetime, idle time) e documentar sizing por replica.
3. Separar migrations como job de deploy ou adicionar advisory lock robusto.
4. Implementar idempotency key no `POST /api/v1/scans`, idealmente baseada em project key, branch/PR, commit SHA e hash do report.
5. Adicionar limite de jobs pendentes/rate limit por token ou projeto, com resposta 429 quando saturado.
6. Atualizar `docs/kubernetes.md` e `docs/architecture.md` para refletir o runtime atual com workers separados e jobs duraveis.

### P1 - Para operacao confiavel

1. Criar reconciler/janitor para jobs stale, com politica configuravel por tipo de job.
2. Adicionar metricas de idade do job mais antigo, tempo em fila, retries por tipo, dead-letter/failures e DB latency.
3. Melhorar readiness dos workers para checar dependencias reais.
4. Fazer paginação no banco para historico de scans e background tasks.
5. Tornar CORS configuravel e restritivo em producao.
6. Rever atomicidade da ingestao: transacao unica para persistencia principal ou reconciliacao clara para estados parciais.

### P2 - Limpeza e maturidade

1. Remover ou conectar `ollantaweb/breaker`.
2. Remover filas em memoria residuais ou documentar seu uso real.
3. Reduzir wrappers de compatibilidade apos a migracao para `application/ingest`.
4. Extrair metadados de regras para modulo sem CGO e eliminar copia em `ollantaweb`.
5. Padronizar logs restantes em `slog`.
6. Criar runbooks: jobs presos, reindexacao, restore de Postgres, restore/rebuild de search, rotacao de JWT/API tokens.

## Conclusao

O Ollanta tomou as decisoes grandes na direcao certa. A separacao de papeis operacionais, os jobs duraveis e a observabilidade basica mostram que o projeto ja saiu de uma arquitetura puramente local/monolitica para uma plataforma com desenho real de servidor.

Os riscos restantes nao invalidam a arquitetura; eles sao sinais de maturidade operacional pendente. O trabalho mais importante agora nao e adicionar mais abstracoes, e sim fechar os contratos distribuidos: idempotencia, limites, recuperacao automatica, tuning por ambiente, migrations coordenadas e documentacao fiel ao codigo.

Minha avaliacao final: **Ollanta e escalavel horizontalmente em principio e bom o bastante para ambientes pequenos/medios controlados. Para producao robusta e crescimento maior, precisa endurecer os pontos P0/P1 antes de depender dele como plataforma critica de qualidade.**