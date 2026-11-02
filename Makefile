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

# The package list. `go list` exits 0 and prints nothing on an empty module,
# unlike `go vet` and `go test`, which exit 1, so the recipes that would break
# on an empty module guard on this; it costs nothing now the tree exists.
# web/node_modules is npm's, not ours: a package there can ship Go files
# (flatted, under eslint, does), which ./... would vet and test as ours.
PKGS := $(shell go list ./... 2>/dev/null | grep -v /node_modules/)

# No cgo, ever. modernc.org/sqlite is pure Go and the build must cross-compile
# from one machine. See ADR-003.
export CGO_ENABLED = 0

# The real frontends are embedded (-tags frontend) when all three have been
# built; otherwise the placeholders are, so a clone with no Node still builds.
# `make build-web && make build` is therefore one binary serving all three
# apps. `make dist` always rebuilds them first, so a release never ships a
# stale or missing bundle. See ADR-025 and ADR-063.
WEB_APPS     := terminal backoffice guest
WEB_BUILT    := $(foreach a,$(WEB_APPS),$(wildcard web/$(a)/dist/index.html))
FRONTEND_TAG := $(if $(filter 3,$(words $(WEB_BUILT))),frontend,)

# The tablet app is embedded too (-tags tabletapp) when a build of it and its
# app.json are in internal/device/tabletapp/app/, where the release puts
# them; otherwise the binary carries none and says so. See ADR-421.
APP_DIR := internal/device/tabletapp/app
APP_TAG := $(if $(and $(wildcard $(APP_DIR)/mez.apk),$(wildcard $(APP_DIR)/app.json)),tabletapp,)

comma := ,
empty :=
space := $(empty) $(empty)
TAGS  := $(subst $(space),$(comma),$(strip $(FRONTEND_TAG) $(APP_TAG)))

.DEFAULT_GOAL := check
.PHONY: check fmt fmt-web vet test typecheck build build-web run dev generate migrate migrate-new clean dist installer android help

## check: fmt, vet, test, typecheck and build. The gate. CI runs this.
check: fmt vet test typecheck build
	@echo "ok: check passed ($(VERSION))"

## typecheck: tsc --noEmit over the three frontends, so a type the Go
##            structs changed cannot reach a screen unchecked, then eslint's
##            rules of hooks over them and shared (errors only: `npm run lint`
##            in web also lists the warnings). Needs Node 22.
typecheck: web/node_modules/.package-lock.json
	@cd web && npm run typecheck --silent
	@echo "  tsc   ok"
	@cd web && npm run lint --silent -- --quiet
	@echo "  lint  ok"

## fmt: fail if anything is not gofmt-clean
fmt:
	@out=$$(gofmt -l . 2>/dev/null | grep -v node_modules); \
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
	@go vet $(PKGS)
	@echo "  vet   ok"
endif

## test: go test across all packages
test:
ifeq ($(strip $(PKGS)),)
	@echo "  test  skipped (no packages yet)"
else
	@go test $(PKGS)
endif

## build: compile the server, and the optional relay, to bin/
build:
ifeq ($(strip $(PKGS)),)
	@echo "  build skipped (no packages yet)"
else
	@if [ -d "$(CMD)" ]; then \
		mkdir -p bin && \
		go build -trimpath -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o bin/$(BIN) $(CMD) && \
		echo "  build bin/$(BIN) ($(if $(FRONTEND_TAG),real,placeholder) frontends$(if $(APP_TAG),$(comma) the tablet app,))" && \
		go build -trimpath -ldflags "$(LDFLAGS)" -o bin/relay ./cmd/relay && \
		echo "  build bin/relay"; \
	else \
		go build ./... && \
		echo "  build ok (no $(CMD) yet)"; \
	fi
endif

## build-web: check formatting, test, typecheck and build the three frontends
##            into web/*/dist, then fail if the guest bundle is over 60 KB
##            gzipped, or if a class in the sources renders nothing (ADR-405).
##            Needs Node 22.
build-web: web/node_modules/.package-lock.json
	@cd web && npm run format:check --silent && npm test --silent && npm run build --silent && npm run size --silent && npm run classes --silent

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

## dev: a dev venue (a demo cafe in .dev-data), the server on port 7791 and
##      the terminal and back office under Vite with hot reload. The first run
##      prints a pairing code for each app. DEV_DATA and DEV_PORT change the
##      directory and the port. Needs Node 22. See build/dev.sh.
dev: web/node_modules/.package-lock.json
	@mkdir -p bin && go build -o bin/$(BIN) $(CMD)
	@sh build/dev.sh

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

## dist: cross-compile release binaries, with the real frontends, and the
##       tablet app when a build of it is in internal/device/tabletapp/app/
dist: build-web
	@if [ ! -d "$(CMD)" ]; then \
		echo "no $(CMD) yet: it arrives in Phase 01"; exit 1; \
	fi
	@rm -rf dist && mkdir -p dist
	@set -e; for t in windows/amd64 linux/amd64 linux/arm64 darwin/arm64; do \
		os=$${t%/*}; arch=$${t#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out="dist/$(BIN)_$(VERSION)_$${os}_$${arch}$$ext"; \
		echo "  $$out"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -tags "frontend$(if $(APP_TAG),$(comma)$(APP_TAG),)" -ldflags "$(LDFLAGS)" -o "$$out" $(CMD); \
	done
	@echo "  dist ok ($(VERSION)$(comma) $(if $(APP_TAG),with,without) the tablet app)"

## installer: the Windows installer, dist/mez_<version>_windows_amd64_setup.exe,
##            around the binary `make dist` built. Needs Inno Setup 6: pass
##            ISCC=/path/to/ISCC.exe if it is not on PATH. Signed when
##            MEZ_SIGNTOOL is a signtool command ending in $$f (ADR-265).
ISCC ?= ISCC
installer:
	@win="dist/$(BIN)_$(VERSION)_windows_amd64.exe"; \
	if [ ! -f "$$win" ]; then echo "no $$win: run 'make dist' first"; exit 1; fi; \
	MSYS2_ARG_CONV_EXCL='*' "$(ISCC)" /Q "/DVersion=$(VERSION)" \
		'/DBinary=..\..\dist\$(BIN)_$(VERSION)_windows_amd64.exe' \
		$(if $(MEZ_SIGNTOOL),'/Ssigntool=$(MEZ_SIGNTOOL)' /DSign,) \
		build/installer/mez.iss
	@echo "  installer dist/$(BIN)_$(VERSION)_windows_amd64_setup.exe"

## android: the tablet app in android-app/ (ADR-409): a debug APK, its unit
##          tests and lint. Needs a JDK and the Android SDK, which `make
##          check` never does. A debug build starts on "Pair this
##          tablet", or at MEZ_URL if given one (http://10.0.2.2:7777/ is
##          the computer an emulator runs on). MEZ_MDNS (10.0.2.2:5354)
##          sends its question for mez.local to a responder there, for an
##          emulator, whose network carries no multicast.
MEZ_URL ?=
MEZ_MDNS ?=
android:
	@cd android-app && ./gradlew --console=plain $(if $(MEZ_URL),'-PmezUrl=$(MEZ_URL)',) \
		$(if $(MEZ_MDNS),'-PmezMdns=$(MEZ_MDNS)',) \
		assembleDebug testDebugUnitTest lintDebug
	@echo "  android ok: android-app/app/build/outputs/apk/debug/app-debug.apk"

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
