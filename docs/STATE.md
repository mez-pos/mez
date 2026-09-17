# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 00 — Repo skeleton, docs bootstrap, design archive**
File: `docs/phases/phase-00.md`

## Last completed task

**Phase 00, Task 1 — Initialise the repo.** Go module
`github.com/byjoellobo/nofiltr-pos` (go 1.23), AGPL-3.0 `LICENSE`, `.gitignore`,
`.gitattributes`, README. Product renamed `OpenPOS` -> `Nofiltr POS` and binary
`openpos` -> `nofiltr` across all docs. `git init`, first commit, pushed to
https://github.com/byjoellobo/nofiltr-pos (public).

## Next task

**Phase 00, Task 2 — Archive the design reference.**
See `docs/phases/phase-00.md`. Note: the four `.dc.html` files plus `support.js`
are already present in `docs/design/reference/`. What Task 2 still owes is
`docs/design/reference/INDEX.md` (one line per file naming which surface and
screen it shows), and confirming nothing in `claude-design/` was missed.

## Known broken / in progress

- No `Makefile` yet (Phase 00 Task 4), so `make check` cannot run. Task 1 was
  verified with `gofmt -l`, `go vet ./...` and `go build ./...` instead; all
  clean, with no Go packages to compile yet.
- No CI workflow yet (Phase 00 Task 4).
- **`make` is not installed on this dev machine** (no `make`, `mingw32-make` or
  `nmake` on PATH). Phase 00 Task 4 is blocked until one is installed —
  `winget install GnuWin32.Make`, or ezwinports. CI on Linux is unaffected.
- `go vet ./...` exits 1 on an empty module ("matched no packages"). Task 4's
  `check` target must tolerate that, or land Task 5 (which creates the
  packages) first.

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
