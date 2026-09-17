# Decisions

Append-only. Never edit or delete an entry — supersede it with a new one that
references the old number. Keep each entry to roughly ten lines.

Format:

```
## ADR-NNN — Title
Date · Status: accepted | superseded by ADR-NNN
Decision: what was chosen.
Because: the reason, in one or two sentences.
Cost: what this gives up.
```

---

## ADR-001 — The counter machine is the source of truth
2026-09 · Status: accepted
Decision: The venue's own computer runs the server and owns the database. Every
other device (second terminal, kitchen display, waiter phone) is a LAN client of
it. The cloud is optional.
Because: A POS that stops selling when the internet drops is not a POS. Making
the local machine authoritative deletes the entire offline-sync and conflict-
resolution problem instead of solving it badly.
Cost: No multi-venue reporting without the optional cloud layer. Chain-scale
deployments need a different topology later.

## ADR-002 — Go for the server
2026-09 · Status: accepted
Decision: Go 1.23+, single static binary.
Because: Distribution is the whole game for an open-source POS. `pos.exe`,
double-click, done — no Node runtime, no Python, no Docker. Cross-compiles to
Windows/Linux/macOS/ARM from one machine. Low, stable memory over 14-hour
shifts. Readable enough that outside contributors can actually contribute.
Cost: Two languages in the repo and no automatic type sharing with the
frontend. Mitigated by generating TypeScript types from Go structs (ADR-006).

## ADR-003 — SQLite, not Postgres
2026-09 · Status: accepted
Decision: SQLite in WAL mode via `modernc.org/sqlite` (pure Go, no cgo).
Because: At cafe scale — low thousands of orders a day, a handful of concurrent
devices — SQLite has orders of magnitude of headroom. Requiring a second
install and a running service would lose most self-hosters.
Cost: Single-writer. Schema stays deliberately portable so a future cloud
deployment can move to Postgres, but that migration is not free.

## ADR-004 — sqlc, no ORM
2026-09 · Status: accepted
Decision: Hand-written SQL compiled to type-safe Go by sqlc.
Because: Money and invoice code must be auditable at the SQL level. ORMs hide
exactly the queries that need scrutiny and generate surprises under load.
Cost: More typing. Schema changes require regenerating.

## ADR-005 — Money as int64 minor units
2026-09 · Status: accepted
Decision: All monetary values are `int64` paise/cents in a `money.Amount` type
carrying a currency code. Floats are banned repo-wide.
Because: A POS off by ₹0.03 on a three-way split is a broken POS and loses
user trust permanently.
Cost: Explicit conversion at every display and input boundary.

## ADR-006 — Generated TypeScript types from Go
2026-09 · Status: accepted
Decision: API request/response structs are the single source of truth; TS types
are generated into the frontend at build time.
Because: 35+ screens with hand-maintained duplicate types will drift and break
silently.
Cost: A codegen step in the build that must be kept working.

## ADR-007 — Browser in kiosk mode, not Electron or Tauri, for v1
2026-09 · Status: accepted
Decision: The terminal is Edge/Chrome launched with `--app=http://localhost:7777`
in kiosk mode by a desktop shortcut. The server runs as a Windows Service.
Because: Skips an entire desktop build pipeline. Staff see an app with no URL
bar and no way to wander off. Hardware access (printer, drawer) happens
server-side over TCP, so the browser needs no native privileges.
Cost: Dependent on a browser being present. Revisit with Tauri only if a real
hardware requirement demands it.

## ADR-008 — Guest QR ordering via outbound reverse tunnel
2026-09 · Status: accepted
Decision: The local server opens a persistent outbound WebSocket to a small
stateless relay. Guest requests to `order.<domain>/t/<table>` are routed down
that socket.
Because: A guest's phone is on cellular and cannot reach a LAN IP. Outbound
tunnelling needs no port forwarding, no static IP, and works behind CGNAT and
any cafe router. It is also the natural open-core boundary: the POS is free,
the hosted relay is the paid convenience.
Cost: QR ordering pauses when the internet drops. The counter keeps selling.

## ADR-009 — Orders are append-only event streams
2026-09 · Status: accepted
Decision: An order is a sequence of immutable events; the displayed state is a
projection. Voids and discounts are corrective events, not destructive edits.
Because: Gives a real audit trail for tax authorities, makes refunds honest,
and makes the order timeline screen fall out of the model for free.
Cost: More code than a mutable row. Projections must be kept fast.

## ADR-010 — Name, module path and licence split
2026-09 · Status: accepted
Decision: Product is **Nofiltr POS**. Binary and command package are `nofiltr`
(`cmd/nofiltr`, `nofiltr.exe`). Go module is
`github.com/byjoellobo/nofiltr-pos`. Server is AGPL-3.0 at the repo root; the
guest ordering app will carry its own MIT `LICENSE` when `web/guest/` is
created in Phase 11.
Because: The module path is effectively one-way (it appears in every internal
import), so it was settled before the first line of Go. The licence split is
the one `docs/00-product.md` already specifies: copyleft protects the hosted
relay, MIT keeps the embeddable guest bundle frictionless.
Cost: A repo named `nofiltr-pos` publishing a binary called `nofiltr` is a
small inconsistency. Two licences means contributors must know which tree they
are in.

## ADR-011 — LF line endings enforced by .gitattributes
2026-09 · Status: accepted
Decision: `.gitattributes` sets `* text=auto eol=lf`, with CRLF kept only for
`.bat`/`.cmd`/`.ps1` and binary assets marked explicitly.
Because: Development happens on Windows but the build cross-compiles to Linux
and macOS. A `Makefile` (Phase 00 Task 4) or shell script checked out with CRLF
fails on Linux CI in a way that reads as a mysterious syntax error. Setting
this before the first commit keeps the entire history clean and avoids a later
`git add --renormalize` churn commit.
Cost: Windows editors that cannot handle LF will show single-line files. All
current tooling handles LF.

## ADR-012 — INDEX.md indexes screens, not just files
2026-09 · Status: accepted
Decision: `docs/design/reference/INDEX.md` carries one row per screen (35 of
them), keyed by the `sc-if` state token and its line number, rather than the
one line per file that Phase 00 Task 2 literally asked for.
Because: the stated purpose was "find a screen without opening every file", and
three of the five files hold 9, 12 and 14 screens each. Five rows would have
sent every later phase back into a 131 KB file to hunt. The design files turned
out to be single-page prototypes where each screen is a `<sc-if value="{{ s_x
}}">` block, which gives a stable identifier to index on.
Cost: Line numbers drift if the archive is ever re-exported. INDEX.md says so
and gives the grep that recovers them.

## ADR-013 - Tokens corrected from the design, rules left alone
2026-09 · Status: accepted
Decision: Phase 00 Task 3 corrected three factual token values against the
archived design (`--paper` `#FBF7EF` to `#F4F1E9`; radius 0/4/8/12 to 3px and
2px; hairline "12% of primary" to 7% on ink) and added the tokens the design
uses that the doc lacked: ink and paper surface ladders, five-step text ladders
per surface, a second vermilion for paper, per-surface sage/gold/danger,
`--slate` `#7E86A8`, and the four paper status chip triads. Three *rules* the
design contradicts were left unchanged and recorded under "Unresolved
conflicts" instead: the 4px spacing scale, the 13-15px body minimum, and the
guest 16px minimum.
Because: a wrong hex is a fact and the design settles it. A spacing scale or a
touch-size floor is a deliberate constraint, and a prototype rendered for a
desktop browser is weak evidence against an ergonomics rule for a gloved hand
on a 12-inch tablet under glare. CLAUDE.md says to write the case and stop
rather than switch unilaterally.
Cost: `tokens.md` now carries three open conflicts that Phase 05 must settle
before it can generate `tokens.css`.
