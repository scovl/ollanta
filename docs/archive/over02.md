# over02 — Papéis Arquiteturais Separados para um Ollanta Escalável, Resiliente e Efêmero

> **ARCHIVED HISTORICAL DIAGNOSTIC** — This document described a future target architecture. The operational role separation envisioned here has since been implemented: `ollantaweb`, `ollantaworker`, `ollantaindexer`, `ollantawebhookworker` with durable PostgreSQL jobs. See `review.md` for current operational assessment. Moved to `docs/archive/` on 2026-05-07.

## Premissa

O Ollanta já tem uma boa separação entre dois mundos que fazem sentido:

- o **scanner** como agente de execução local ou de CI
- o **servidor** como ponto central de ingestão, consulta, governança e histórico

O problema restante não é mais a falta de modularização do código. O problema é que, no runtime do backend, **papéis operacionais demais continuam concentrados dentro do `ollantaweb`**.

Isso é aceitável para desenvolvimento local e para uma fase inicial de produto. Não é coerente, porém, com a proposta original já registrada no repositório para operação **cloud-native**, **escalável**, **resiliente** e **efêmera**, especialmente no que está declarado em [docs/kubernetes.md](docs/kubernetes.md).

O ponto central deste documento é simples:

> O Ollanta não precisa copiar a complexidade completa do SonarQube, mas precisa separar claramente os papéis operacionais que hoje ainda estão misturados dentro do mesmo processo.

---

## Tese principal

O debate correto não é “monólito vs distribuído” no nível do repositório.

O debate correto é:

- **quais papéis precisam ser independentes para escalar bem**
- **quais papéis precisam falhar sem derrubar os outros**
- **quais papéis precisam sobreviver à morte de pods sem perder trabalho**

Em outras palavras:

- o código pode continuar em um monorepo modular
- até alguns binários podem continuar compartilhando partes do código
- mas os **papéis operacionais** precisam ficar explícitos

Se os papéis continuam fundidos em um único processo, o sistema até funciona, mas deixa de honrar a promessa de ser realmente:

- escalável
- resiliente
- efêmero

---

## O que o Ollanta mistura hoje

Hoje o `ollantaweb` concentra, ao mesmo tempo, responsabilidades de:

1. **API pública e UI backend**
2. **ingestão síncrona de relatórios**
3. **compute engine de tracking, quality gate e measures**
4. **indexação assíncrona de busca**
5. **entrega de webhooks**
6. **bootstrap operacional do servidor**

Esses papéis estão visíveis no código atual:

| Papel atual | Onde aparece hoje | Observação |
|---|---|---|
| API pública, auth, leitura de projetos, issues e dashboards | [ollantaweb/api](ollantaweb/api), [ollantaweb/cmd/ollantaweb/main.go](ollantaweb/cmd/ollantaweb/main.go) | Papel correto para um serviço web |
| Ingestão do relatório em `POST /api/v1/scans` | [ollantaweb/api/scans.go](ollantaweb/api/scans.go) | O endpoint ainda dispara o fluxo principal de escrita |
| Orquestração de ingestão, tracking, gate e measures | [application/ingest/usecase.go](application/ingest/usecase.go) e [ollantaweb/ingest/pipeline.go](ollantaweb/ingest/pipeline.go) | Hoje isso é executado inline no caminho do request |
| Worker de indexação de busca | [ollantaweb/ingest/worker.go](ollantaweb/ingest/worker.go) | Usa fila in-process em memória |
| Dispatcher de webhook | [ollantaweb/webhook/dispatcher.go](ollantaweb/webhook/dispatcher.go) | Usa fila in-process em memória |
| Bootstrap, migrations, wiring, startup e shutdown | [ollantaweb/cmd/ollantaweb/main.go](ollantaweb/cmd/ollantaweb/main.go) | Mistura papel de aplicação com papel operacional |

O resultado é um backend que ainda se comporta mais como um **“superprocesso central”** do que como uma plataforma com papéis nitidamente separados.

---

## Onde isso entra em conflito com a proposta original

Em [docs/kubernetes.md](docs/kubernetes.md), o Ollanta se descreve com estes princípios:

- **Resilient**
- **Customizable**
- **Atomic**
- **Scalable**
- **Ephemeral**

Esses princípios têm implicações diretas.

### 1. Escalável

Para ser realmente escalável:

- o tráfego HTTP de leitura e autenticação não deve competir com processamento de ingestão pesada
- a ingestão de scans não deve escalar junto com a consulta de dashboards por obrigação
- busca e projeção de índices não devem obrigar cada réplica web a carregar responsabilidades de worker
- side effects externos, como webhooks, não devem dividir orçamento de CPU e falha com a API interativa

Hoje isso ainda está acoplado.

### 2. Resiliente

Para ser realmente resiliente:

- aceitar um scan deve gerar um handoff durável
- indexação não pode depender de uma fila local em memória como mecanismo principal
- webhook não pode depender de uma fila local em memória como mecanismo principal
- matar um pod não pode significar “talvez depois alguém faça um reindex manual”

Hoje a indexação e os webhooks ainda dependem de filas in-process em [ollantaweb/ingest/worker.go](ollantaweb/ingest/worker.go) e [ollantaweb/webhook/dispatcher.go](ollantaweb/webhook/dispatcher.go).

### 3. Efêmero

Para ser realmente efêmero:

- qualquer pod de aplicação precisa ser descartável a qualquer momento
- nenhum trabalho essencial pode depender do lifetime do processo local
- restart precisa ser uma operação segura, não um evento que arrisca perda de backlog

Hoje o runtime ainda depende de goroutines e canais locais para completar partes relevantes do fluxo.

Isso é bom para simplicidade inicial. Não é bom como arquitetura operacional alvo.

---

## A diferença correta entre SonarQube e Ollanta

O SonarQube separa com bastante nitidez:

- **Web**
- **Compute Engine**
- **Search layer**
- **Database**

O Ollanta não precisa replicar exatamente essa topologia. Mas precisa preservar a mesma ideia essencial:

> quem aceita comandos, quem processa trabalho pesado, quem materializa projeções e quem serve leitura não deveria viver como a mesma responsabilidade operacional.

Uma versão Ollanta dessa separação pode ser mais simples do que a do Sonar e ainda assim correta.

---

## Papéis que deveriam existir separados

O objetivo não é multiplicar serviços por moda. O objetivo é separar aquilo que tem:

- perfil de carga diferente
- perfil de falha diferente
- necessidade de retry diferente
- relação diferente com persistência e efemeridade

### 1. Scanner / Agent

**Responsabilidade**

- analisar código local ou em CI
- gerar `report.json` e `report.sarif`
- opcionalmente enviar o relatório ao servidor

**Natureza**

- totalmente efêmero
- stateless
- orientado a execução por job

**Status atual**

- esse papel já está relativamente bem separado no scanner

### 2. Intake API / Command API

**Responsabilidade**

- autenticar scanners e usuários
- aceitar `POST /api/v1/scans`
- validar o payload
- registrar a recepção do relatório
- enfileirar trabalho durável para processamento posterior

**O que não deveria fazer**

- tracking pesado inline
- quality gate inline
- persistência completa de issues e measures inline
- chamadas de rede externas como side effect crítico do request

**Por quê**

- esse papel precisa responder rápido
- esse papel precisa escalar com tráfego HTTP
- esse papel não deve carregar CPU de processamento de lote

**Status atual**

- hoje esse papel está fundido com compute, porque [ollantaweb/api/scans.go](ollantaweb/api/scans.go) ainda chama o pipeline principal diretamente

### 3. Compute Engine

**Responsabilidade**

- consumir jobs de scan aceitos pela API
- executar tracking de issues
- avaliar quality gate
- materializar scans, issues e measures canônicas no banco
- emitir jobs derivados para indexação e notificações

**Este é o papel que mais claramente falta como papel operacional separado.**

O código de negócio para esse papel já existe em grande parte em:

- [application/ingest/usecase.go](application/ingest/usecase.go)
- partes de `ollantaengine/`

O problema não é ausência de lógica. O problema é ausência de separação de runtime.

### 4. Search Projection Worker

**Responsabilidade**

- consumir jobs de indexação
- atualizar ZincSearch ou Postgres FTS
- reconstruir projeções de busca sem afetar a API pública

**O que este papel deve assumir**

- a busca é uma projeção secundária
- a fonte de verdade continua sendo Postgres
- rebuild completo precisa ser possível a qualquer momento

**Status atual**

- o worker existe, mas como fila local em [ollantaweb/ingest/worker.go](ollantaweb/ingest/worker.go)

**Problema**

- fila local não é handoff durável
- morte do pod pode perder backlog
- o papel ainda está acoplado à réplica web que recebeu o request

### 5. Webhook / Notification Worker

**Responsabilidade**

- consumir eventos publicados pelo write side
- entregar webhooks com retry, timeout e dead-letter
- isolar falhas de rede externa do fluxo principal do produto

**Status atual**

- a lógica já existe em [ollantaweb/webhook/dispatcher.go](ollantaweb/webhook/dispatcher.go)
- mas a execução ainda depende de canal em memória e goroutine do processo web

**Problema**

- isso viola o objetivo de efemeridade do runtime
- entrega externa não deveria depender da sobrevivência do processo HTTP

### 6. Query API / Dashboard API

**Responsabilidade**

- servir UI, dashboards, issues, trends, activity, badges e overview
- consultar Postgres e search backend
- responder bem a carga de leitura humana e integrações de consulta

**Status atual**

- esse papel já existe e está bem identificado em [ollantaweb/api](ollantaweb/api)

**Observação importante**

- em uma primeira fase, esse papel ainda pode continuar no mesmo binário da API de comandos
- mas arquiteturalmente ele precisa ser tratado como um papel diferente, porque o perfil de carga é diferente

### 7. Operações / Maintenance Job

**Responsabilidade**

- migrations
- reindex completo
- backfills de measures
- limpeza de jobs mortos ou DLQ
- manutenção periódica do sistema

**Status atual**

- migrations e bootstrap ainda vivem no startup de [ollantaweb/cmd/ollantaweb/main.go](ollantaweb/cmd/ollantaweb/main.go)

**Problema**

- isso é aceitável no começo
- em operação distribuída madura, vale mais a pena separar certas tarefas como jobs explícitos

---

## Topologia alvo recomendada para o Ollanta

O desenho abaixo é mais próximo do que o Ollanta deveria perseguir:

```text
                    +----------------------+
                    |  User / Browser / CI |
                    +----------+-----------+
                               |
                               v
                    +----------------------+
                    |  Intake / Query API  |
                    |  stateless           |
                    +----+-------------+---+
                         |             |
             reads       |             | writes commands
                         |             v
                         |      +------------------+
                         |      | Postgres         |
                         |      | source of truth  |
                         |      +---+----------+---+
                         |          |          |
                         |          |          |
                         |      scan jobs   outbox / index jobs
                         |          |          |
                         v          v          v
                  +-------------+  +------------------+  +------------------+
                  | Search      |  | Compute Engine   |  | Webhook Worker   |
                  | Zinc / FTS  |  | background role  |  | background role  |
                  +------+------+  +---------+--------+  +------------------+
                         ^                    |
                         |                    |
                         +--------------------+
                              emits projections
```

### Leitura correta desse desenho

- **Postgres** é a fonte de verdade do write side
- **search** é projeção reconstruível
- **Compute Engine** não atende usuário final; atende fila durável
- **Webhook Worker** é side effect separado
- **API** continua stateless e descartável

---

## Onde a separação precisa ser física e onde pode ser apenas lógica

Nem toda separação precisa virar serviço independente no primeiro dia.

### Separação que deveria ser física no alvo

- API / Web
- Compute Engine
- Search backend
- Webhook worker
- banco de dados

### Separação que pode começar lógica e virar física depois

- command API vs query API
- maintenance job vs processo principal

### Regra prática

Se o papel tem uma destas características, ele merece separação física mais cedo:

- consome backlog
- precisa retry durável
- sofre com chamadas externas lentas
- disputa CPU com tráfego interativo
- precisa escalar independentemente dos outros

---

## O que isso muda no contrato de ingestão

Hoje o modelo implícito é:

1. scanner envia relatório
2. servidor processa quase tudo inline
3. servidor responde `201` com o resultado final

Em uma arquitetura mais aderente aos princípios originais, o ideal seria:

1. scanner envia relatório
2. API valida e persiste um `report_receipt` ou `scan_job`
3. API responde `202 Accepted`
4. Compute Engine processa o job
5. UI e API consultam status e resultado depois

Esse contrato é mais coerente com:

- escala
- resiliência
- retries
- idempotência
- pods efêmeros

---

## O que não deveria ser separado agora

Separar papéis não é permissão para adicionar complexidade gratuita.

O Ollanta **não precisa**, agora, de:

- Kafka só por status arquitetural
- RabbitMQ só para “parecer distribuído”
- microserviços por domínio de negócio HTTP
- parser server-side separado
- search embedded e search externo ao mesmo tempo

Uma arquitetura correta para o Ollanta pode continuar simples se seguir esta ordem:

1. Postgres como fonte de verdade
2. filas duráveis via tabelas de job/outbox
3. workers separados por papel
4. search como projeção secundária
5. API stateless

---

## Sequência recomendada de evolução

### Fase 1 — separar responsabilidades sem explodir o sistema

- manter o scanner como está
- transformar ingestão em handoff durável
- introduzir `scan_jobs`
- mover o processamento pesado para um papel de Compute Engine
- transformar indexação em job durável
- transformar webhook em outbox durável

### Fase 2 — tornar o runtime realmente efêmero

- remover dependência de canais em memória para trabalho essencial
- garantir reprocessamento seguro após restart
- garantir idempotência por `project_key + fingerprint do report` ou equivalente
- reduzir o `ollantaweb` a API, autenticação e consulta

### Fase 3 — escalar por papel

- HPA para API web
- workers independentes para compute
- workers independentes para webhooks
- scaling separado do backend de busca

### Fase 4 — maturidade operacional

- DLQ explícita
- reindex job explícito
- maintenance jobs explícitos
- observabilidade por papel, não só por processo

---

## Mapeamento objetivo: atual → papel correto

| Peça atual | Papel correto no alvo |
|---|---|
| [ollantaweb/api/scans.go](ollantaweb/api/scans.go) | Intake API |
| [application/ingest/usecase.go](application/ingest/usecase.go) | Compute Engine |
| `ollantaengine/*` | Compute Engine |
| [ollantaweb/ingest/worker.go](ollantaweb/ingest/worker.go) | Search Projection Worker |
| [ollantaweb/webhook/dispatcher.go](ollantaweb/webhook/dispatcher.go) | Webhook Worker |
| [ollantaweb/api/overview.go](ollantaweb/api/overview.go) | Query API |
| [ollantaweb/api/activity.go](ollantaweb/api/activity.go) | Query API |
| [ollantaweb/api/measures.go](ollantaweb/api/measures.go) | Query API |
| [ollantaweb/api/issues.go](ollantaweb/api/issues.go) | Query API |
| [ollantaweb/cmd/ollantaweb/main.go](ollantaweb/cmd/ollantaweb/main.go) | bootstrap provisório; parte deve migrar para jobs operacionais |

---

## Conclusão

O problema do Ollanta não é “ser um monólito” no sentido raso da expressão.

O problema é outro:

> o runtime ainda concentra papéis que deveriam falhar, escalar e sobreviver de forma independente.

Se o Ollanta quiser honrar de forma séria sua proposta de arquitetura:

- escalável
- resiliente
- efêmera

então ele deveria convergir para esta divisão mínima de papéis:

1. **Scanner / Agent**
2. **Intake API / Command API**
3. **Compute Engine**
4. **Search Projection Worker**
5. **Webhook Worker**
6. **Query API**
7. **Maintenance Job**

Essa separação já é suficiente para o Ollanta ficar muito mais próximo de uma plataforma distribuída com papéis claros, sem precisar copiar toda a complexidade do SonarQube.

Esse é, na prática, o ponto arquitetural mais importante para o próximo estágio do produto.