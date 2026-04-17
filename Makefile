.PHONY: build test lint fmt clean run

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
		-serve \
		-port $(PORT)
