# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 01 - Go server skeleton, config, logging, embed, cross-compile**
File: `docs/phases/phase-01.md`

## Last completed task

**Phase 01, Task 4 - Embedded static serving.** New `package web` at `web/`
(go:embed cannot reach outside its package dir). Placeholder bundles by default,
`web/*/dist` with `-tags frontend`, because dist is gitignored and would break
a fresh clone: ADR-025. `internal/api/static.go` mounts terminal `/`, back
office `/backoffice/`, guest `/order/`. Fallback stops at the first dot and
`/api/` is claimed before the mounts: ADR-026.

Four probes, all caught: dropping the `/api/` claim, falling back for every
missing file, caching `index.html` forever, two mounts on one bundle. Ran live:
three distinct bundles, deep routes, `/order` redirecting, a missing asset
404ing, and the placeholder warning at startup.

## Next task

**Phase 01, Task 5 - Cross-compile and prove it.** See `docs/phases/phase-01.md`.
`make dist` already cross-compiles; this adds commit and build date via ldflags
into `api.BuildInfo`, whose fields already exist and are omitted while empty.
Then `docs/running.md`. Do **not** add `-tags frontend` to `make dist` yet:
`web/*/dist` does not exist and the build would fail. Every binary until Phase
05 ships placeholders.

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
