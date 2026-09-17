# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 00 — Repo skeleton, docs bootstrap, design archive**
File: `docs/phases/phase-00.md`

## Last completed task

**Phase 00, Task 4 - Makefile and CI.** `Makefile` with build, test, check,
run, generate, migrate, clean, dist, help. `make check` passes on the empty
module: it guards vet and test behind `go list ./...`, which exits 0 where vet
and test exit 1 (ADR-015). `dist` cross-compiles windows/amd64, linux/amd64 and
darwin/arm64, `CGO_ENABLED=0` Makefile-wide. `.github/workflows/ci.yml` runs
`make check` on push and PR, taking its Go version from `go.mod`.

Also settled the three token conflicts ADR-013 left open, see ADR-014.

## Next task

**Phase 00, Task 5 - Folder skeleton.** The last task in Phase 00. Create the
tree from `docs/01-architecture.md` with a `doc.go` per package, plus
`internal/domain/imports_test.go`, a test that walks `internal/domain` and
fails if it imports `internal/db` or `internal/api`. Once packages exist,
`make check` stops skipping vet and test, so expect it to do real work.

## Known broken / in progress

- `make` needs `sh` on PATH, so run it from Git Bash or WSL, not cmd.exe or
  PowerShell. The installed GnuWin32 make 3.81 works from Git Bash; ezwinports
  ships 4.x if it ever chokes.
- `make check` currently prints "skipped (no packages yet)" for vet, test and
  build. That is expected and disappears when Task 5 adds the first package.
- `make run`, `make dist` and `make migrate` fail by design until Phase 01 and
  Phase 02 create `cmd/nofiltr` and the migrations.

## Open questions for the human

None. Name, module path and licence were settled in ADR-010.

---

## Phase progress

- [ ] 00 — Repo skeleton, docs bootstrap, design tokens
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
