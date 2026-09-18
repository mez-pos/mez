# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 03 - Domain core: order events, projections, invoice series, tax**
File: `docs/phases/phase-03.md`. Phase 02 is complete.

## Last completed task

**Phase 03, Task 5 - tax profiles.** `internal/domain/tax`: the `TaxProfile`
interface, a shared engine (line rounding, then reconciliation per rate), and
GST, VAT and US sales tax profiles with hand-worked examples (Bengaluru cafe,
99.00 inclusive back-calculation, London cafe, NYC 8.875%). 500-bill property
test on the engine's invariants. 95% coverage. ADR-049. Task 5 was split:
this session built the package; Task 5b wires it into the projection.

What later tasks must know: tax rates are `tax.Rate`, thousandths of a
percent, not basis points (ADR-049). `money.Amount.Scale(num, den)` exists
for fractional arithmetic. `tax.TaxResult` is the invoice's `tax_snapshot`.

## Next task

**Phase 03, Task 5b - tax in the projection.** Snapshot tax facts on
`line.added`, allocate order discounts across lines, and fill `Totals.Tax`
through a `TaxProfile` passed into the projection, keeping it pure. Read
`docs/02-domain.md` in full first. The task text is in the phase file.

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
- `line.added` has no tax snapshot yet, so `Totals.Tax` is always zero until
  Task 5b. `invoice.issued.tax_snapshot` is raw JSON; `tax.TaxResult` is the
  shape to store in it. No events are persisted, so changing them is free.
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
