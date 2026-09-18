# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 02 - SQLite, migrations, sqlc, money type, IDs**
File: `docs/phases/phase-02.md`. Phase 01 is complete.

## Last completed task

**Phase 02, Task 1 - The money package.** `internal/domain/money`: Add, Sub,
Mul by basis points, Neg, IsZero, Compare, Sum, RoundToNearest, SplitEvenly,
Parse, String. 96% statement coverage. Nothing imports it yet.

Four things to know before using it (ADR-029 to ADR-032). Every operation
returns an error, `Neg` included. `Amount{}` is not a universal zero, so start
a total with `Sum(currency)`, not `var total Amount`. `math.MinInt64` is
refused as an input *and* as a result, which is what makes "anything this
package returns can be negated for a refund" true. `SplitEvenly` gives the
leftover paise to the earliest parts, so a replayed split lands on the same
numbers. Rounding is half to even, integer arithmetic throughout; the per-venue
rounding setting is not read here, a later phase decides when to call it.

## Next task

**Phase 02, Task 2 - the ID package.** Nothing blocks it. Still open from
Phase 01: `/api/v1/health` has a `checks.db` field stubbed to `unknown` that
Task 3 should make real, and `Config.DataDir` is absolute and already created
at startup.

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
