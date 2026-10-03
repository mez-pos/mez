# mez
#
# Run this from a shell that has `sh` on PATH. On Windows that means Git Bash
# or WSL, not cmd.exe or PowerShell: every recipe below is POSIX.
#
# `make check` is the gate for Go; `make build-web` is the gate for the
# frontends. CI runs exactly those two, so if they pass locally they pass there.

BIN     := mez
CMD     := ./cmd/$(BIN)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)

# The commit date, not the wall clock. Two builds of the same commit then
# produce identical binaries, which is what makes -trimpath worth having and
# what lets someone check that a release matches its tag. See ADR-027.
DATE    := $(shell git show -s --format=%cI HEAD 2>/dev/null || echo unknown)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildDate=$(DATE)

# Empty until Phase 00 Task 5 creates the tree. `go list` exits 0 and prints
# nothing on an empty module, unlike `go vet` and `go test`, which exit 1. Every
# recipe that would break on an empty module guards on this.
PKGS := $(shell go list ./... 2>/dev/null)

# No cgo, ever. modernc.org/sqlite is pure Go and the build must cross-compile
# from one machine. See ADR-003.
export CGO_ENABLED = 0

# The real frontends are embedded (-tags frontend) when all three have been
# built; otherwise the placeholders are, so a clone with no Node still builds.
# `make build-web && make build` is therefore one binary serving all three
# apps. `make dist` always rebuilds them first, so a release never ships a
# stale or missing bundle. See ADR-025 and ADR-063.
WEB_APPS  := terminal backoffice guest
WEB_BUILT := $(foreach a,$(WEB_APPS),$(wildcard web/$(a)/dist/index.html))
TAGS      := $(if $(filter 3,$(words $(WEB_BUILT))),frontend,)

.DEFAULT_GOAL := check
.PHONY: check fmt fmt-web vet test typecheck build build-web run generate migrate migrate-new clean dist help

## check: fmt, vet, test, typecheck and build. The gate. CI runs this.
check: fmt vet test typecheck build
	@echo "ok: check passed ($(VERSION))"

## typecheck: tsc --noEmit over the three frontends, so a type the Go
##            structs changed cannot reach a screen unchecked. Needs Node 22.
typecheck: web/node_modules/.package-lock.json
	@cd web && npm run typecheck --silent
	@echo "  tsc   ok"

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
		mkdir -p bin && \
		go build -trimpath -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o bin/$(BIN) $(CMD) && \
		echo "  build bin/$(BIN) ($(if $(TAGS),real,placeholder) frontends)"; \
	else \
		go build ./... && \
		echo "  build ok (no $(CMD) yet)"; \
	fi
endif

## build-web: check formatting, test, typecheck and build the three frontends
##            into web/*/dist, then fail if the guest bundle is over 60 KB
##            gzipped. Needs Node 22.
build-web: web/node_modules/.package-lock.json
	@cd web && npm run format:check --silent && npm test --silent && npm run build --silent && npm run size --silent

## fmt-web: format the frontends with Prettier (web/.prettierrc.json)
fmt-web: web/node_modules/.package-lock.json
	@cd web && npm run format --silent

# npm rewrites this file on every install, so it is newer than the lockfile
# exactly when node_modules is current.
web/node_modules/.package-lock.json: web/package-lock.json
	@cd web && npm ci --no-audit --no-fund

## run: build and run the server
run:
	@if [ ! -d "$(CMD)" ]; then \
		echo "no $(CMD) yet: it arrives in Phase 01"; exit 1; \
	fi
	@go run -ldflags "$(LDFLAGS)" $(CMD)

## generate: sqlc, go generate, TypeScript types
##           sqlc output is committed, so a clone builds without it installed.
##           Install with: go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
##           The TypeScript types are generated from the Go request and
##           response structs and committed too. `make check` regenerates them
##           in memory and fails if they differ, so the two cannot drift.
generate:
	@sqlc=$$(command -v sqlc 2>/dev/null); \
	if [ -z "$$sqlc" ] && [ -x "$$(go env GOPATH)/bin/sqlc" ]; then \
		sqlc="$$(go env GOPATH)/bin/sqlc"; \
	fi; \
	if [ -n "$$sqlc" ]; then \
		"$$sqlc" generate && echo "  sqlc  ok"; \
	else \
		echo "  sqlc  not installed: go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1"; \
		exit 1; \
	fi
ifneq ($(strip $(PKGS)),)
	@go generate ./...
	@go test ./internal/api -run TestTypeScriptTypes -update -count=1 >/dev/null && 		echo "  types web/shared/api-types.ts"
endif

## migrate: apply database migrations
##          The server does this on every startup, so this target is only for
##          migrating a database without starting one.
migrate:
	@if ! command -v goose >/dev/null 2>&1; then \
		echo "goose not installed: go install github.com/pressly/goose/v3/cmd/goose@v3.26.0"; \
		echo "or just run the server, which migrates on startup"; \
		exit 1; \
	fi
	@goose -dir internal/db/migrations sqlite3 "$${MEZ_DB:-./data/mez.db}" up

## migrate-new: scaffold the next migration, NAME=add_orders
##              Numbered sequentially, not by timestamp: one line of history,
##              so the number is also the order it ran in. Deliberately plain
##              shell, so adding a migration needs no tool installed.
migrate-new:
	@if [ -z "$(NAME)" ]; then \
		echo "usage: make migrate-new NAME=add_orders"; exit 1; \
	fi
	@case "$(NAME)" in \
		*[!a-z0-9_]*) echo "NAME must be lower_snake_case: $(NAME)"; exit 1 ;; \
	esac
	@dir=internal/db/migrations; mkdir -p $$dir; \
	last=$$(ls $$dir 2>/dev/null | sed -n 's/^\([0-9][0-9][0-9][0-9][0-9]\)_.*\.sql$$/\1/p' | sort -n | tail -1); \
	next=$$(printf '%05d' $$(( 10#$${last:-0} + 1 ))); \
	file="$$dir/$${next}_$(NAME).sql"; \
	if [ -e "$$file" ]; then echo "$$file already exists"; exit 1; fi; \
	printf -- '-- +goose Up\n\n\n-- +goose Down\n\n' > "$$file"; \
	echo "  $$file"

## clean: remove build output
clean:
	@rm -rf bin dist $(foreach a,$(WEB_APPS),web/$(a)/dist)
	@echo "  clean ok"

## dist: cross-compile release binaries, with the real frontends
dist: build-web
	@if [ ! -d "$(CMD)" ]; then \
		echo "no $(CMD) yet: it arrives in Phase 01"; exit 1; \
	fi
	@rm -rf dist && mkdir -p dist
	@set -e; for t in windows/amd64 linux/amd64 darwin/arm64; do \
		os=$${t%/*}; arch=$${t#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out="dist/$(BIN)_$(VERSION)_$${os}_$${arch}$$ext"; \
		echo "  $$out"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -tags frontend -ldflags "$(LDFLAGS)" -o "$$out" $(CMD); \
	done
	@echo "  dist ok ($(VERSION))"

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
