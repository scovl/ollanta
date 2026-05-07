# Análise de foco, boilerplate e over-engineering no Ollanta

> **ARCHIVED HISTORICAL DIAGNOSTIC** — This document was written as a snapshot of a previous repository state. The main structural issues identified here (dual backend tracks, legacy adapter stack, pgnotify) have since been resolved. See `openspec/changes/` for current state. Moved to `docs/archive/` on 2026-05-07.

## Nota de status

Este documento foi escrito como diagnóstico do estado anterior do repositório. Depois dele, os principais pontos estruturais destacados aqui foram corrigidos no código:

- o runtime do scanner convergiu para `application/scan`
- a stack de servidor legada em `adapter/` foi removida, mantendo apenas os bridges ativos de parser e regras
- o pipeline de ingestão do `ollantaweb` passou a delegar para `application/ingest`
- `pgnotify` e `OLLANTA_INDEX_COORDINATOR` foram removidos; a indexação agora usa apenas o worker in-process atual
- a suíte de validação descrita em `CLAUDE.md` voltou a passar após essas mudanças

O restante do arquivo permanece como registro do diagnóstico original que motivou essa limpeza.

## Resumo executivo

O Ollanta **não está perdido no núcleo do produto**. O foco central continua claro: reduzir o tempo entre detectar e corrigir problemas de qualidade de código, com feedback local, imediato e acionável para o desenvolvedor. No Ollanta, o scanner local não é só parte do pacote; ele é peça central da proposta. Regras, relatórios, quality gates e servidor centralizado entram para ampliar esse fluxo, não para substituir a correção rápida na máquina do dev. Essa linha aparece com consistência em `ollantascanner/`, `ollantarules/`, `ollantaparser/`, `ollantaengine/` e também na documentação principal.

O ponto em que o projeto começa a se perder é **na arquitetura do servidor e na convivência entre a trilha hexagonal em migração e a trilha operacional atual do backend**. Hoje existem **duas trilhas backend em paralelo**, com responsabilidades muito parecidas:

- a trilha hexagonal em migração: `domain/`, `application/`, `adapter/`
- a trilha operacional atual do servidor: `ollantaweb/`, `ollantastore/`, ainda apoiada em partes do fluxo por módulos legados como `ollantacore/` e `ollantaengine/`

Isso gera três efeitos ruins ao mesmo tempo:

1. **custo cognitivo alto** para entender qual trilha é a principal
2. **duplicação real de código e conceitos**
3. **abstrações criadas antes da migração terminar**, o que é o principal sinal de over-engineering aqui

Em resumo: o produto não perdeu o problema que quer resolver, mas a implementação **está espalhando energia entre a trilha hexagonal em migração e a trilha operacional atual do backend**.

---

## Veredito rápido

### Onde o projeto está forte

- `ollantascanner/`, `ollantarules/`, `ollantaparser/` e `ollantaengine/` formam um núcleo relativamente coerente
- a separação de parser CGo em `ollantaparser/` faz sentido técnico
- a distinção entre scanner local e servidor centralizado é boa e clara
- a documentação comunica bem a proposta do produto

### Onde o projeto está se perdendo

- coexistência prolongada entre a trilha hexagonal `adapter + application + domain` e a trilha operacional atual `ollantaweb + ollantastore`
- duplicação de handlers, repositórios, pipelines e modelos
- partes inteiras da arquitetura hexagonal já modeladas, mas ainda sem adoção real pelo runtime principal
- acúmulo de ativos de produto, docs, demo, storytelling e material de apresentação no mesmo repositório raiz

### Onde há boilerplate

- repositórios CRUD por agregado em duas trilhas backend diferentes
- handlers HTTP muito semelhantes em duas trilhas backend diferentes
- structs de dependência gigantes (`RouterDeps`) e wiring manual extenso
- duplicação de docs e conteúdo operacional em múltiplas versões

### Onde há over-engineering

- modelar uma arquitetura hexagonal completa antes de consolidar a migração
- manter duas implementações operacionais do servidor ao mesmo tempo
- criar pontos de extensão e estratégias alternativas antes de provar a necessidade
- adicionar mecanismos de coordenação e resiliência mais sofisticados em áreas ainda não estabilizadas

---

## 1. O principal problema: duas trilhas backend em paralelo

O maior sinal de desvio não está no scanner, e sim no backend do servidor.

Hoje existem duas trilhas quase espelhadas:

- `adapter/cmd/web/main.go`
- `adapter/primary/http/router.go`
- `adapter/secondary/postgres/*.go`
- `application/ingest/usecase.go`

e, em paralelo:

- `ollantaweb/cmd/ollantaweb/main.go`
- `ollantaweb/api/router.go`
- `ollantastore/postgres/*.go`
- `ollantaweb/ingest/pipeline.go`

Essa duplicação não é apenas conceitual. Ela já aparece na implementação.

### Evidências concretas

#### 1.1 Main de servidor duplicado

Os arquivos abaixo fazem praticamente o mesmo papel de composição do servidor:

- `adapter/cmd/web/main.go`
- `ollantaweb/cmd/ollantaweb/main.go`

Ambos carregam config, inicializam banco, search, dispatcher, pipeline e router. O segundo está mais evoluído, mas a existência do primeiro continua cobrando manutenção mental e técnica.

#### 1.2 Router duplicado

Os arquivos abaixo são claramente parentes estruturais:

- `adapter/primary/http/router.go`
- `ollantaweb/api/router.go`

O router de `ollantaweb` evoluiu mais e ganhou mais endpoints, mas o de `adapter` ainda existe como uma versão paralela do mesmo produto. Isso não é “variação saudável”; é uma migração incompleta que já virou superfície permanente.

#### 1.3 Repositórios duplicados

Exemplo direto:

- `adapter/secondary/postgres/projects.go`
- `ollantastore/postgres/projects.go`

Os dois definem `ProjectRepository`, `NewProjectRepository`, `Upsert`, `Create`, `GetByKey`, `GetByID`, `Delete` e variações de `List`. A diferença principal é o tipo usado (`domain/model.Project` de um lado, `ollantastore/postgres.Project` do outro), não a responsabilidade.

Esse mesmo padrão aparece para `IssueRepository`, `ScanRepository`, `MeasureRepository`, `UserRepository`, `TokenRepository`, `ProfileRepository`, `GateRepository`, `WebhookRepository` e outros.

#### 1.4 Pipeline de ingestão duplicado

Os arquivos abaixo são quase o mesmo caso de uso em dois mundos diferentes:

- `application/ingest/usecase.go`
- `ollantaweb/ingest/pipeline.go`

Ambos:

- fazem upsert de projeto
- buscam scan anterior
- rodam tracking
- avaliam quality gate
- persistem scan
- inserem issues
- inserem measures
- disparam indexação assíncrona

No arquivo de `ollantaweb` ainda aparecem complexidades adicionais, como breaker e coordenação de indexação, mas o fluxo base continua essencialmente o mesmo.

### Por que isso é grave

Porque essa duplicação não está confinada a uma função ou util. Ela está no **nível arquitetural**. Isso aumenta:

- custo de mudança
- risco de divergência comportamental
- dificuldade de onboarding
- atrito para decidir “onde implementar a próxima coisa”

Esse é o ponto mais claro em que o projeto está se perdendo.

---

## 2. A camada hexagonal existe, mas ainda não virou o centro do sistema

O repositório comunica uma intenção forte de migrar para a arquitetura hexagonal baseada em:

- `domain/`
- `application/`
- `adapter/`

Só que a adoção prática ainda é parcial.

### Evidências concretas

#### 2.1 `application/scan` não tem uso real

As buscas por imports de `github.com/scovl/ollanta/application/scan` e `github.com/scovl/ollanta/application/analysis` não retornam consumidores fora do próprio módulo.

Na prática:

- `application/scan/usecase.go` modela uma orquestração nova
- `ollantascanner/scan/scan.go` continua sendo a implementação realmente usada pelo scanner

Ou seja: existe uma versão arquiteturalmente “certa”, mas quem roda é a versão legada/operacional.

#### 2.2 `application/ingest` só aparece no caminho `adapter`

As referências de `application/ingest` aparecem basicamente em:

- `adapter/cmd/web/main.go`
- `adapter/primary/http/router.go`
- `adapter/primary/http/health.go`
- `adapter/primary/http/scans.go`

Enquanto isso, o servidor principal segue com:

- `ollantaweb/ingest/pipeline.go`

Isso mostra que a trilha hexagonal foi desenhada e já existe no repositório, mas ainda não ganhou o caminho principal do backend operacional.

### Por que isso é over-engineering

Porque a base já está pagando o custo de:

- novos modelos
- novas portas
- novos adapters
- novas convenções

sem ainda colher o benefício principal: **um runtime consolidado em cima dessa arquitetura**.

Em outras palavras, parte da abstração foi comprada antes de a migração ter sido concluída.

---

## 3. Boilerplate estrutural demais na camada de servidor

Há boilerplate saudável e há boilerplate que já indica atrito sistêmico. Aqui há os dois.

### Boilerplate aceitável

É normal ter:

- repositórios separados por agregado
- handlers separados por domínio HTTP
- wiring explícito de dependências em Go

Isso, por si só, não é um problema.

### Boilerplate excessivo

O excesso aparece quando a mesma estrutura se repete em duas trilhas backend.

#### 3.1 `RouterDeps` gigante em duplicidade

Arquivos:

- `adapter/primary/http/router.go`
- `ollantaweb/api/router.go`

Ambos têm structs `RouterDeps` grandes, com muitos campos de repositório, search, pipeline, telemetry, gates, profiles, periods, webhooks, etc. Esse wiring manual já seria aceitável se existisse **uma única trilha backend**. Em duas trilhas paralelas, vira boilerplate caro.

#### 3.2 CRUD handlers muito próximos entre si

Exemplo claro:

- `adapter/primary/http/projects.go`
- `ollantaweb/api/projects.go`

O mesmo padrão se repete em scans, groups, gates, profiles, permissions, users, webhooks e outros.

O problema aqui não é “ter handlers”; é ter handlers equivalentes duplicados, mudando apenas o tipo de dado e alguns endpoints extras.

#### 3.3 utilitários de pipeline repetidos

Arquivos:

- `application/ingest/steps.go`
- `ollantaweb/ingest/steps.go`

Esse tipo de repetição é típico de migração parada no meio: a abstração nova existe, mas a antiga ainda continua viva, então o projeto passa a manter duas infraestruturas de orquestração.

---

## 4. Over-engineering no servidor: mecanismos avançados antes da convergência

No `ollantaweb`, há sinais de sofisticação técnica que não seriam problema em um backend estabilizado, mas aqui aparecem antes da consolidação arquitetural.

### 4.1 Coordenação de indexação com múltiplas estratégias

Arquivos relevantes:

- `ollantaweb/config/config.go`
- `ollantaweb/ingest/worker.go`
- `ollantaweb/pgnotify/coordinator.go`
- `ollantaweb/cmd/ollantaweb/main.go`

Hoje o servidor suporta pelo menos duas formas de coordenar indexação:

- `memory`
- `pgnotify`

Isso é uma boa ideia para deploy distribuído, mas eleva bastante a complexidade operacional e mental. Num projeto ainda definindo qual trilha backend vai sobreviver como principal, isso parece antecipação demais.

### 4.2 Breakers no pipeline antes de o desenho estabilizar

Arquivo relevante:

- `ollantaweb/ingest/pipeline.go`

O pipeline mantém `dbBreaker` e `msBreaker`. Só que, no arquivo analisado, `msBreaker` sequer aparece sendo usado no fluxo mostrado. Isso é um sinal clássico de mecanismo introduzido cedo demais ou deixado pela metade.

Não é um problema isolado, mas mostra tendência: adicionar resiliência e opção de coordenação antes de reduzir a duplicação estrutural principal.

### 4.3 Conclusão desse ponto

Não é que breaker, fila assíncrona ou `pgnotify` sejam ruins. O problema é a **ordem de investimento**:

- primeiro o projeto precisava convergir para uma trilha principal
- depois faria sentido sofisticar coordenação distribuída

Hoje parece que as duas coisas estão andando juntas, e isso dispersa foco.

---

## 5. Duplicação de tipos e dados: aqui existe dívida controlada e dívida perigosa

Esse item merece nuance.

### 5.1 Duplicação perigosa

É a duplicação de modelos e repositórios em trilhas paralelas:

- `domain/model.Project` versus `ollantastore/postgres.Project`
- `application/ingest/usecase.go` versus `ollantaweb/ingest/pipeline.go`
- `adapter/secondary/postgres/*` versus `ollantastore/postgres/*`

Essa é a duplicação mais cara, porque afeta comportamento.

### 5.2 Duplicação conhecida e parcialmente justificada

A duplicação dos JSONs de regras em:

- `ollantarules/languages/*/rules/*.json`
- `ollantaweb/api/rules_data/*.json`

é uma dívida conhecida por causa da fronteira de CGo. O próprio projeto já documenta isso.

Ainda assim, continua sendo um ponto de atrito, porque toda regra nova ou alteração de metadado depende de atualização em dois lugares.

Então o diagnóstico aqui é:

- **duplicação tolerável como dívida técnica temporária**: metadados de regras para preservar `CGO_ENABLED=0` no servidor
- **duplicação ruim e estrutural**: handlers, repositórios, pipelines e modelos em trilhas paralelas

---

## 6. Logging inconsistente com os guardrails do próprio projeto

Os guardrails dizem para privilegiar `slog` com contexto estruturado. Na prática, o código ainda usa bastante:

- `log.Printf`
- `fmt.Printf`

Exemplos:

- `ollantaweb/webhook/dispatcher.go`
- `ollantaweb/pgnotify/coordinator.go`
- `ollantaweb/ingest/worker.go`
- `adapter/secondary/webhook/dispatcher.go`
- `adapter/secondary/search/worker.go`
- `ollantascanner/executor/executor.go`
- `ollantascanner/report/builder.go`

Isso não é o maior problema do projeto, mas mostra um desalinhamento entre arquitetura/documentação e prática real. Em times pequenos, esse tipo de divergência costuma ser sintoma de código andando mais rápido que as convenções.

---

## 7. O repositório raiz está acumulando funções demais

No topo do repositório convivem:

- produto
- documentação técnica
- OpenSpec
- drafts
- demo app (`OllantaDemoApp.jsx`)
- roteiro (`scenes.md`)
- imagens de branding
- apresentação (`presentation/` no workspace, mesmo que hoje ignorada)

Isso não é um erro grave por si só. Projetos open source em fase inicial frequentemente concentram tudo em um lugar. Mas aqui já existe um efeito colateral claro: **o repositório mistura código de produto com material de narrativa e marketing**.

O risco não é técnico direto; é de foco. Fica mais difícil perceber rapidamente:

- o que é código operacional
- o que é material de apoio
- o que é experimento
- o que é dívida de migração

Se isso crescer sem curadoria, a raiz do repositório começa a contar histórias demais ao mesmo tempo.

---

## 8. Há documentação paralela demais?

Aqui o problema não é excesso bruto de documentação. O problema é **superposição**.

Exemplos:

- `CONTRIBUTIONS.md` e `docs/contributing.md`
- `docs/architecture.md` e `docs/arquitetura.md`

No caso da arquitetura em inglês e português, isso pode ser ótimo para alcance, mas dobra o custo de manutenção.

No caso de contribuição, a versão curta e a longa fazem sentido, desde que continuem rigidamente sincronizadas. Caso contrário, rapidamente viram boilerplate documental.

Então o diagnóstico aqui é:

- **não é over-engineering grave**
- **já é custo de manutenção adicional**
- precisa de disciplina para não virar informação divergente

---

## 9. O que NÃO parece over-engineering

É importante separar crítica estrutural de complexidade legítima.

### 9.1 Separar parser CGo do restante

`ollantaparser/` como fronteira de CGo faz total sentido. Isso reduz contaminação do restante da base e protege builds sem CGo.

### 9.2 Separar scanner de servidor

Também faz sentido. Scanner local e servidor centralizado têm responsabilidades diferentes e ritmos diferentes.

### 9.3 Manter regras em JSON + código

Para um analisador estático, isso é razoável. Metadado declarativo com lógica de regra em Go é um desenho claro.

### 9.4 Módulos específicos por domínio técnico

`ollantarules/`, `ollantaparser/`, `ollantascanner/`, `ollantaengine/` ainda parecem uma decomposição saudável.

Ou seja: o projeto não está complicado “porque tem muitos módulos”. Ele está complicado porque **alguns módulos representam uma migração incompleta e duplicada**.

---

## 10. Diagnóstico consolidado

Se eu tivesse que resumir o estado atual em uma frase:

> O Ollanta não está se perdendo no produto; está se perdendo na convivência prolongada entre a trilha hexagonal em migração e a trilha operacional atual do backend.

O scanner e o core analítico ainda contam uma história relativamente coesa.

Já o backend conta duas histórias ao mesmo tempo:

- a história do backend que existe e roda hoje
- a história da arquitetura hexagonal que está sendo preparada para absorver esse backend

Enquanto essas duas histórias coexistirem sem convergência clara, o projeto continuará pagando:

- boilerplate em dobro
- manutenção em dobro
- onboarding mais lento
- decisões mais caras

---

## 11. Prioridades recomendadas

### Prioridade 1: escolher a trilha backend oficial

Decidir explicitamente qual caminho vai sobreviver como backend principal:

- ou a trilha hexagonal `adapter + application + domain`
- ou a trilha operacional atual `ollantaweb + ollantastore`, com uma migração mais tardia dos componentes legados ao redor

Sem isso, cada nova feature de servidor tende a aumentar a duplicação.

### Prioridade 2: parar de expandir as duas stacks ao mesmo tempo

Se a trilha operacional atual ainda precisa viver, ela deveria entrar em modo de manutenção mínima. Se a trilha hexagonal ainda não está pronta para absorver o backend principal, o ideal é migrar por fatias reais, não por espelhamento de tudo.

### Prioridade 3: eliminar duplicações de alto custo primeiro

Em ordem:

1. pipeline de ingestão
2. repositórios postgres duplicados
3. handlers HTTP duplicados
4. wiring de servidor duplicado

### Prioridade 4: só depois refinar sofisticação operacional

Depois de convergir backend, aí sim faz mais sentido investir mais fundo em:

- coordenação distribuída de indexação
- breakers
- resiliência avançada
- variações de backend/search orchestration

### Prioridade 5: limpar a narrativa do repositório raiz

Não precisa remover tudo. Basta deixar mais claro o que é:

- código de produto
- documentação oficial
- material de demo
- artefatos de exploração

---

## 12. Conclusão final

### O Ollanta está se perdendo?

**Parcialmente, sim.**

Mas não no objetivo do produto.

Ele está se perdendo **na forma de organizar a evolução do backend**, porque a migração arquitetural criou uma trilha hexagonal paralela antes de absorver ou aposentar de forma clara a trilha operacional atual do servidor.

### Há boilerplate?

**Sim, bastante**, principalmente no servidor. O boilerplate mais caro não é o número de arquivos; é a repetição de estruturas equivalentes em duas trilhas backend.

### Há over-engineering?

**Sim, em pontos específicos**:

- arquitetura hexagonal extensa sem adoção consolidada no backend principal
- duas trilhas backend convivendo por tempo demais
- coordenação distribuída e resiliência sofisticada antes da convergência estrutural

### O projeto ainda tem um caminho bom?

**Sim.**

O núcleo do produto é bom e reconhecível. O problema parece muito mais de **disciplina de convergência** do que de direção estratégica.

Se o projeto reduzir a duplicação arquitetural e parar de carregar duas narrativas backend ao mesmo tempo, ele tende a ficar bem mais simples, mais rápido de evoluir e mais fácil de explicar.