# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 02 - SQLite, migrations, sqlc, money type, IDs**
File: `docs/phases/phase-02.md`. Phase 01 is complete.

## Last completed task

**Phase 01, Task 5 - Cross-compile and prove it. Phase 01 is complete.** The
Makefile stamps version, commit and commit date; all three show at
`/api/v1/health`, with a fallback to the Go toolchain's own VCS stamps so
`go run` still reports a commit (ADR-027). `docs/running.md` written for
operators. `build_date` is the commit date, so two builds of a commit are
byte-identical: verified by hashing two `make dist` runs, and by swapping in
`date -u` and watching them diverge. Done-when checked on the cross-compiled
PE32+ binary: health with real stamps, placeholder page, `CGO_ENABLED=0`.

Writing `running.md` found a real bug: `./nofiltr -h` printed nothing, because
`parseFlags` captures flag's output and nothing printed it. Fixed with
`config.Usage()` (ADR-028) and a test that fails if it is empty or incomplete.

## Next task

**Phase 02, Task 1.** Read `docs/phases/phase-02.md`; this session did not.
Nothing in Phase 01 blocks it. Two things it inherits: `Config.DataDir` is
absolute and already created at startup, and `/api/v1/health` has a `checks.db`
field stubbed to `unknown` that Phase 02 should make real.

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
