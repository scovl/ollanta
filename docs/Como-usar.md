# Como Usar o Ollanta

Este guia cobre a primeira jornada de uso: escanear um projeto real, enviar o resultado para o servidor, inspecionar os achados e decidir o que configurar em seguida.

Os exemplos usam o próprio checkout do Ollanta como projeto alvo, porque esse caminho exercita o mesmo fluxo que uma equipe nova usará com o seu próprio repositório.

## 1. Escolha Um Modo

Use o modo somente scanner quando você quiser um relatório local rápido ou um artefato de CI:

- A CLI grava `.ollanta/report.json` e `.ollanta/report.sarif`.
- A UI local opcional roda em `http://localhost:7777`.
- Não é necessário banco de dados nem servidor.

Use o modo servidor quando você quiser histórico, rastreamento de issues, Quality Gates, perfis, dashboards, regras customizadas, configuração de provedores de IA ou acesso em equipe:

- `ollantaweb` roda em `http://localhost:8080`.
- PostgreSQL armazena relatórios, issues, perfis, gates, usuários, jobs e metadados de projeto.
- ZincSearch fornece busca, a menos que o deploy use busca via PostgreSQL.
- Workers em background processam ingestão de scans, indexação e entrega de webhooks.

## 2. Rode o Primeiro Scan Local

A partir da raiz do repositório:

```sh
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . \
  -project-key ollanta-self \
  -format all \
  -local-ui
```

Abra `http://localhost:7777` e inspecione o overview, as issues, os detalhes das regras, as métricas e os relatórios gerados em `.ollanta/`.

O modo scanner via Docker faz a mesma coisa sem exigir uma toolchain Go local:

```sh
docker compose --profile scanner up local-ui
```

Para outro projeto, defina `PROJECT_DIR` e `PROJECT_KEY` antes de rodar o comando.

Exemplo em shell:

```sh
export PROJECT_DIR='D:\projects\myapp'
export PROJECT_KEY='myapp'
docker compose --profile scanner up local-ui
```

## 3. Suba o Servidor

Inicie a stack local do servidor:

```sh
docker compose --profile server up -d --build --wait
```

Abra `http://localhost:8080` e entre com a conta local de desenvolvimento:

- Login: `admin`
- Senha: `admin`

A stack local do Compose funciona sem arquivo `.env`. Ela usa defaults de desenvolvimento para PostgreSQL, ZincSearch, segredo JWT e token do scanner. Para ambientes compartilhados ou duradouros, crie um `.env` local e sobrescreva pelo menos `PG_PASSWORD`, `OLLANTA_JWT_SECRET` e `OLLANTA_SCANNER_TOKEN`.

Para ambientes de alto throughput, configure o pool de workers:

```bash
# 16 goroutines para 10k+ projetos
OLLANTA_WORKER_POOL=16 docker compose --profile server up -d
```

## 4. Envie Um Scan Para o Servidor

Com o servidor rodando, envie o checkout atual pela CLI local:

```sh
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . \
  -project-key ollanta-self \
  -format all \
  -server http://localhost:8080 \
  -server-token ollanta-dev-scanner-token \
  -server-wait
```

O modo de push containerizado é o caminho Docker mais amigável para uso inicial:

```sh
docker compose --profile push run --build --rm push
```

Defina estes valores ao escanear outro projeto ou quando quiser que o container de push aguarde o processamento no servidor:

```sh
export PROJECT_DIR='D:\projects\myapp'
export PROJECT_KEY='myapp'
export OLLANTA_SERVER_WAIT='true'
docker compose --profile push run --build --rm push
```

Quando `server wait` está habilitado, um push concluído ainda pode sair com código diferente de zero se o Quality Gate avaliado for `ERROR`. Nesse caso, o scan foi aceito e processado; o código diferente de zero é o sinal de CI de que o projeto não passou no gate configurado.

### Códigos de Saída do Scanner

| Código | Significado |
|--------|-------------|
| `0` | Sucesso ou `-skip` ativado |
| `1` | Erro interno (falha de I/O, falha de parse) |
| `2` | Erro de usuário (flag inválida, config ruim) |
| `3` | Quality Gate `ERROR` (scan concluiu, gate falhou) |

Use `$? -ne 0` em scripts de CI para capturar qualquer falha, ou `$? -eq 3` para detectar apenas falhas de gate.

### Suporte a Proxy

Para ambientes atrás de proxy corporativo, use a flag `-proxy` ou o campo `proxy` no `config.toml`:

```sh
ollanta -server http://ollanta.example.com -proxy http://corp-proxy:3128
```

Alternativamente, defina as variáveis de ambiente `HTTP_PROXY` / `HTTPS_PROXY` — o scanner as respeita automaticamente.

### Pular Scan

Use `-skip` para sair imediatamente sem análise:

```sh
ollanta -skip
```

Útil em pipelines CI onde o scan é condicionalmente desabilitado. O campo `skip` no `config.toml` também funciona:

```toml
[scanner]
skip = true
```

### Interpolação de Variáveis no Config

O `config.toml` suporta placeholders `${VAR}`, `${env.VAR}` e `${env.VAR:-default}`:

```toml
[scanner]
server_url    = "${env.OLLANTA_URL:-http://localhost:8080}"
server_token  = "${env.OLLANTA_TOKEN}"
project_key   = "myapp-${env.BRANCH:-main}"
```

Use `$$` para um cifrão literal.

### Arquivo de Config Global

Coloque configurações compartilhadas em `~/.ollanta/config.toml` (`%USERPROFILE%\.ollanta\config.toml` no Windows):

```toml
[scanner]
server_url = "https://ollanta.example.com"
proxy      = "http://corp-proxy:3128"
```

Valores do `config.toml` por projeto sobrescrevem os globais. Use `-global-config /caminho/global.toml` para um caminho customizado, ou `-global-config ""` para desabilitar.

### Versão

```sh
ollanta --version
# Ollanta Scanner 0.2.0
```

Para validações locais repetidas depois que a imagem do scanner já existe, omita `--build` para verificar apenas o comportamento de runtime sem reconstruir a imagem.

## 5. Inspecione o Resultado

Na UI do servidor, abra o projeto e verifique estas abas:

- Overview: status do gate mais recente, tendências de issues, distribuições, hotspots e detalhes do projeto.
- Issues: filtros por severidade, tipo, qualidade, ciclo de vida, linguagem, regra, tag, diretório ou arquivo.
- Coverage: navegação pelo snapshot de código armazenado com as issues correspondentes.
- Activity: comparação de scans ao longo do tempo.
- Scopes: troca entre branches e pull requests.
- Quality Gate: revisão das condições de aprovação e falha.
- Profiles: escolha de quais regras rodam para cada linguagem.
- Rule Studio: criação e publicação de Custom Rule Packs.
- AI Providers: conexão de modelos locais ou cloud para geração de rascunhos no Rule Studio.

## 6. Configure Só o Necessário

O `config.toml.example` da raiz é intencionalmente um ponto de partida, não uma referência completa. Copie esse arquivo apenas quando um projeto ou execução local precisar de configurações repetíveis:

```sh
cp config.toml.example config.toml
```

Divisão recomendada:

- Configurações do scanner ficam em `[scanner]`, `[tests]` e `[mutations]`.
- Configurações do servidor, workers e migrations ficam em `[server]`, `[database]`, `[search]` e `[ui]`.
- Docker Compose deve continuar environment-first, porque containers normalmente recebem segredos e endpoints por `.env`, CI ou orquestrador.

Arquivos TOML separados por componente são úteis como exemplos de deploy, mas não devem ser obrigatórios no primeiro uso. Se você rodar binários diretamente, aponte cada um para o arquivo desejado com `OLLANTA_CONFIG_FILE=/path/to/config.toml`.

## 7. Customize Regras

Para regras somente locais, coloque arquivos de Custom Rule Pack no projeto escaneado:

```text
.ollanta/rules/team-rules.yaml
```

Para regras gerenciadas pelo servidor, use o Rule Studio:

1. Crie ou importe um draft.
2. Valide os exemplos.
3. Publique a regra.
4. Adicione a regra a um Quality Profile compatível.
5. Rode outro scan com perfis do servidor habilitados.

A assistência de IA apenas cria rascunhos de regra. O usuário ainda salva, valida, publica e ativa a regra explicitamente.

## 8. Entenda Tags

Tags já existem como metadados em projetos, regras, regras customizadas e issues. Hoje elas são úteis para filtros de issues, facets, derivação de domínio de qualidade e categorias de segurança.

O que ainda falta é um fluxo de tagging de primeira classe: catálogo gerenciado de tags, descrições de tags, edição em massa, páginas por tag de projeto ou regra, filtros salvos por tag e governança para vocabulários específicos de cada equipe. Trate isso como uma feature de produto separada, não como uma limpeza apenas de documentação.

## 9. Resolva Problemas do Primeiro Uso

Se o push via Docker parecer travado, verifique se ele está reconstruindo a imagem do scanner ou aguardando o processamento do servidor:

```sh
docker compose --profile server --profile push ps
docker ps --filter name=ollanta
```

Se apenas os containers do servidor estiverem rodando e não existir container `push`, o comando de push já terminou.

Se a saída terminar com `Server job ... completed` seguido de `gate=ERROR`, o Docker não travou. O scan foi concluído e o scanner retornou um código de Quality Gate reprovado.

Se uma UI local anterior ainda estiver ocupando a porta `7777`, pare apenas esse serviço:

```sh
docker compose --profile scanner stop local-ui
```

Se a prontidão do servidor falhar, inspecione os últimos logs:

```sh
docker compose --profile server logs --tail=100 ollantaweb ollantaworker ollantaindexer
```