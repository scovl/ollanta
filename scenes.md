# scenes.md â€” Roteiro de VÃ­deo: Ollanta em AÃ§Ã£o

Roteiro tÃ©cnico para gravaÃ§Ã£o de vÃ­deo demonstrativo do Ollanta.
Formato: screen recording de terminal + browser, com narraÃ§Ã£o ou legendas.
DuraÃ§Ã£o estimada: 6â€“8 minutos.
ResoluÃ§Ã£o sugerida: 1920Ã—1080, tema escuro no terminal e no browser.

---

## Cena 1 â€” Abertura (15s)

**Visual:** Um terminal em html animado simulando um console terminal de laptop executando o ollanta scanner. O texto aparece com um efeito de digitaÃ§Ã£o.

```
ollanta -project-dir . -project-key ollanta -format all -local-ui
```

**Legenda:**
> "Vamos rodar o Ollanta Scanner pela primeira vez. Com o flag `-local-ui`, o scanner analisa o cÃ³digo e abre automaticamente um dashboard local no browser â€” sem precisar configurar nada."

---

## Cena 2 â€” Apresentar o projeto-alvo (30s)

**Visual:** O browser abre automaticamente em `http://localhost:7777` com o dashboard do projeto "ollanta" carregado â€” direto apÃ³s o scan terminar:

![ollanta dashboard](docs/imgs/o01.png)

**Legenda:**
> "Este Ã© o dashboard do projeto 'ollanta' com os resultados da anÃ¡lise. Aqui mostra que falhou no Quality Gate. Mostra tambÃ©m um overview geral, as principais issues, mÃ©tricas. O desenvolvedor pode rodar na mÃ¡quina e conferir localmente sem precisar esperar por uma operaÃ§Ã£o de CI/CD em pipeline para depois visualizar os resultados."

---

## Cena 3 â€” Explorar as issues (1m)

**Visual:** Clique na seÃ§Ã£o de Issues para mostrar a lista de issues detectadas, ordenadas por severidade. Mostre uma issue especÃ­fica (ex: "Large Function") e clique para abrir o detalhe da issue:

![issue detail](docs/imgs/o02.jpeg)

**Legenda:**
> "Aqui estÃ£o as issues detectadas, ordenadas por severidade. Vamos clicar em uma issue especÃ­fica para ver o detalhe. O Ollanta mostra a mensagem da issue, a localizaÃ§Ã£o exata no cÃ³digo, a regra que gerou a issue, e atÃ© um trecho do cÃ³digo com destaque para a linha problemÃ¡tica. Isso ajuda o desenvolvedor a entender rapidamente o que estÃ¡ errado e onde."


---

## Cena 4 â€” Subindo o servidor centralizado

**Visual:** De volta ao terminal. Mostrar o comando para subir o stack completo com Docker Compose:

```
docker compose --profile server up -d
```

Output esperado:

```
[+] Running 3/3
 âœ” Container ollanta-postgres-1    Healthy
 âœ” Container ollanta-zincsearch-1  Started
 âœ” Container ollanta-ollantaweb-1  Started
```

**Legenda:**
> "Para ter histÃ³rico centralizado e acompanhar mÃºltiplos projetos, o Ollanta tem um servidor. TrÃªs containers sobem com um Ãºnico comando: PostgreSQL para persistÃªncia, ZincSearch para busca, e o ollantaweb que expÃµe a API REST na porta 8080."

---

## Cena 5 â€” Enviar resultados ao servidor (push) (45s)

**Visual:** De volta ao terminal. Rodar o scanner novamente, desta vez com os flags `-server` e `-server-token` para enviar os resultados ao servidor:

```
ollanta -project-dir . -project-key ollanta -server http://localhost:8080 -server-token ollanta-dev-scanner-token
```

Output no terminal ao final:

```
Server: gate=OK new=12 closed=0
```

**Legenda:**
> "Com um Ãºnico flag a mais â€” `-server` â€” o scanner envia o relatÃ³rio direto ao servidor apÃ³s o scan. O servidor processa, compara com scans anteriores, avalia o quality gate e retorna o resultado. Aqui vemos: gate OK, 12 issues novas, 0 fechadas â€” Ã© o primeiro envio deste projeto."

---

## Cena 6 â€” HistÃ³rico e tracking no servidor (45s)

**Visual:** No browser, navegue atÃ© o servidor em `http://localhost:8080`. Mostre a lista de projetos, depois entre no projeto "ollanta" e mostre o histÃ³rico de scans e a evoluÃ§Ã£o das mÃ©tricas ao longo do tempo:

![server dashboard](docs/imgs/login.png)
![server dashboard](docs/imgs/projects.png)
![server dashboard](docs/imgs/server-dash.png)

**Legenda:**
> "No servidor, cada scan fica registrado. DÃ¡ pra ver a evoluÃ§Ã£o do projeto ao longo do tempo â€” quantos bugs foram introduzidos, quantos foram corrigidos. O Ollanta rastreia cada issue por um hash do conteÃºdo da linha: se a issue foi corrigida, ela Ã© fechada automaticamente no prÃ³ximo scan. Se foi movida de arquivo, Ã© reconhecida como a mesma issue â€” nÃ£o abre duplicata."

---

## Cena 7 â€” Encerramento (15s)

**Visual:** Tela com o logo do Ollanta e os pontos principais em texto.

```
â—† Ollanta

Scan local em millisegundos
Dashboard interativo na porta 7777
Servidor centralizado com histÃ³rico
Tracking automÃ¡tico de issues entre scans
Quality Gates configurÃ¡veis
RelatÃ³rios JSON + SARIF para CI/CD

github.com/scovl/ollanta
```

**Legenda:**
> "Ollanta â€” do scan ao dashboard, em menos de um segundo."