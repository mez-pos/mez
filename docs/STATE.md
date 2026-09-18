# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 03 - Domain core: order events, projections, invoice series, tax**
File: `docs/phases/phase-03.md`. Phase 02 is complete.

## Last completed task

**Phase 03, Task 4 - invoice series.** `internal/domain/invoice`: financial
years judged in the venue's time zone (`IndiaFY`, `CalendarFY`), parsed
number templates, `Series` with a length cap, the `Allocator` contract,
`Issue`, and `Audit` (the gapless check). `invoicetest` holds a reference
allocator and `Conformance`, the contract as a suite. 100% coverage. ADR-048.

What later tasks must know: the DB allocator (phase 07) must pass
`invoicetest.Conformance` through an adapter to `invoicetest.Tx`, and any
error from `invoice.Issue` means roll back. The documented default template
is 17 characters, one over GST's 16: GST venues need `{fy:short}`. Invoice
numbers can repeat across years, so a DB uniqueness index must include the
year.

## Next task

**Phase 03, Task 5 - tax profiles.** `internal/domain/tax`: the `TaxProfile`
interface, GST (CGST/SGST vs IGST, HSN per item, inclusive and exclusive,
line rounding then order reconciliation), and thinner VAT and US sales tax
profiles, with real worked examples. It must also settle the tax snapshot on
`line.added` and fill `Totals.Tax` in the order projection. Read
`docs/02-domain.md` in full first. If that is too much for one session, split
it in the phase file.

## Known broken / in progress

- `make` needs `sh` on PATH, so run it from Git Bash or WSL, not cmd.exe or
  PowerShell. GnuWin32 make 3.81 works from Git Bash; ezwinports ships 4.x.
- `make run` starts the real server on `:7777`, writing to the default
  `data_dir`. See `docs/running.md`. `make migrate` needs the goose CLI; the
  server migrates itself on startup, so it is only for a database with no
  server.
- `go test -race` needs cgo, which is banned, so `make check` runs without it.
  Now racing untested: the serve goroutine, the log mutex, the health prober.
- Every binary ships placeholder frontends. Phase 05 must add `-tags frontend`
  to `make dist`; it fails today because `web/*/dist` does not exist.
- `make build` writes `bin/nofiltr` with no `.exe`, so a stray server on Windows
  is killed with `taskkill /F /IM nofiltr`, without the extension.
- The domain import test checks direct imports only. See the cost note on
  ADR-016.
- `make generate` needs sqlc installed: `go install
  github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`. The generated code is committed,
  so only someone editing SQL needs it.
- `docs/02-domain.md` asks for a lint rule that fails the build on `float64` in
  `internal/domain`. No phase task owns it and it does not exist yet.
- `line.added` has no tax snapshot yet, so `Totals.Tax` is always zero. Task 5
  decides its shape; until then `invoice.issued.tax_snapshot` is raw JSON. No
  events are persisted, so adding it then is free.
- `invoice.issued` needs a zero balance (ADR-046), so a bill printed before
  payment has no invoice number. Phase 07 must work that way or revisit it.

## Open questions for the human

None.

---

## Phase progress

- [x] 00 — Repo skeleton, docs bootstrap, design tokens
- [x] 01 — Go server skeleton, config, logging, embed, cross-compile
- [x] 02 — SQLite, migrations, sqlc, money type, IDs
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
