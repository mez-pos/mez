# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 04 - HTTP API, WebSocket hub, TypeScript type codegen**
File: `docs/phases/phase-04.md`. Phase 03 is complete.

## Last completed task

**Phase 04, Task 3d - first-run setup. Task 3 is complete.** `nofiltr setup`
(`cmd/nofiltr/setup.go`) creates the venue, the default roles, the owner with
a PIN and the first pairing code, in one transaction that refuses to run on a
database with a venue in it. `config.LoadArgs` exported so the subcommand
resolves the data directory as the server does. Documented in
`docs/running.md`. ADR-058.

Phase 04 tasks 1 to 3 are done: order persistence, the order service, and
auth (pairing, PIN sign-in, permissions, setup).

## Next task

**Phase 04, Task 4 - HTTP handlers.** `/api/v1` for orders, tickets, tables,
menu read, payments, shifts. The pieces are in place: `api.Meta(r)` builds
`service/order.Meta`, `api.Override` and `s.authorise` handle the manager
override, `decodeJSON` and the error envelope are there, and every route
mounts inside `requireDevice` and `requireStaff`.

## Known broken / in progress

- `make` needs `sh` on PATH, so run it from Git Bash or WSL, not cmd.exe or
  PowerShell. GnuWin32 make 3.81 works from Git Bash; ezwinports ships 4.x.
- `make run` starts the real server on `:7777`, writing to the default
  `data_dir`. See `docs/running.md`. `make migrate` needs the goose CLI; the
  server migrates itself on startup, so it is only for a database with no
  server.
- `go test -race` needs cgo, which is banned, so `make check` runs without it.
  Now racing untested: the serve goroutine, the log mutex, the health prober.
- Every binary ships placeholder frontends. Phase 05 must add `-tags frontend`
  to `make dist`; it fails today because `web/*/dist` does not exist.
- `make build` writes `bin/nofiltr` with no `.exe`, so a stray server on Windows
  is killed with `taskkill /F /IM nofiltr`, without the extension.
- The domain import test checks direct imports only. See the cost note on
  ADR-016.
- `make generate` needs sqlc installed: `go install
  github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`. The generated code is committed,
  so only someone editing SQL needs it.
- `docs/02-domain.md` asks for a lint rule that fails the build on `float64` in
  `internal/domain`. No phase task owns it and it does not exist yet.
- A change to `order.Project`'s rules or `order.State`'s shape must bump
  `order.ProjectionVersion`, or stale snapshots are served. Nothing checks it.
- `AddLine` takes the price and tax snapshot from the client, because no menu
  tables exist yet. When they do, the service must resolve item IDs itself.
- Idempotency keys never expire. GST place of supply is always the venue's
  state, so inter-state (IGST) sales cannot be rung up yet.
- `nofiltr setup` cannot finish a US sales tax venue: its jurisdiction rates
  are a list, and the back office that edits them arrives in phase 10.
- Expired staff sessions and old sign-in attempts are never swept. The rate
  limiter is per process and resets with it.
- `invoice.issued` needs a zero balance (ADR-046), so a bill printed before
  payment has no invoice number. Phase 07 must work that way or revisit it.

## Open questions for the human

None.

---

## Phase progress

- [x] 00 — Repo skeleton, docs bootstrap, design tokens
- [x] 01 — Go server skeleton, config, logging, embed, cross-compile
- [x] 02 — SQLite, migrations, sqlc, money type, IDs
- [x] 03 — Domain core: order events, projections, invoice series, tax
- [ ] 04 — HTTP API, WebSocket hub, TypeScript type codegen
- [ ] 05 — Frontend shell: Vite, Tailwind tokens, router, primitives, auth
- [ ] 06 — Terminal: register, modifiers, cart, open tickets
- [ ] 07 — Payment, split, invoice, print queue, cash drawer
- [ ] 08 — Floor plan, tables, kitchen display, station routing
- [ ] 09 — Shift, blind drawer count, Z-report, tips
- [ ] 10 — Back office: menu editor, inventory, reports
- [ ] 11 — Relay service, outbound tunnel, guest QR ordering
- [ ] 12 — Installer, Windows Service, Docker, Litestream, docs
