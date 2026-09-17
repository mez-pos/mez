# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 01 - Go server skeleton, config, logging, embed, cross-compile**
File: `docs/phases/phase-01.md`

## Last completed task

**Phase 01, Task 2 - Logging and lifecycle.** New `internal/logging` (ADR-020,
a ninth package because Phase 12's Windows Service cannot import `cmd/`; the
tree in `01-architecture.md` was updated). One handler over stdout and a
hand-written rotating file log, 8 MB and five kept (ADR-021). Timestamps are
UTC. A seventh config key, `log_format` = auto|json|text, decides the handler;
auto is text on a terminal, JSON otherwise.

`cmd/nofiltr/main.go` is now real: load, `MkdirAll`, logger, bind, serve,
SIGINT/SIGTERM, drain, then close everything in reverse under one 10s deadline
(ADR-022). `handler()` returns an empty mux, the seam Task 3 fills. Ran for real
on Windows: binds, logs to both sinks, 404s, and a second instance on the same
port exits 1 naming the address; that run also caught a duplicated `venue_id` on
the startup line, now fixed. Three failure probes, all caught: forward shutdown
order, draining with the cancelled context, a reopened log forgetting its size.

## Next task

**Phase 01, Task 3 - HTTP server and health.** See `docs/phases/phase-01.md`.
`internal/api` is still only a `doc.go`. Replace `handler()` in
`cmd/nofiltr/main.go` with the real chain; `TestHandlerIsEmptyForNow` asserts
the 404 and is meant to be rewritten, not deleted quietly. `serve` already owns
the lifecycle, so the task is routing and middleware only.

## Known broken / in progress

- `make` needs `sh` on PATH, so run it from Git Bash or WSL, not cmd.exe or
  PowerShell. GnuWin32 make 3.81 works from Git Bash; ezwinports ships 4.x.
- `make run` now starts the real server on `:7777` and writes to the default
  `data_dir`, which on Windows is `%PROGRAMDATA%\Nofiltr POS`. Override with
  `NOFILTR_DATA_DIR`. `make migrate` fails by design until Phase 02.
- `go test -race` needs cgo, which is banned here, so `make check` runs without
  it. There is now concurrency to race: the serve goroutine and the log
  writer's mutex. If CI ever gets a cgo-enabled job, this is what it is for.
- The domain import test checks direct imports only. See the cost note on
  ADR-016.

## Open questions for the human

None.

---

## Phase progress

- [x] 00 — Repo skeleton, docs bootstrap, design tokens
- [ ] 01 — Go server skeleton, config, logging, embed, cross-compile
- [ ] 02 — SQLite, migrations, sqlc, money type, IDs
- [ ] 03 — Domain core: order events, projections, invoice series, tax
- [ ] 04 — HTTP API, WebSocket hub, TypeScript type codegen
- [ ] 05 — Frontend shell: Vite, Tailwind tokens, router, primitives, auth
- [ ] 06 — Terminal: register, modifiers, cart, open tickets
- [ ] 07 — Payment, split, invoice, print queue, cash drawer
- [ ] 08 — Floor plan, tables, kitchen display, station routing
- [ ] 09 — Shift, blind drawer count, Z-report, tips
- [ ] 10 — Back office: menu editor, inventory, reports
- [ ] 11 — Relay service, outbound tunnel, guest QR ordering
- [ ] 12 — Installer, Windows Service, Docker, Litestream, docs
