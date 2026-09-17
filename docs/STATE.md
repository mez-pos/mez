# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 01 - Go server skeleton, config, logging, embed, cross-compile**
File: `docs/phases/phase-01.md`

## Last completed task

**Phase 00, Task 5 - Folder skeleton. Phase 00 is complete.** Created the tree
from `docs/01-architecture.md`: `cmd/{nofiltr,relay}`, eight `internal/*`
packages each with a `doc.go` stating its responsibility and import rules, plus
`web/` and `build/`. Added `internal/domain/imports_test.go` (ADR-016), which
walks the domain subtree and fails on `internal/db`, `internal/api`, `net/http`
and `database/sql`. It was verified by making it fail, not just pass: probes at
the top level and nested in a subpackage were both caught, and it refuses to
pass if it parses zero files.

`make check` now does real work instead of skipping. `make dist` produces
verified PE32+, static ELF and Mach-O arm64 binaries, `CGO_ENABLED=0` confirmed
in the build info.

## Next task

**Phase 01, Task 1.** See `docs/phases/phase-01.md`. `cmd/nofiltr/main.go` and
`cmd/relay/main.go` currently hold placeholder `main` functions that log a
version and exit; Phase 01 replaces the first one.

## Known broken / in progress

- `make` needs `sh` on PATH, so run it from Git Bash or WSL, not cmd.exe or
  PowerShell. GnuWin32 make 3.81 works from Git Bash; ezwinports ships 4.x.
- `make run`, `make migrate` still fail by design until Phase 01 and Phase 02
  create the real entrypoint and the migrations.
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
