.PHONY: build test lint fmt clean run smoke-local up down recreate logs

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

build:
	go build $(PKGS)

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
