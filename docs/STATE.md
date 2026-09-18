# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 02 - SQLite, migrations, sqlc, money type, IDs**
File: `docs/phases/phase-02.md`. Phase 01 is complete.

## Last completed task

**Phase 02, Task 3 - The database connection.** `internal/db`: `Open`, `Tx`,
`Writer`, `Reader`, `Health`, `Close`. 87% coverage, ADR-036 to ADR-039.
`/api/v1/health` reports `checks.db` for real, verified live.

**The first dependency moved the Go floor.** `go get modernc.org/sqlite@latest`
takes v1.59.0, which needs Go 1.25 and silently rewrote `go.mod` from 1.23. It
is pinned to v1.39.0, the newest release still on 1.23 (ADR-036). CI reads its
toolchain from `go.mod`, so check that line after any `go get`.

For Tasks 4 and 5: write through `d.Tx`, read through `d.Reader()`, which is
`mode=ro` and refuses writes. Per-connection pragmas live in the DSN, so a
`PRAGMA` run after Open protects one connection and nothing else (ADR-038).
Tasks 1 and 2 shipped money and id, ADR-029 to ADR-035, both still unimported.

## Next task

**Phase 02, Task 4 - migrations.** goose, embedded, run on startup from
`internal/db/migrations/*.sql`, plus a `make migrate-new NAME=x` target. The
connection it runs on exists: use `d.Writer()` or `d.Tx`. First migration is
`venues`, `settings`, `devices`, `staff`, `roles`; give every public ID column
a unique index, per ADR-034.

## Known broken / in progress

- `make` needs `sh` on PATH, so run it from Git Bash or WSL, not cmd.exe or
  PowerShell. GnuWin32 make 3.81 works from Git Bash; ezwinports ships 4.x.
- `make run` starts the real server on `:7777`, writing to the default
  `data_dir`. See `docs/running.md`. `make migrate` fails until Task 4.
- `go test -race` needs cgo, which is banned, so `make check` runs without it.
  Now racing untested: the serve goroutine, the log mutex, the health prober.
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
