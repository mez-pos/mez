# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 00 — Repo skeleton, docs bootstrap, design archive**
File: `docs/phases/phase-00.md`

## Last completed task

**Phase 00, Task 2 — Archive the design reference.** The copy step was already
done before this session. Wrote `docs/design/reference/INDEX.md`: 35 screens
across three surfaces, each keyed by its `sc-if` state token and line number,
all verified against source. Ticked Tasks 1 and 2 in `phase-00.md`.

**`claude-design/` is fully archived and already gone. Safe to delete.**

## Next task

**Phase 00, Task 3 — Verify and complete the tokens.** Check every value in
`docs/design/tokens.md` against the reference and fill what it marks "to be
defined" (derived text colours, hairline, danger). Use `INDEX.md` to open only
the screens needed; never read the reference folder wholesale.

## Known broken / in progress

- **`make` is not installed on this dev machine** (no `make`, `mingw32-make` or
  `nmake`). Phase 00 Task 4 is blocked until one is: `winget install
  GnuWin32.Make`, or ezwinports. Linux CI is unaffected.
- No `Makefile` or CI workflow yet (both Task 4), so `make check` cannot run.
  Work so far is verified with `gofmt -l`, `go vet ./...`, `go build ./...`.
- `go vet ./...` exits 1 on an empty module ("matched no packages"). Task 4's
  `check` target must tolerate that, or Task 5 must land first.

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
