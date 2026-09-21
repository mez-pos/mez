# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 05 - Frontend shell: Vite, Tailwind tokens, router, primitives, auth**
File: `docs/phases/phase-05.md`. Phase 04 is closed.

## Last completed task

**Phase 05, Task 1 - Tokens into code.** `web/shared/tokens.css` (palette
on `:root`, roles per `[data-surface]`), `tailwind.preset.ts` (roles only,
defaults replaced), subset fonts in `web/shared/fonts/`, and
`tokens-demo.html` (open it directly). ADR-062.

## Next task

**Phase 05, Task 2 - Three Vite apps.** Install Tailwind, add `satisfies
Config` to the preset and load it per app (`@config` on v4). Set
`data-surface` on each root: ink for terminal, paper for the others. Import
from `web/shared/api-types.ts`, never restate a server shape.

## Known broken / in progress

- Carried from phase 04, owned by no task yet:
  - The done-when script: a sale over HTTP against the real binary, with a
    WebSocket client watching. Start from `nofiltr setup`.
  - A `float64` lint rule for `internal/domain` (`docs/02-domain.md`).
  - A guard that fails the build when `order.State` changes without a bump
    to `order.ProjectionVersion`; today a forgotten bump serves stale
    snapshots.
- `AddLine` trusts the client's price and tax snapshot until menu tables
  exist (phase 10). GST place of supply is always the venue's state, so
  IGST sales cannot be rung up. `nofiltr setup` cannot finish a US venue.
- Never swept: idempotency keys, expired staff sessions, sign-in attempts.
  The pairing rate limiter is per process.
- Token gaps (ADR-062): five paper roles are stand-ins; ink has no chip
  triad (Task 4 needs one, in tokens.md first); `--vermilion-on` on the
  paper fill is 3.99:1, under AA. The preset is uncompiled until Task 2.
- `invoice.issued` needs a zero balance (ADR-046), so a bill printed before
  payment has no number. Phase 07 must work that way or revisit it.
- Every binary ships placeholder frontends. Phase 05 must add `-tags
  frontend` to `make dist`, which fails until `web/*/dist` exists.
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
- [ ] 05 — Frontend shell: Vite, Tailwind tokens, router, primitives, auth
- [ ] 06 — Terminal: register, modifiers, cart, open tickets
- [ ] 07 — Payment, split, invoice, print queue, cash drawer
- [ ] 08 — Floor plan, tables, kitchen display, station routing
- [ ] 09 — Shift, blind drawer count, Z-report, tips
- [ ] 10 — Back office: menu editor, inventory, reports
- [ ] 11 — Relay service, outbound tunnel, guest QR ordering
- [ ] 12 — Installer, Windows Service, Docker, Litestream, docs
