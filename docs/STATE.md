# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 04 - HTTP API, WebSocket hub, TypeScript type codegen**
File: `docs/phases/phase-04.md`. Phase 03 is complete.

## Last completed task

**Phase 03, Task 6 - split payment. Phase 03 is complete.**
`internal/domain/payment`: `Evenly`, `ByAmount`, `ByItem`, `ByGuest`, each
returning a `Split` checked to sum to the bill exactly; tips on
`Share.Tip`, never in `Amount`. 2000-bill property test, 96.4% coverage.
`order.Line.Due` added so a bill can be split by item: the active lines' Due
sums to `Totals.Total`, checked after every event. ADR-051.

Phase 03 leaves the domain whole and pure: `order` (events, projection with
tax, state machines), `invoice` (series, allocator contract, Audit), `tax`
(GST, VAT, US sales tax), `payment`, `fsm`, `table`, `kitchen`. Every
package is 95 to 100% covered, with no HTTP or SQL import. ADR-045 to 051.

## Next task

**Phase 04, Task 1.** Read `docs/phases/phase-04.md`; this session did not.
The API must build `order.Taxes` from venue settings for every projection,
store `State.TaxDetail` as `invoice.issued.tax_snapshot`, and hold the DB
allocator to `invoicetest.Conformance` (ADR-048, ADR-050).

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
