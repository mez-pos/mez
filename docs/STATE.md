# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 02 - SQLite, migrations, sqlc, money type, IDs**
File: `docs/phases/phase-02.md`. Phase 01 is complete.

## Last completed task

**Phase 02, Task 2 - The ID package.** `internal/domain/id`: `NewPublic`,
`ParsePublic`, `Normalize`, `NewPickupCode`, `NewBlocklist`, `Generator`.
98.9% coverage. ADR-033 to ADR-035. Nothing imports it or money yet.

**Task 4 must act on ADR-034:** a public ID is forty bits and is not unique on
its own, so every column holding one needs a unique index and a redraw on
conflict. Also unwired: `NewBlocklist(extra...)` takes the venue's language
but nothing reads it from config; whichever phase builds venue settings owns
that. Two gotchas for later callers: `NewGenerator(nil)` means crypto/rand
(entropy is injected like the clock), and in money, `Amount{}` is not a
universal zero, so a running total starts at `Sum(currency)`.

## Next task

**Phase 02, Task 3 - the database connection.** Nothing blocks it. It inherits
two things from Phase 01: `/api/v1/health` has a `checks.db` field stubbed to
`unknown` that this task should make real, and `Config.DataDir` is absolute and
already created at startup, so the file can go straight in it.

## Known broken / in progress

- `make` needs `sh` on PATH, so run it from Git Bash or WSL, not cmd.exe or
  PowerShell. GnuWin32 make 3.81 works from Git Bash; ezwinports ships 4.x.
- `make run` starts the real server on `:7777`, writing to the default
  `data_dir`. See `docs/running.md`. `make migrate` fails until Phase 02.
- `go test -race` needs cgo, which is banned, so `make check` runs without it.
  There is concurrency to race now: the serve goroutine and the log mutex.
- Every binary ships placeholder frontends. Phase 05 must add `-tags frontend`
  to `make dist`; it fails today because `web/*/dist` does not exist.
- `make build` writes `bin/nofiltr` with no `.exe`, so a stray server on Windows
  is killed with `taskkill /F /IM nofiltr`, without the extension.
- The domain import test checks direct imports only. See the cost note on
  ADR-016.
- `docs/02-domain.md` asks for a lint rule that fails the build on `float64` in
  `internal/domain`. No phase task owns it and it does not exist yet.

## Open questions for the human

None.

---

## Phase progress

- [x] 00 — Repo skeleton, docs bootstrap, design tokens
- [x] 01 — Go server skeleton, config, logging, embed, cross-compile
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
