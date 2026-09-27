# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 08 - Floor plan, tables, kitchen display, station routing**
File: `docs/phases/phase-08.md`. Phase 07 is complete.

## Last completed task

**Phase 08, Task 2a: table state, and the floor read for terminals**
(ADR-086). `table.Derive` works out free, seated, ordered, settling or
dirty from the orders at a table; `tables.cleared_at` (migration 00012) is
the only stored part. `GET /api/v1/floor/live` and `POST
.../tables/{id}/clear` need `order.open`. Clears and plan edits move
`floor_versions`, sent as `floor_changed`; the web client emits a `floor`
notice, and the back office editor refetches on it.

## Next task

**Phase 08, Task 2b: the floor screen.** Spec first. Read the phase file.

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
- `orders.table_id` has no foreign key (ADR-085). "Guests left" in 2b
  closes an empty order and must clear the table in the same step, or it
  shows dirty (ADR-086).
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
- The Windows spooler's write path is untested on a real printer (ADR-080).
- The cash drawer opens only with the settling receipt, not on a cash part
  payment (ADR-081). Kitchen tickets print only once phase 08 adds Send.
- The till refunds to one payment per credit note (ADR-084); past bills
  search only exact numbers and invoice sequences.
- `make build` does not build under cmd or PowerShell; for the browser,
  `CGO_ENABLED=0 go build -tags frontend -o bin/nofiltr ./cmd/nofiltr`.
- A data dir path containing `%` cannot be opened: `internal/db/open.go`
  does not escape the path in its `file:` DSN.
- To try the apps: `nofiltr setup -demo-menu` prints a pairing code
  (`-device-kind back_office` for the back office); pair in the browser. A
  code for the wrong app is refused but its device stays (ADR-068).
- No endpoint returns the venue's name or a staff member's role; the lock
  screen shows the device name and no roles until one does.
- `go test -race` needs cgo, which is banned; `make check` runs without it.
- The domain import test checks direct imports only (ADR-016).
- Tooling: `make` needs `sh` (Git Bash; GnuWin32 make 3.81 works, at
  `C:/Program Files (x86)/GnuWin32/bin/make.exe`). `make check` does not
  rebuild the web bundles: run `make build-web` before trying the browser. `make
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
- [x] 07 — Payment, split, invoice, print queue, cash drawer
- [ ] 08 — Floor plan, tables, kitchen display, station routing
- [ ] 09 — Shift, blind drawer count, Z-report, tips
- [ ] 10 — Back office: menu editor, inventory, reports
- [ ] 11 — Relay service, outbound tunnel, guest QR ordering
- [ ] 12 — Installer, Windows Service, Docker, Litestream, docs
