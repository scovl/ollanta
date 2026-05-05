.PHONY: build test lint fmt clean run smoke-local up down recreate logs release release-dry-run

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
VERSION := 0.2.0

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

# Run the scanner against a project. Override with:
#   make run PROJECT_DIR=D:\projects\myapp PROJECT_KEY=myapp
PROJECT_DIR ?= .
PROJECT_KEY ?= $(notdir $(abspath $(PROJECT_DIR)))
PORT        ?= 7777

run:
	go run github.com/scovl/ollanta/ollantascanner/cmd/ollanta \
		-project-dir "$(PROJECT_DIR)" \
		-project-key "$(PROJECT_KEY)" \
		-format all \
		-local-ui \
		-port $(PORT)

SMOKE_BACKEND_PORT ?= 18080

smoke-local:
	powershell -ExecutionPolicy Bypass -File scripts/smoke-local.ps1 -BackendPort $(SMOKE_BACKEND_PORT)

# Docker compose helpers for the server stack (postgres + zincsearch + ollantaweb).
COMPOSE_PROFILE ?= server

up:
	docker compose --profile $(COMPOSE_PROFILE) up -d

down:
	docker compose --profile $(COMPOSE_PROFILE) down

# Full rebuild: stop everything, rebuild images without cache, recreate containers.
recreate:
	docker compose --profile $(COMPOSE_PROFILE) down --remove-orphans
	docker compose --profile $(COMPOSE_PROFILE) build --no-cache
	docker compose --profile $(COMPOSE_PROFILE) up -d --force-recreate
	docker compose --profile $(COMPOSE_PROFILE) ps

logs:
	docker compose --profile $(COMPOSE_PROFILE) logs -f --tail=100
