# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 07 - Payment, split, invoice, print queue, cash drawer**
File: `docs/phases/phase-07.md`. Phase 06 is complete.

## Last completed task

**Phase 07, Task 4: ESC/POS driver** (ADR-079). `internal/device/escpos`:
the command builder, receipt, kitchen ticket and Z-report layouts at 80mm
and 58mm, `Send` over TCP 9100, and `nofiltr --test-print ADDRESS`
(`docs/running.md`). Checked against a stand-in TCP printer, not yet on
real paper.

## Next task

**Phase 07, Task 5a: print queue core.** Task 5 was split into 5a (queue,
worker, transports), 5b (jobs from the till and the queue on screen) and
5c (printers in the back office); see `docs/phases/phase-07.md`. Nothing
calls the Task 4 layouts yet: 5b is what prints a receipt, a kitchen
ticket and a reprint (ADR-078, ADR-079).

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
- A bill printed before payment has no number: numbers come with the
  settling payment (ADR-046, ADR-078). Task 4's pre-payment bill layout
  must say it is not a tax invoice.
- A split lives only on the tender screen (ADR-077); by guest picks seats
  there until lines carry them (phase 08).
- The server does not cap a tender's `reference`; the till caps it at 40.
- Printing is ASCII only (ADR-079): non-Latin item names print as "?".
  No physical printer has run the driver yet.
- A payment can still be voided after its invoice (ADR-078): Task 6 must
  decide that, with refunds.
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
