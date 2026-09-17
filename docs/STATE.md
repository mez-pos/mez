# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 01 - Go server skeleton, config, logging, embed, cross-compile**
File: `docs/phases/phase-01.md`

## Last completed task

**Phase 01, Task 3 - HTTP server and health.** `internal/api`: `ServeMux` with
Go 1.22 method patterns, `GET /api/v1/health` returning version, uptime and four
`unknown` checks. Middleware, outermost first: requestID, accessLog,
recoverPanic, cors. 404 and 405 are routed deliberately so every response is the
`{"error":{...}}` envelope (ADR-024). An eighth config key, `dev_origin`,
gates CORS; empty means no CORS headers at all, so a shipped binary has none
(ADR-023). `main` wires it; `serve` is unchanged.

Four failure probes. Three caught their bug: reflecting any Origin, trusting an
inbound `X-Request-Id`, dropping the 405 route. The fourth, swapping accessLog
and recoverPanic, **passed** - the ordering was undertested, so `middleware()`
was extracted and a test now wraps a panicking handler in the production order.
Ran live: health, 405, 404, preflight from the dev origin, nothing from any
other, request ids matching between header and log.

## Next task

**Phase 01, Task 4 - Embedded static serving.** See `docs/phases/phase-01.md`.
Two things it inherits: the `/` catch-all in `internal/api/routes` answers 404
for everything unmatched, and Task 4 must take that over for the static mounts
without swallowing unknown `/api/v1/...` paths, which keep the JSON envelope.
`web/*/dist` does not exist yet, so the empty-dir fallback is the first thing
to settle or `go build` breaks.

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
