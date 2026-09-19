# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 04 - HTTP API, WebSocket hub, TypeScript type codegen**
File: `docs/phases/phase-04.md`. Phase 03 is complete.

## Last completed task

**Phase 04, Task 1 - order persistence.** Migration 00002: `orders`,
`order_events` (unique `(order_id, seq)`, append-only by trigger),
`order_snapshots`, `invoice_series`, `invoices`. `internal/db`:
`OpenOrder`, `AppendOrder(orderID, taxes, Decide)` (read, decide, project,
append in one tx, retry on seq conflict), `LoadOrder` (snapshot + tail),
`RebuildOrder`, `OrderEvents`, and `InvoiceAllocator`, which passes
`invoicetest.Conformance`. Snapshots every 50 events, keyed on projection
version and tax regime, and self-healing. Domain gained `order.Resume`,
`order.DecodePayload`, `State.Invoiced`, `order.ProjectionVersion`. ADR-052.

## Next task

**Phase 04, Task 2 - order service** (`internal/service/order`). Build each
command as a `db.Decide`, publish after `AppendOrder` returns, and handle
idempotency keys. Carried over: build `order.Taxes` from venue settings for
every projection, store `State.TaxDetail` as `invoice.issued.tax_snapshot`
(ADR-048, ADR-050), and issue invoices through `OrderTx.Invoices()` and
`OrderTx.RecordInvoice` inside the append.

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
- A change to `order.Project`'s rules or `order.State`'s shape must bump
  `order.ProjectionVersion`, or stale snapshots are served. Nothing checks it.
- `invoice.issued` needs a zero balance (ADR-046), so a bill printed before
  payment has no invoice number. Phase 07 must work that way or revisit it.

## Open questions for the human

None.

---

## Phase progress

- [x] 00 — Repo skeleton, docs bootstrap, design tokens
- [x] 01 — Go server skeleton, config, logging, embed, cross-compile
- [x] 02 — SQLite, migrations, sqlc, money type, IDs
- [x] 03 — Domain core: order events, projections, invoice series, tax
- [ ] 04 — HTTP API, WebSocket hub, TypeScript type codegen
- [ ] 05 — Frontend shell: Vite, Tailwind tokens, router, primitives, auth
- [ ] 06 — Terminal: register, modifiers, cart, open tickets
- [ ] 07 — Payment, split, invoice, print queue, cash drawer
- [ ] 08 — Floor plan, tables, kitchen display, station routing
- [ ] 09 — Shift, blind drawer count, Z-report, tips
- [ ] 10 — Back office: menu editor, inventory, reports
- [ ] 11 — Relay service, outbound tunnel, guest QR ordering
- [ ] 12 — Installer, Windows Service, Docker, Litestream, docs
