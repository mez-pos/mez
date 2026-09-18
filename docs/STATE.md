# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 02 - SQLite, migrations, sqlc, money type, IDs**
File: `docs/phases/phase-02.md`. Phase 01 is complete.

## Last completed task

**Phase 02, Task 4 - Migrations.** goose as a library, embedded from
`internal/db/migrations`, applied on every startup before the listener serves.
`00001` creates `venues`, `settings`, `roles`, `staff`, `devices`. 83.9%
coverage, ADR-040 to ADR-042. Both startup paths verified live: a fresh data
directory logs the migration, a second start logs "schema is current".

`make migrate-new NAME=add_orders` scaffolds the next file. The schema carries
the invariants itself, and every later migration must keep doing so:
`typeof(x) = 'integer'` on money columns, a `GLOB` check and no SQL default on
timestamps, `0/1` checks on booleans, a unique index on every public ID.
`docs/03-conventions.md` lists the rules; ADR-041 says why each exists.

**Both dependencies are pinned to keep the Go 1.23 floor** (ADR-036, ADR-040):
`modernc.org/sqlite` v1.39.0 and `goose` v3.26.0. `@latest` on either raises
`go.mod` to 1.25, and CI reads its toolchain from that line. Check it after
any `go get`.

## Next task

**Phase 02, Task 5 - sqlc.** `sqlc.yaml` generating into `internal/db/gen`,
first queries for venues, settings and staff, `make generate` regenerating, and
an integration test that migrates a temp DB and round-trips a venue. Two things
to settle there: timestamps are `TEXT` so sqlc will give `string` unless an
override maps them to `time.Time`, and reads go through `d.Reader()`, writes
through `d.Tx`, so the generated code needs both. This is the last task in
Phase 02.

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
