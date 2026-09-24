# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 07 - Payment, split, invoice, print queue, cash drawer**
File: `docs/phases/phase-07.md`. Phase 06 is complete.

## Last completed task

**Phase 07, Task 1: tender screen** (ADR-076, `terminal-tender.md`).
Charge on the register opens it: cash with quick-tender and change due,
card and UPI with a reference, a tip keyed apart from the bill, amount due
always in view. Part payments leave the ticket settling with the right
balance. One idempotency key per tender, reused on retry. Checked in a
browser against the built binary: card part payment with tip, then cash.

## Next task

**Phase 07, Task 2: splits.** By amount, by item, by guest, evenly,
driven by `internal/domain/payment` (split.go). The tender screen stops
at "Paid in full"; Task 3 issues the invoice.

## Known broken / in progress

- Carried from phase 04, owned by no task yet:
  - The done-when script: a sale over HTTP against the real binary, with a
    WebSocket client watching. Start from `nofiltr setup`.
  - A `float64` lint rule for `internal/domain` (`docs/02-domain.md`).
  - A guard that fails the build when `order.State` changes without a bump
    to `order.ProjectionVersion`; today a forgotten bump serves stale
    snapshots.
- Phase 09's exception report should count `entered_in_error` removes
  per staff member, apart from manager voids (ADR-073).
- `AddLine` is not checked against the menu (price, tax, modifier min/max);
  no menu writes over HTTP until phase 10. GST place of supply is always
  the venue's state, so no IGST. `nofiltr setup` cannot finish a US venue.
- Never swept: idempotency keys, expired staff sessions, sign-in attempts.
  The pairing rate limiter is per process.
- Token gaps (ADR-062): five paper roles are stand-ins; `--vermilion-on`
  on the paper fill is 3.99:1, under AA, and shows on the paper primary
  button in the gallery.
- `invoice.issued` needs a zero balance (ADR-046), so a bill printed before
  payment has no number. Phase 07 must work that way or revisit it.
- A paid ticket still takes lines and discounts until Task 3 closes it.
  The server does not cap a tender's `reference`; the till caps it at 40.
- A data dir path containing `%` cannot be opened: `internal/db/open.go`
  does not escape the path in its `file:` DSN.
- To try the apps: `nofiltr setup -demo-menu` prints a pairing code
  (`-device-kind back_office` for the back office); pair in the browser. A
  code for the wrong app is refused but its device stays (ADR-068).
- No endpoint returns the venue's name or a staff member's role; the lock
  screen shows the device name and no roles until one does.
- `go test -race` needs cgo, which is banned; `make check` runs without it.
- The domain import test checks direct imports only (ADR-016).
- Tooling: `make` needs `sh` (Git Bash; GnuWin32 make 3.81 works). `make
  generate` needs sqlc v1.31.1 and can dirty `go.sum` with goose's test
  modules: restore it with `git checkout go.sum`. `bin/nofiltr` has no
  `.exe`, so kill a stray server with `taskkill /F /IM nofiltr`.

## Open questions for the human

None.

---

## Phase progress

- [x] 00 — Repo skeleton, docs bootstrap, design tokens
- [x] 01 — Go server skeleton, config, logging, embed, cross-compile
- [x] 02 — SQLite, migrations, sqlc, money type, IDs
- [x] 03 — Domain core: order events, projections, invoice series, tax
- [x] 04 — HTTP API, WebSocket hub, TypeScript type codegen
- [x] 05 — Frontend shell: Vite, Tailwind tokens, router, primitives, auth
- [x] 06 — Terminal: register, modifiers, cart, open tickets
- [ ] 07 — Payment, split, invoice, print queue, cash drawer
- [ ] 08 — Floor plan, tables, kitchen display, station routing
- [ ] 09 — Shift, blind drawer count, Z-report, tips
- [ ] 10 — Back office: menu editor, inventory, reports
- [ ] 11 — Relay service, outbound tunnel, guest QR ordering
- [ ] 12 — Installer, Windows Service, Docker, Litestream, docs
