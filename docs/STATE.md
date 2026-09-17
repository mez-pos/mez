# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 01 - Go server skeleton, config, logging, embed, cross-compile**
File: `docs/phases/phase-01.md`

## Last completed task

**Phase 01, Task 1 - Config.** `internal/config`: `config.go` plus `toml.go`, a
flat-subset TOML reader written rather than depended on (ADR-018). Precedence is
defaults → `config.toml` beside the binary → `NOFILTR_*` → flags; all six fields
reach through every layer, and it boots with no file. `Config` carries parsed
values, not strings: `slog.Level`, `*time.Location`, an absolute `DataDir`. Bad
input stops the binary at startup (ADR-019), and `time/tzdata` is embedded so
named zones resolve on a machine with no Go install (ADR-017).
`load(args, lookupEnv, exeDir, goos)` is the testable core, so precedence and
both platform defaults run without touching process state. 92.7% coverage, and
both claims above were proved by making the tests fail: swapping two merge
lines, and dropping the tzdata import under a bogus `GOROOT`.

## Next task

**Phase 01, Task 2 - Logging and lifecycle.** See `docs/phases/phase-01.md`. It
replaces the placeholder `cmd/nofiltr/main.go`, the first consumer of
`config.Load`. Two inherited details: `Config.LogDir()` returns `data_dir/logs`
and nothing creates it yet, and `Load` wraps `flag.ErrHelp` on `-h`, so main
should `errors.Is` it and exit 0 rather than report a failure.

## Known broken / in progress

- `make` needs `sh` on PATH, so run it from Git Bash or WSL, not cmd.exe or
  PowerShell. GnuWin32 make 3.81 works from Git Bash; ezwinports ships 4.x.
- `make run` starts the placeholder `main`: it logs a version and exits, and
  reads no config until Task 2. `make migrate` fails by design until Phase 02.
- `go test -race` needs cgo, which is banned here, so `make check` runs without
  it. Nothing is concurrent yet; revisit at the hub in Phase 04.
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
