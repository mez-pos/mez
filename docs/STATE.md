# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 00 — Repo skeleton, docs bootstrap, design archive**
File: `docs/phases/phase-00.md`

## Last completed task

**Phase 00, Task 3 - Verify and complete the tokens.** Measured every colour,
size, radius and gap in the design archive by frequency and rewrote
`docs/design/tokens.md` against it. Filled all five "to be defined" tokens,
corrected `--paper`, radius and hairline, and added what the doc lacked
(surface ladders, per-surface text ladders, a second vermilion for paper,
`--slate`, paper status chip triads). All 41 hexes verified against source.

**The three conflicts are now settled** (ADR-014), so nothing in `tokens.md`
is open. Spacing moved to a 2px base scale (the doc was wrong, the 4px grid
could not express the design's 10px gap). Type gained an 11px floor for
uppercase metadata, 13px sentence-case minimum and 15px interactive labels (the
design moves here: an 11px "Void" beside "Modify" is a hazard). The guest 16px
rule was rescoped to form controls, where the Safari zoom actually applies.

## Next task

**Phase 00, Task 4 - Makefile and CI.** `make` 3.81 is installed via winget
under "C:/Program Files (x86)/GnuWin32/bin" but is not on PATH in an
already-running shell. 3.81 is old; ezwinports ships 4.x if it chokes.

## Known broken / in progress

- No `Makefile` or CI workflow yet (both Task 4), so `make check` still cannot
  run. Work so far is verified with `gofmt -l`, `go vet ./...`, `go build ./...`.
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
