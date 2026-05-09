.PHONY: build test lint fmt clean run push up down recreate logs release release-dry-run swagger

PKGS := \
	github.com/scovl/ollanta/ollantacore/... \
	github.com/scovl/ollanta/ollantaparser/... \
	github.com/scovl/ollanta/ollantarules/... \
	github.com/scovl/ollanta/ollantascanner/... \
	github.com/scovl/ollanta/ollantaengine/...

DIRS := ollantacore ollantaparser ollantarules ollantascanner ollantaengine

# CGO is required by go-tree-sitter. On Windows, point to the MSYS2 MinGW gcc.
export CGO_ENABLED := 1
export PATH := C:\msys64\mingw64\bin;$(PATH)

SCANNER_SRC := ./ollantascanner/cmd/ollanta
RELEASE_DIR := build
VERSION := 0.3.0

# ── Default settings ────────────────────────────────────────────────────
SCANNER_BIN := $(RELEASE_DIR)/ollanta.exe
PROJECT_DIR ?= .
PROJECT_KEY ?= $(notdir $(abspath $(PROJECT_DIR)))
PORT        ?= 7777
SERVER      ?= http://localhost:8080
TOKEN       ?= ollanta-dev-scanner-token

# ── Core targets ─────────────────────────────────────────────────────────
build:
	go build -ldflags="-X main.version=$(VERSION)" $(PKGS)

test:
	go test $(PKGS)

lint:
	golangci-lint run ./ollantacore/...
	golangci-lint run ./ollantaparser/...
	golangci-lint run ./ollantarules/...
	golangci-lint run ./ollantascanner/...
	golangci-lint run ./ollantaengine/...

fmt:
	gofmt -w $(DIRS)

clean:
	go clean $(PKGS)

# ── Run: scan + local interactive UI ─────────────────────────────────────
#   make run                        scan current directory
#   make run PROJECT_DIR=../myapp   scan another project
#   make run PORT=8888              custom port
$(SCANNER_BIN):
	-mkdir $(RELEASE_DIR)
	go build -ldflags="-X main.version=$(VERSION)" -o "$(SCANNER_BIN)" $(SCANNER_SRC)

run: $(SCANNER_BIN)
	./$(SCANNER_BIN) -project-dir "$(PROJECT_DIR)" -project-key "$(PROJECT_KEY)" -format all -with-tests -with-mutations -local-ui -port $(PORT)

# Background variant: same as run but doesn't block the terminal.
# Close the browser tab and run `make stop` to kill the server.
run-bg: $(SCANNER_BIN)
	powershell -Command "Start-Process -FilePath '$(SCANNER_BIN)' -ArgumentList @('-project-dir','$(PROJECT_DIR)','-project-key','$(PROJECT_KEY)','-format','all','-with-tests','-with-mutations','-local-ui','-port','$(PORT)') -WindowStyle Minimized"
	@echo "Scanner running in background on http://localhost:$(PORT)"
	@echo "Run 'make stop' to kill it."

stop:
	-taskkill /f /im ollanta.exe 2>nul
	@echo Scanner stopped.

# ── Push: scan + send to ollantaweb server ──────────────────────────────
#   make push                       push to http://localhost:8080
#   make push SERVER=http://prod:8080 TOKEN=my-key
push: $(SCANNER_BIN)
	./$(SCANNER_BIN) \
		-project-dir "$(PROJECT_DIR)" \
		-project-key "$(PROJECT_KEY)" \
		-format all \
		-with-tests \
		-with-mutations \
		-server $(SERVER) \
		-server-token $(TOKEN) \
		-server-wait \
		-server-wait-timeout 5m \
		-server-wait-poll 2s

# ── Server: docker compose management ────────────────────────────────────
COMPOSE_PROFILE ?= server

up:
	docker compose --profile $(COMPOSE_PROFILE) up -d

down:
	docker compose --profile $(COMPOSE_PROFILE) down

# recreate: destroy everything and start from zero.
# Stops all containers, deletes volumes (DB + search index + observability),
# removes project images, rebuilds with no cache, and starts fresh.
recreate:
	@echo "=== Stopping all ollanta containers ==="
	docker compose --profile $(COMPOSE_PROFILE) down --remove-orphans --volumes
	@echo "=== Removing ollanta images ==="
	-@docker images ollanta-scanner -q | xargs -r docker rmi -f
	-@docker images ollanta-server -q | xargs -r docker rmi -f
	@echo "=== Pruning dangling build cache ==="
	-docker builder prune -a -f
	@echo "=== Building images from scratch ==="
	docker compose --profile $(COMPOSE_PROFILE) build --no-cache
	docker compose --profile scanner build --no-cache
	@echo "=== Starting services ==="
	docker compose --profile $(COMPOSE_PROFILE) up -d --force-recreate
	@echo "=== Status ==="
	docker compose --profile $(COMPOSE_PROFILE) ps

logs:
	docker compose --profile $(COMPOSE_PROFILE) logs -f --tail=100

# ── Release ──────────────────────────────────────────────────────────────
release:
	@echo Building Ollanta Scanner $(VERSION) for all platforms...
	@mkdir -p $(RELEASE_DIR)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="-X main.version=$(VERSION)" -o $(RELEASE_DIR)/ollanta-linux-amd64       $(SCANNER_SRC)
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="-X main.version=$(VERSION)" -o $(RELEASE_DIR)/ollanta-linux-arm64       $(SCANNER_SRC)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="-X main.version=$(VERSION)" -o $(RELEASE_DIR)/ollanta-windows-amd64.exe $(SCANNER_SRC)
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="-X main.version=$(VERSION)" -o $(RELEASE_DIR)/ollanta-darwin-amd64      $(SCANNER_SRC)
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="-X main.version=$(VERSION)" -o $(RELEASE_DIR)/ollanta-darwin-arm64      $(SCANNER_SRC)
	@for bin in $(RELEASE_DIR)/ollanta-*; do tar -czf "$$bin.tar.gz" -C $(RELEASE_DIR) "$$(basename $$bin)"; done
	@cd $(RELEASE_DIR) && sha256sum *.tar.gz > checksums.txt
	@echo Done. Artifacts in $(RELEASE_DIR)/

release-dry-run:
	@echo Checking cross-compilation for all platforms...
	GOOS=linux   GOARCH=amd64 go build -o /dev/null $(SCANNER_SRC)
	GOOS=linux   GOARCH=arm64 go build -o /dev/null $(SCANNER_SRC)
	GOOS=windows GOARCH=amd64 go build -o /dev/null $(SCANNER_SRC)
	GOOS=darwin  GOARCH=amd64 go build -o /dev/null $(SCANNER_SRC)
	GOOS=darwin  GOARCH=arm64 go build -o /dev/null $(SCANNER_SRC)
	@echo All platforms compile successfully.

SMOKE_BACKEND_PORT ?= 18080

smoke-local:
	powershell -ExecutionPolicy Bypass -File scripts/smoke-local.ps1 -BackendPort $(SMOKE_BACKEND_PORT)

swagger:
	go run github.com/swaggo/swag/cmd/swag init -g api/router.go -d ./ollantaweb --parseDependency --parseInternal
	powershell -Command "New-Item -ItemType Directory -Force -Path 'ollantaweb/docs'; Move-Item -Force -Path 'docs/docs.go' -Destination 'ollantaweb/docs/docs.go'; Move-Item -Force -Path 'docs/swagger.json' -Destination 'ollantaweb/docs/swagger.json'; Move-Item -Force -Path 'docs/swagger.yaml' -Destination 'ollantaweb/docs/swagger.yaml'"
