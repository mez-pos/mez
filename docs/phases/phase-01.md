# Phase 01 — Server skeleton

## Goal
`nofiltr` runs, serves a health endpoint, logs properly, reads config, serves an
embedded placeholder page, and cross-compiles to a Windows `.exe`. Prove the
distribution story on day one, before any logic depends on it.

## Depends on
Phase 00.

## Tasks

- [ ] **Task 1 — Config.**
  `internal/config`. Layered: defaults → `config.toml` next to the binary →
  environment → flags. Fields: `listen_addr` (default `:7777`), `data_dir`
  (platform-appropriate: `%PROGRAMDATA%/Nofiltr POS` on Windows, `./data` elsewhere),
  `log_level`, `venue_id`, `relay_url` (optional, empty = disabled),
  `timezone`. Must work with **no config file present**. Tests for precedence.

- [ ] **Task 2 — Logging and lifecycle.**
  `log/slog` to stdout, JSON in production, text in dev. A rotating file log in
  `data_dir/logs`. `cmd/nofiltr/main.go`: build the server, listen, handle
  SIGINT/SIGTERM, graceful shutdown with a timeout. Every subsystem gets a
  `Close()` and is shut down in reverse order.

- [ ] **Task 3 — HTTP server and health.**
  `internal/api`. stdlib `ServeMux` with Go 1.22 method routing. Middleware
  chain: request ID, structured access log, panic recovery, CORS for the dev
  Vite origin only. `GET /api/v1/health` returning version, uptime, and a
  `checks` object (db, disk, printer, tunnel) — all stubbed to `unknown` for now.

- [ ] **Task 4 — Embedded static serving.**
  `embed.FS` serving three SPA bundles from `web/*/dist` at `/`, `/backoffice`
  and `/order`. SPA fallback to `index.html` on unknown paths within each mount.
  A build tag or empty-dir fallback so `go build` works before the frontends
  exist. Drop a placeholder `index.html` in each so the routing is testable.

- [ ] **Task 5 — Cross-compile and prove it.**
  `make dist` produces `nofiltr.exe` (windows/amd64), plus linux/amd64 and
  darwin/arm64, all `CGO_ENABLED=0`. Embed version, commit and build date via
  ldflags, exposed at `/api/v1/health`. Write `docs/running.md`: how to start it,
  where data lives, how to change the port.

## Done when
`make dist` produces a Windows binary. Running it serves health and a
placeholder page on `:7777`. `make check` passes.

## Notes
If the binary does not run cleanly on a real Windows machine at the end of this
phase, stop and fix it. Everything downstream assumes this works.
