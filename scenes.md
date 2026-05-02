# scenes.md - Roteiro de Video: Ollanta em Acao

Roteiro tecnico para gravacao de video demonstrativo do Ollanta.
Formato: screen recording de terminal + browser, com narracao ou legendas.
Duracao estimada: 6-8 minutos.
Resolucao sugerida: 1920x1080, tema escuro no terminal e no browser.

---

## Cena 1 - Abertura (15s)

**Visual:** Um terminal em html animado simulando um console terminal de laptop executando o ollanta scanner. O texto aparece com um efeito de digitacao.

```
ollanta -project-dir . -project-key ollanta -format all -local-ui
```

**Legenda:**
> "Vamos rodar o Ollanta Scanner pela primeira vez. Com o flag `-local-ui`, o scanner analisa o codigo e abre automaticamente um dashboard local no browser - sem precisar configurar nada."

---

## Cena 2 - Apresentar o projeto-alvo (30s)

**Visual:** O browser abre automaticamente em `http://localhost:7777` com o dashboard do projeto "ollanta" carregado - direto apos o scan terminar:

![ollanta dashboard](docs/imgs/o01.png)

**Legenda:**
> "Este e o dashboard do projeto 'ollanta' com os resultados da analise. Aqui mostra que falhou no Quality Gate. Mostra tambem um overview geral, as principais issues, metricas. O desenvolvedor pode rodar na maquina e conferir localmente sem precisar esperar por uma operacao de CI/CD em pipeline para depois visualizar os resultados."

---

## Cena 3 - Explorar as issues (1m)

**Visual:** Clique na secao de Issues para mostrar a lista de issues detectadas, ordenadas por severidade. Mostre uma issue especifica (ex: "Large Function") e clique para abrir o detalhe da issue:

![issue detail](docs/imgs/o02.jpeg)

**Legenda:**
> "Aqui estao as issues detectadas, ordenadas por severidade. Vamos clicar em uma issue especifica para ver o detalhe. O Ollanta mostra a mensagem da issue, a localizacao exata no codigo, a regra que gerou a issue, e ate um trecho do codigo com destaque para a linha problematica. Isso ajuda o desenvolvedor a entender rapidamente o que esta errado e onde."

---

## Cena 4 - Subindo o servidor centralizado

**Visual:** De volta ao terminal. Mostrar o comando para subir o stack completo com Docker Compose:

```
docker compose --profile server up -d
```

Output esperado:

```
[+] Running 3/3
 OK Container ollanta-postgres-1    Healthy
 OK Container ollanta-zincsearch-1  Started
 OK Container ollanta-ollantaweb-1  Started
```

**Legenda:**
> "Para ter historico centralizado e acompanhar multiplos projetos, o Ollanta tem um servidor. Tres containers sobem com um unico comando: PostgreSQL para persistencia, ZincSearch para busca, e o ollantaweb que expoe a API REST na porta 8080."

---

## Cena 5 - Enviar resultados ao servidor (push) (45s)

**Visual:** De volta ao terminal. Rodar o scanner novamente, desta vez com os flags `-server` e `-server-token` para enviar os resultados ao servidor:

```
ollanta -project-dir . -project-key ollanta -server http://localhost:8080 -server-token ollanta-dev-scanner-token
```

Output no terminal ao final:

```
Server: gate=OK new=12 closed=0
```

**Legenda:**
> "Com um unico flag a mais - `-server` - o scanner envia o relatorio direto ao servidor apos o scan. O servidor processa, compara com scans anteriores, avalia o quality gate e retorna o resultado. Aqui vemos: gate OK, 12 issues novas, 0 fechadas - e o primeiro envio deste projeto."

---

## Cena 6 - Historico e tracking no servidor (45s)

**Visual:** No browser, navegue ate o servidor em `http://localhost:8080`. Mostre a lista de projetos, depois entre no projeto "ollanta" e mostre o historico de scans e a evolucao das metricas ao longo do tempo:

![server dashboard](docs/imgs/login.png)
![server dashboard](docs/imgs/projects.png)
![server dashboard](docs/imgs/server-dash.png)

**Legenda:**
> "No servidor, cada scan fica registrado. Da pra ver a evolucao do projeto ao longo do tempo - quantos bugs foram introduzidos, quantos foram corrigidos. O Ollanta rastreia cada issue por um hash do conteudo da linha: se a issue foi corrigida, ela e fechada automaticamente no proximo scan. Se foi movida de arquivo, e reconhecida como a mesma issue - nao abre duplicata."

---

## Cena 7 - Encerramento (15s)

**Visual:** Tela com o logo do Ollanta e os pontos principais em texto.

```
Ollanta

Scan local em milissegundos
Dashboard interativo na porta 7777
Servidor centralizado com historico
Tracking automatico de issues entre scans
Quality Gates configuraveis
Relatorios JSON + SARIF para CI/CD

github.com/scovl/ollanta
```

**Legenda:**
> "Ollanta - do scan ao dashboard, em menos de um segundo."