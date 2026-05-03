# Review da feature de testes unitarios e mutantes

## Veredito executivo

O Ollanta esta indo na direcao certa: a feature nao tenta reinventar runners de teste nem engines de mutacao; ela atua como uma camada de descoberta, normalizacao, diagnostico e exposicao de sinais de qualidade. Essa e uma boa escolha para uma plataforma estatica multi-linguagem, porque respeita o ecossistema de cada linguagem e permite que CI, pipelines e ferramentas maduras continuem fazendo o trabalho pesado.

O desenho atual ja e util para adocao controlada em projetos reais, especialmente no modo `collect`, usando relatorios ja produzidos pelo CI. Porem, eu ainda nao trataria esses sinais como base madura para bloqueio forte de qualidade sem antes corrigir algumas lacunas: execucao de comandos por shell, opcoes configuradas mas sem efeito pratico, parsers documentados alem do que o codigo realmente suporta, falta de timeout em comandos de teste unitario, path mapping permissivo e semantica de status de mutantes que pode distorcer score.

Minha leitura curta: **continue nessa direcao, mas endureca antes de transformar isso em politica obrigatoria**.

## O que a feature faz hoje

A implementacao esta espalhada principalmente nestes pontos:

- [application/scan/testsignals.go](application/scan/testsignals.go): modelos, opcoes, descoberta de modulos, defaults e coleta principal de sinais de teste.
- [application/scan/testsignals_collect.go](application/scan/testsignals_collect.go): execucao opcional de comandos e fallback bounded para achar relatorios.
- [application/scan/testsignals_parse.go](application/scan/testsignals_parse.go): parsing de JUnit, Go coverage, LCOV, XML tipo Cobertura e JSON nativo do Ollanta.
- [application/scan/testsignals_health.go](application/scan/testsignals_health.go): health score por modulo e por projeto, com expectativas por papel arquitetural.
- [application/scan/mutations.go](application/scan/mutations.go): discovery de ferramentas de mutacao, execucao opcional, coleta de relatorios e merge com test signals.
- [application/scan/mutations_parse.go](application/scan/mutations_parse.go): parsing nativo, Stryker JSON e PIT XML.
- [application/scan/usecase.go](application/scan/usecase.go): flags, composicao do scan e aplicacao de medidas no report.
- [ollantascanner/cmd/ollanta/config.go](ollantascanner/cmd/ollanta/config.go): leitura de configuracao TOML para testes e mutacoes.
- [application/ingest/usecase.go](application/ingest/usecase.go): ingestao e persistencia de metricas agregadas.
- [docs/test-signals.md](docs/test-signals.md): documentacao de uso, modos e formatos.

Hoje a feature trabalha com tres modos:

- `collect`: descobre modulos e coleta relatorios existentes, sem rodar comandos.
- `run`: permite executar comandos de teste/mutacao e depois coletar relatorios.
- `doctor`: mostra diagnosticos, comandos candidatos e paths esperados, sem executar comandos.

Ela tambem integra metricas de cobertura, testes, falhas, duracao, score de mutacao, mutantes mortos/sobreviventes e mutacao em codigo alterado ao report final do scanner e ao ingest do servidor.

## Pontos positivos

### 1. Report-first e execucao opt-in

A decisao mais importante e correta e o modo `collect` como caminho natural. Em vez de o Ollanta tentar controlar cada ecossistema, ele coleta artefatos ja gerados: JUnit, coverage, Stryker, PIT, relatorio nativo etc. Isso combina muito bem com CI moderno, containers efemeros e monorepos.

Tambem e positivo que a execucao nao aconteca por padrao. `doctor` ajuda o usuario a entender o que seria executado/coletado antes de liberar `run`. Essa escolha reduz surpresa operacional e evita que o scanner vire um runner opaco.

### 2. Modelo multi-modulo e multi-linguagem

A descoberta por `go.work`, workspaces JS/TS, `go.mod`, `package.json`, `pom.xml`, `pyproject.toml`, `Cargo.toml` e marcadores similares e uma boa base para monorepos. O modelo por modulo, com `root`, `language`, `architecture_role`, `owner`, `team` e thresholds, permite evoluir para qualidade por fronteira arquitetural, nao so por projeto inteiro.

Isso e especialmente bom para o Ollanta, porque o projeto em si e multi-modulo Go e ja tem uma arquitetura hexagonal. A feature nasceu olhando para um problema real do proprio produto.

### 3. Diagnosticos estruturados

O uso de `TestSignalDiagnostic` com `level`, `code`, `message`, `module` e `path` e uma boa escolha. Em vez de falhar silenciosamente ou imprimir texto solto, a feature consegue explicar coisas como modulo ignorado, relatorio stale, path ambiguo, comando nao executado, fallback encontrado e parser carregado.

Essa estrutura e o inicio certo para tornar a feature observavel. Ainda falta melhorar persistencia e exibicao desses diagnosticos, mas a forma dos dados esta boa.

### 4. Freshness, tamanho e limites de busca

`max_report_age`, `max_report_bytes`, `max_depth` e `max_candidates` sao controles importantes. Eles evitam que o scanner varra o repositorio inteiro sem limite, leia artefatos enormes ou use relatorios antigos sem avisar.

Marcar relatorios antigos como `stale` e transformar isso em perda de confianca e uma decisao sensata para adocao gradual. Nem todo time quer falhar o scan por um relatorio velho logo no primeiro dia.

### 5. Health arquitetural

O health por papel arquitetural e uma boa ideia. `domain` e `application` terem expectativas mais altas do que `adapter` e `infrastructure` faz sentido: codigo de regra de negocio deve ser mais testavel e mais deterministico, enquanto bordas externas tendem a exigir mais testes de integracao.

O suporte a `new_coverage_threshold` e `changed_code_score` tambem aponta para a direcao correta: qualidade deve melhorar principalmente no codigo alterado, nao exigir que um legado inteiro seja consertado de uma vez.

### 6. Formato nativo como escape hatch

O `ollanta-tests.json` e o `ollanta-mutations.json` sao excelentes escolhas. Eles permitem integrar ferramentas proprietarias, pipelines internos e linguagens ainda nao suportadas sem esperar um parser oficial.

Esse escape hatch reduz acoplamento e ajuda a plataforma amadurecer por contrato de dados, nao por suporte manual a cada ferramenta.

## Boas escolhas de produto e arquitetura

- **Nao construir uma engine de mutacao propria agora.** O Ollanta deve normalizar e avaliar sinais; Stryker, PIT, mutmut, Cosmic Ray e Infection ja resolvem a parte cara e especifica por linguagem.
- **Manter teste e mutacao como feature opcional.** Isso evita bloquear usuarios que ainda nao tem relatorios no CI.
- **Expor modos claros.** `collect`, `run` e `doctor` formam uma linguagem simples para operacao.
- **Separar discovery, collection, parsing e health.** A divisao dos arquivos esta compreensivel e facilita manutencao.
- **Guardar proveniencia dos relatorios.** `source_mode`, `freshness`, `age_ms` e `size_bytes` sao informacoes valiosas para confianca.
- **Priorizar changed-code mutation quando existir.** Essa e a direcao certa para evitar que mutacao vire uma meta impossivel em bases grandes.
- **Permitir path mapping.** Relatorios de CI frequentemente trazem paths de container, devcontainer ou workspace remoto; sem mapping, a cobertura por arquivo ficaria fraca.

## Pontos negativos e riscos

### 1. Execucao por shell e `command_policy` fraco

Os comandos de teste e mutacao sao executados via shell: `cmd /C` no Windows e `sh -c` em Unix. Isso e flexivel, mas tambem significa que qualquer string configurada tem semantica completa de shell: pipes, redirects, variaveis, encadeamento de comandos e efeitos colaterais.

O risco e parcialmente mitigado porque `run` e opt-in. Mesmo assim, `command_policy = "explicit"` hoje parece mais um campo documentado/serializado do que uma politica realmente aplicada. Nao ha validacao clara de valores, allowlist, bloqueio de comandos descobertos, nem distincao forte entre comando configurado e comando inferido.

Outro detalhe: `tests-mode=run` pode executar comandos candidatos de modulos descobertos, como `go test ./...`, `npm test`, `mvn test`, `pytest` ou `cargo test`. Isso e aceitavel se for intencional, mas a documentacao da a entender que apenas comandos configurados seriam executados.

Recomendacao: tratar `run` como modo confiavel apenas para repositorios confiaveis, validar `command_policy`, e considerar uma politica explicita como `configured_only`, `discovered_allowed`, `deny_shell_features` ou `allowlist`.

### 2. Testes unitarios nao tem timeout

Mutacao tem `max_runtime` e `fail_on_timeout`; teste unitario nao tem equivalente. `executeTestCommand` usa contexto cancelavel, mas sem deadline. Um `npm test` em watch mode, um teste travado ou um comando interativo pode segurar o scan indefinidamente.

Esse e um ponto de maturidade operacional importante. Se `run` existe, precisa de timeout para testes tambem.

### 3. Overrides por modulo de mutacao parecem nao ser aplicados na execucao

A configuracao aceita `max_runtime`, `max_mutants`, `changed_only`, `fail_on_timeout` e `changed_code_threshold` por modulo de mutacao. Porem, a execucao chama `executeMutationCommand` com as opcoes globais, nao com uma opcao efetiva por modulo.

Na pratica, isso sugere que parte da configuracao por modulo e apenas preservada no `scanner_options` ou no resumo, mas nao governa o comportamento real. Isso e perigoso porque o usuario acredita que limitou um modulo especifico, mas a execucao pode seguir o limite global.

Recomendacao: criar uma funcao de `effectiveMutationOptions(global, module)` e usar essa estrutura tanto na execucao quanto na coleta, no resumo e no health.

### 4. `changed_code_threshold` parece configuracao morta

`changed_code_threshold` e lido na configuracao e aparece no tipo de modulo de mutacao, mas o modelo de health usa `MutationThreshold` e nao preserva um campo separado para threshold de codigo alterado. O check de changed-code mutation compara `ChangedCodeScore` contra `MutationThreshold` quando ele existe.

Isso enfraquece uma das melhores ideias da feature. Changed-code mutation deveria ter threshold proprio e semantica propria.

Recomendacao: adicionar `ChangedMutationThreshold` ao modelo efetivo de modulo ou separar claramente `mutation_threshold` de `changed_mutation_threshold`.

### 5. `changed_only` e `max_mutants` ainda nao controlam ferramentas reais

Hoje `changed_only` e `max_mutants` aparecem no modelo e na configuracao, mas os comandos detectados sao strings genericas como `npx stryker run`, `mvn ... mutationCoverage`, `mutmut run` etc. Nao ha adaptador por ferramenta montando argumentos para limitar escopo ou quantidade de mutantes.

Isso nao invalida a feature, mas muda a promessa: por enquanto, changed-code mutation so e confiavel quando o relatorio nativo ou a ferramenta ja trouxe os campos `changed_*`. O scanner ainda nao consegue, sozinho, transformar `changed_only = true` em execucao limitada ao diff.

### 6. Parsers documentados acima da capacidade real

A documentacao e os caminhos default citam JaCoCo e `coverage.json`, mas o parser atual cobre Go coverage, LCOV, JUnit, JSON nativo e XML estilo Cobertura. JaCoCo XML tem estrutura propria, e `coverage.py` JSON tambem nao parece ser parseado no fluxo atual.

Isso pode gerar uma experiencia ruim: o usuario coloca um relatorio em um path documentado, o scanner coleta o arquivo, mas nao extrai cobertura. O resultado vira health parcial sem uma explicacao forte de que aquele formato ainda nao e suportado.

Recomendacao: ou implementar parsers reais para JaCoCo e coverage.py JSON, ou ajustar a documentacao para dizer claramente que esses paths sao candidatos, mas nem todos os formatos estao normalizados ainda.

### 7. Normalizacao de status de mutacao precisa revisao semantica

A normalizacao atual trata alguns status de forma discutivel. Um exemplo sensivel e `no-coverage`: em ferramentas de mutacao, isso normalmente significa que o mutante nao foi coberto por testes e deveria pesar como sobrevivente, nao como morto. Status como runtime error, non-viable, ignored, equivalent e no coverage precisam de uma matriz semantica por ferramenta.

Tambem e preciso revisar o denominador do score calculado. Mutantes equivalentes, ignorados ou nao viaveis normalmente nao deveriam entrar no mesmo denominador de mutantes testaveis. Se entrarem, o score pode ficar artificialmente baixo ou alto dependendo do caso.

Recomendacao: documentar a taxonomia interna de status e criar testes com exemplos reais de Stryker e PIT.

### 8. Path mapping e leitura de artefatos precisam de fronteira mais explicita

O path mapping e necessario, mas hoje e permissivo. Mappings sao aplicados por prefixo simples, e paths configurados absolutos podem apontar para fora do projeto. Isso pode ser util para artefatos externos de CI, mas tambem cria risco de confusao ou leitura acidental de arquivos fora do workspace.

Se o Ollanta quiser permitir artefatos externos, isso deveria ser uma decisao explicita: por exemplo `allow_external_artifacts = true`, com diagnostico claro quando um path esta fora do projeto. Se nao quiser, todo path normalizado deveria ser garantidamente relativo ao `project_dir`.

### 9. Erros de WalkDir e Scanner sao silenciosos

Os fallbacks de busca usam `filepath.WalkDir`, mas ignoram erros de caminhada. Os parsers baseados em scanner tambem nao parecem emitir diagnostico quando a leitura termina por erro. Isso reduz a confianca: ausencia de relatorio pode significar que nao havia relatorio, ou que houve erro de permissao, symlink quebrado, path inacessivel ou linha grande demais.

Como a feature tem uma boa estrutura de diagnosticos, esses erros deveriam virar `warn` com codigo proprio.

### 10. Output de comando e truncado sem sinalizacao

Stdout e stderr sao limitados a 16 KB, o que e correto para nao explodir o report. Mas nao ha `output_truncated` nem diagnostico avisando que a saida foi cortada. Em falhas reais, o trecho importante pode estar justamente depois do limite.

Recomendacao: adicionar `stdout_truncated`, `stderr_truncated` ou um campo unico `output_truncated` em `TestExecutionStatus`.

### 11. Health score e simples demais para decisoes fortes

O health por modulo e util, mas o score de projeto e uma media simples. Isso pode esconder risco em modulos criticos. Um modulo `domain` em risco deveria pesar mais que um modulo auxiliar ou gerado.

Tambem existe uma tensao entre `mutation-only` e `test health`: quando apenas mutacao esta habilitada, o modelo ainda penaliza ausencia de unit-test report e coverage. Pode ser desejavel, mas precisa estar documentado como decisao de produto; do contrario, o usuario pode achar estranho que uma coleta de mutacao gere health ruim por ausencia de JUnit.

### 12. Integracao ainda e dificil de provar

O modelo espera suites com `Kind = "integration"` para satisfazer exigencias de integracao em `adapter` e `infrastructure`. Mas o parser JUnit atual marca suites como `unit`. Mesmo que o path seja `integration-junit.xml` ou o nome da suite contenha `integration`, isso nao vira evidencia de integracao automaticamente.

Recomendacao: permitir `kind` por `test_reports`, inferir por path/nome com cuidado, ou exigir relatorio nativo quando o time quiser classificar suites.

### 13. Metricas zero somem no ingest

No ingest, varias metricas opcionais so viram linhas quando sao maiores que zero. Isso cria ambiguidade entre "medido e deu zero" e "nao medido". Para test failures, mutants survived, mutants skipped e mutants error, essa diferenca importa muito.

Exemplo: `mutants_survived = 0` e um bom sinal; se nao for persistido, dashboards e historico podem perder uma informacao positiva. O mesmo vale para `test_failures = 0`.

Recomendacao: quando `test_signals.summary.enabled = true`, persistir zeros explicitos para as metricas testaveis daquele scan.

## Escolhas ruins ou escolhas frageis

Aqui estao as escolhas que eu mudaria antes de promover a feature a "contrato de qualidade" do produto:

- `command_policy` existe sem enforcement claro. Campo de politica que nao governa comportamento gera falsa sensacao de seguranca.
- `run` executa comandos inferidos por discovery. Isso pode ser bom, mas precisa ser explicitamente assumido e protegido.
- Test command sem timeout. Isso e fraco para CI e para uso local.
- `changed_code_threshold`, `changed_only` e `max_mutants` aparecem como produto, mas ainda nao entregam todo o comportamento que prometem.
- JaCoCo e coverage.py JSON aparecem na experiencia/documentacao sem parser correspondente evidente.
- Status de mutacao sao normalizados de forma generica demais para ferramentas com semanticas diferentes.
- Score de mutacao calculado usa denominador simples; isso pode misturar mutantes testaveis com equivalentes, ignorados ou nao viaveis.
- Persistencia de metricas opcionais ignora zeros, que em qualidade de teste sao informacoes importantes.
- Path mapping permite flexibilidade sem deixar clara a fronteira de seguranca entre projeto e artefato externo.

## Lacunas de teste

Os testes existentes cobrem caminhos felizes importantes: discovery de modulos, override configurado, stale reports, doctor mode, execucao de Go test, mapping de paths, Stryker, native mutation, timeout de mutacao, changed-code mutation no health e fixture monorepo.

Eu adicionaria os seguintes testes antes de considerar a feature madura:

1. `tests-mode` e `mutations-mode` invalidos devem retornar erro de parse/validacao.
2. `command_policy` deve ser validado e deve alterar comportamento de execucao.
3. `tests-run` deve ter timeout e teste cobrindo timeout.
4. `mutations.modules.max_runtime` e `mutations.modules.fail_on_timeout` devem sobrepor o global em execucao real.
5. `changed_code_threshold` deve afetar health quando `changed_code_score` existe.
6. `changed_only` e `max_mutants` devem ter testes por ferramenta quando forem implementados.
7. Path mappings com prefixos parecidos, como `/workspace/app` e `/workspace/application`, devem ter boundary check.
8. Mappings ou report paths fora do projeto devem gerar diagnostico ou exigir permissao explicita.
9. Symlinks, paths `../` e paths absolutos externos devem ser cobertos.
10. WalkDir com erro de permissao deve produzir diagnostico.
11. Saida de comando acima de 16 KB deve marcar truncation.
12. JaCoCo XML real deve ser parseado ou rejeitado com diagnostico claro.
13. `coverage.py` JSON real deve ser parseado ou documentado como nao suportado.
14. Status reais de Stryker e PIT devem ter tabela de normalizacao testada.
15. Mutantes `no coverage`, equivalent, ignored, non-viable e runtime error devem afetar score corretamente.
16. Ingest deve persistir zeros explicitos quando sinais de teste/mutacao foram coletados.
17. Health de projeto deve ser testado com pesos por papel arquitetural, se essa melhoria for adotada.
18. Suites de integracao devem poder ser reconhecidas via config, path, nome ou formato nativo.

## Sugestoes de melhoria

### Curto prazo

1. Validar `tests-mode`, `mutations-mode`, `test_policy`, `mutation_policy` e `command_policy`.
2. Adicionar `tests.max_runtime` e `tests.fail_on_timeout`.
3. Implementar `effectiveMutationOptions` por modulo.
4. Fazer `changed_code_threshold` funcionar de verdade.
5. Marcar truncamento de stdout/stderr.
6. Emitir diagnosticos para erros de `WalkDir` e erros de scanner.
7. Corrigir ou documentar claramente suporte real a JaCoCo e `coverage.json`.
8. Rever a tabela de status de mutacao, principalmente `no-coverage`.

### Medio prazo

1. Criar adaptadores de comando por ferramenta de mutacao, em vez de strings soltas para tudo.
2. Separar politica de execucao: `never`, `configured_only`, `discovered`, `trusted_ci` ou equivalente.
3. Persistir metricas zero quando o sinal foi medido.
4. Tornar `test_signals` mais consultavel no servidor, nao apenas uma fonte para medidas agregadas.
5. Permitir classificacao de suites como `unit`, `integration`, `contract`, `e2e`.
6. Implementar health ponderado por papel arquitetural e criticidade.
7. Adicionar gate configuravel para `test_failures`, `coverage`, `new_code_coverage`, `mutation_score` e `changed_mutation_score`.

### Longo prazo

1. Calcular changed-code coverage/mutation a partir do snapshot/diff quando a ferramenta nao trouxer esses campos.
2. Exibir mutantes sobreviventes no servidor como itens acionaveis, com arquivo, linha, mutator e contexto.
3. Criar tendencias historicas de mutation score, survived mutants, flaky/failing tests e cobertura por modulo.
4. Suportar importacao SARIF-like ou formato nativo versionado para test/mutation evidence.
5. Criar um contrato estavel de `ollanta-tests.json` e `ollanta-mutations.json` com schema publicado.

## O que eu nao faria agora

- Nao construir uma engine propria de mutacao dentro do Ollanta.
- Nao executar comandos de teste/mutacao no servidor web.
- Nao tornar mutacao obrigatoria para todos os modulos por padrao.
- Nao tentar suportar todas as ferramentas por parsing manual antes de estabilizar o contrato nativo.
- Nao transformar health score em gate duro enquanto as semanticas de parsers, thresholds e stale reports ainda estiverem amadurecendo.

## Conclusao

A feature tem uma fundacao boa e uma intuicao correta: testes e mutacao devem entrar no Ollanta como sinais normalizados, rastreaveis e conectados a arquitetura, nao como uma execucao magica escondida dentro do scanner. O modo `collect`, o `doctor`, o formato nativo e o health por papel arquitetural sao escolhas fortes.

O risco principal e a distancia entre a promessa e o comportamento efetivo em alguns pontos: `command_policy`, `changed_code_threshold`, `changed_only`, `max_mutants`, JaCoCo/coverage.py JSON e overrides por modulo ainda precisam ser fechados. Tambem ha uma camada de seguranca operacional a endurecer em `run`.

Minha recomendacao e manter o investimento, mas com uma ordem clara: primeiro corrigir as ambiguidades e falsos contratos, depois ampliar parsers, depois transformar em gates de qualidade. Assim o Ollanta evita virar apenas mais um dashboard de metricas frageis e passa a oferecer evidencia de qualidade realmente confiavel.