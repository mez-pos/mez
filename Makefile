# Nofiltr POS
#
# Run this from a shell that has `sh` on PATH. On Windows that means Git Bash
# or WSL, not cmd.exe or PowerShell: every recipe below is POSIX.
#
# `make check` is the gate. CI runs exactly it and nothing else, so if it
# passes locally it passes there.

BIN     := nofiltr
CMD     := ./cmd/$(BIN)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Empty until Phase 00 Task 5 creates the tree. `go list` exits 0 and prints
# nothing on an empty module, unlike `go vet` and `go test`, which exit 1. Every
# recipe that would break on an empty module guards on this.
PKGS := $(shell go list ./... 2>/dev/null)

# No cgo, ever. modernc.org/sqlite is pure Go and the build must cross-compile
# from one machine. See ADR-003.
export CGO_ENABLED = 0

.DEFAULT_GOAL := check
.PHONY: check fmt vet test build run generate migrate clean dist help

## check: fmt, vet, test and build. The gate. CI runs this.
check: fmt vet test build
	@echo "ok: check passed ($(VERSION))"

## fmt: fail if anything is not gofmt-clean
fmt:
	@out=$$(gofmt -l . 2>/dev/null); \
	if [ -n "$$out" ]; then \
		echo "gofmt: not formatted:"; echo "$$out"; \
		echo "fix with: gofmt -w ."; \
		exit 1; \
	fi
	@echo "  fmt   ok"

## vet: go vet across all packages
vet:
ifeq ($(strip $(PKGS)),)
	@echo "  vet   skipped (no packages yet)"
else
	@go vet ./...
	@echo "  vet   ok"
endif

## test: go test across all packages
test:
ifeq ($(strip $(PKGS)),)
	@echo "  test  skipped (no packages yet)"
else
	@go test ./...
endif

## build: compile the server to bin/
build:
ifeq ($(strip $(PKGS)),)
	@echo "  build skipped (no packages yet)"
else
	@if [ -d "$(CMD)" ]; then \
		mkdir -p bin; \
		go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BIN) $(CMD); \
		echo "  build bin/$(BIN)"; \
	else \
		go build ./...; \
		echo "  build ok (no $(CMD) yet)"; \
	fi
endif

## run: build and run the server
run:
	@if [ ! -d "$(CMD)" ]; then \
		echo "no $(CMD) yet: it arrives in Phase 01"; exit 1; \
	fi
	@go run -ldflags "$(LDFLAGS)" $(CMD)

## generate: sqlc, go generate, TypeScript types
generate:
	@if command -v sqlc >/dev/null 2>&1; then \
		sqlc generate; echo "  sqlc  ok"; \
	else \
		echo "  sqlc  not installed (needed from Phase 02)"; \
	fi
ifneq ($(strip $(PKGS)),)
	@go generate ./...
endif

## migrate: apply database migrations
migrate:
	@if ! command -v goose >/dev/null 2>&1; then \
		echo "goose not installed (needed from Phase 02)"; exit 1; \
	fi
	@if [ ! -d internal/db/migrations ]; then \
		echo "no migrations yet: they arrive in Phase 02"; exit 1; \
	fi
	@goose -dir internal/db/migrations sqlite3 "$${NOFILTR_DB:-./data/nofiltr.db}" up

## clean: remove build output
clean:
	@rm -rf bin dist
	@echo "  clean ok"

## dist: cross-compile release binaries
dist:
	@if [ ! -d "$(CMD)" ]; then \
		echo "no $(CMD) yet: it arrives in Phase 01"; exit 1; \
	fi
	@rm -rf dist && mkdir -p dist
	@set -e; for t in windows/amd64 linux/amd64 darwin/arm64; do \
		os=$${t%/*}; arch=$${t#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out="dist/$(BIN)_$(VERSION)_$${os}_$${arch}$$ext"; \
		echo "  $$out"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o "$$out" $(CMD); \
	done
	@echo "  dist ok ($(VERSION))"

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
