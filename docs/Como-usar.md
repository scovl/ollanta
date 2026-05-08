# Como Usar o Ollanta

Este guia cobre a primeira jornada: escanear um projeto, enviar para o servidor, inspecionar os resultados e decidir o que configurar.

## 1. O caminho feliz em três linhas

```bash
make up          # sobe o servidor (docker, faz uma vez)
make run         # scan + abre UI interativa em http://localhost:7777
make push        # scan + envia resultados para o servidor em http://localhost:8080
```

`make run` prende o terminal enquanto serve a UI. Use `make run-bg` para rodar em background e `make stop` para matar o processo.

Customize os defaults quando precisar:

```sh
make run   PROJECT_DIR=D:\meuprojeto PROJECT_KEY=meu-app
make push  PROJECT_DIR=D:\meuprojeto PROJECT_KEY=meu-app SERVER=http://prod:8080 TOKEN=meu-segredo
```

## 2. O que cada comando faz

| Comando | Função |
|---------|--------|
| `make run` | Scan + UI local embutida na porta 7777. Sem banco. Gera `.ollanta/report.json`. |
| `make push` | Scan + push para o servidor `ollantaweb`. Resultados em PostgreSQL com histórico, tracking e gates. |
| `make up`  | Sobe a stack docker completa (postgres, ollantaweb, workers). |
| `make down` | Para a stack do servidor. |
| `make stop` | Mata o scanner em background (`make run-bg`). |

## 3. Suba o servidor

```bash
make up
```

Abra `http://localhost:8080` e entre com:
- Login: `admin`
- Senha: `admin`

A stack usa defaults de desenvolvimento. Para ambientes compartilhados, crie um `.env` sobrescrevendo `PG_PASSWORD`, `OLLANTA_JWT_SECRET` e `OLLANTA_SCANNER_TOKEN`.

Para alto throughput:

```bash
OLLANTA_WORKER_POOL=16 make up
```

## 4. Inspecione os Resultados

**UI local** (`make run` → `http://localhost:7777`):
- Overview: quality gate, métricas, distribuição de severidade, arquivos críticos, mutation testing, módulos de test signals, coverage
- Issues: filtro por severidade, tipo, regra; busca; detalhes da regra com rationale e exemplos de código
- Coverage: cobertura por arquivo com source viewer, linhas cobertas/descobertas
- Painel de detalhes: severidade, tipo, regra, engine, localização, tags; aba Rule com rationale/código; aba Fix with AI

**UI do servidor** (`http://localhost:8080`):
- Overview: resumo de review, métricas de new code, changed-code mutation, survived mutants, arquivos críticos
- Issues: busca facetada por severidade, tipo, qualidade, ciclo de vida, linguagem, regra, tag, diretório
- Coverage: navegue pelo snapshot de código com issue markers inline
- Activity: compare scans ao longo do tempo com gráficos de tendência
- Scopes: alterne entre branches e pull requests
- Quality Gate: veja as condições de pass/fail/warn
- Profiles: escolha quais regras rodam por linguagem; importe/exporte perfil como código
- Rule Studio: crie e publique regras customizadas; geração de rascunho assistida por IA
- Tags: catálogo governado de tags com trilha de auditoria

## 5. Códigos de saída do scanner

| Código | Significado |
|--------|-------------|
| `0` | Sucesso ou `-skip` ativado |
| `1` | Erro interno (I/O, parse) |
| `2` | Erro de usuário (flag inválida, config) |
| `3` | Quality Gate `ERROR` (scan concluiu, gate falhou) |

## 6. Configure apenas o necessário

Copie o config de exemplo só quando precisar de ajustes repetíveis:

```sh
cp config.toml.example config.toml
```

- Configurações do scanner: `[scanner]`, `[tests]`, `[mutations]`
- Configurações do servidor: `[server]`, `[database]`, `[search]`, `[ui]`
- Binários aceitam paths customizados via `OLLANTA_CONFIG_FILE=/caminho/config.toml`

## 7. Customize regras

Regras locais: coloque arquivos Custom Rule Pack em `.ollanta/rules/`:

```yaml
# .ollanta/rules/team-rules.yaml
version: 1
rules:
  - key: team:no-debug-print
    name: No debug prints in production
    language: go
    engine: text
    engine_config: { pattern: "fmt.Println", target: "" }
    type: code_smell
    severity: major
    message: Remove debug printing before committing.
```

Regras gerenciadas pelo servidor via Rule Studio:
1. Crie ou importe um rascunho
2. Valide os exemplos
3. Publique a regra
4. Adicione ao Quality Profile
5. Rode outro scan com perfis do servidor habilitados

## 8. Alternativa: comandos CLI puros

Se preferir não usar `make`, os equivalentes são:

```sh
# Scan local + UI
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . -project-key meu-app -format all \
  -with-tests -with-mutations -local-ui

# Push para o servidor
go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
  -project-dir . -project-key meu-app -format all \
  -with-tests -with-mutations \
  -server http://localhost:8080 -server-token ollanta-dev-scanner-token -server-wait
```

Ou via Docker:

```sh
docker compose --profile server up -d                     # sobe servidor
export PROJECT_DIR=. PROJECT_KEY=meu-app
docker compose --profile push run --build --rm push       # push scan
```

## 9. Troubleshooting

**Scanner já está rodando na porta 7777:**
```sh
make stop           # mata o scanner em background
```

**Containers do servidor não sobem:**
```sh
docker compose --profile server ps
docker compose --profile server logs --tail=100 ollantaweb
```

**Push parece travado:**
O scanner espera o processamento no servidor quando `-server-wait` está ativo. `make push` usa isso por padrão para confirmar que o scan foi totalmente ingerido.

**Interpolação de variáveis no config:**
`config.toml` suporta `${VAR}`, `${env.VAR}` e `${env.VAR:-default}`. Use `$$` para um `$` literal.

**Arquivo de config global:**
Coloque configurações compartilhadas em `~/.ollanta/config.toml` (`%USERPROFILE%\.ollanta\config.toml` no Windows).

**Suporte a proxy:**
```sh
ollanta -proxy http://corp-proxy:3128
# ou defina HTTP_PROXY / HTTPS_PROXY
```

**Pular scan em CI:**
```sh
ollanta -skip
```
Útil quando o scan é condicionalmente desabilitado. Código de saída 0.
