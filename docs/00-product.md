# Product

## What this is

An open-source, self-hostable Point of Sale system for cafes, bars and small
restaurants. It installs as a single application on the venue's own computer,
keeps working with no internet, and can be extended or hosted by anyone.

## The promise

**One ticket, wherever it starts.** A guest can order at the counter, from a
table QR, on the web, at a kiosk or through an aggregator — it becomes the same
ticket, routes to the same kitchen display, settles to the same invoice series
and lands in the same night's Z-report.

## Who it is for

**Primary — the owner-operator.** One to three locations. Currently on paper, a
spreadsheet, or a subscription POS that costs more than it returns. Technical
enough to run an installer. Not technical enough to run Docker.

**Secondary — the self-hoster.** Runs it on a NAS or a small VPS, wants no
vendor in the loop, will read a README.

**Not for — chains.** Multi-hundred-location central management is explicitly
out of scope. The architecture would need to change (ADR-001).

## Three surfaces

**Terminal** — staff. The counter, the floor and the pass. Built for speed on a
12-inch tablet with gloves, glare and a queue.

**Back office** — owner. Costs, menus, stock, people, tax and reporting.

**Guest** — web, QR and kiosk. Same menu, same modifiers, same invoice series as
the counter.

## The first venue's devices

Confirmed 2026-10-31 (ADR-381): the first venue sells on a Windows PC or
laptop running the till in a browser, a tablet used as a till or on the
floor, and a separate kitchen screen at the pass. Staff phones are not in
use there; guests use their own phones for QR ordering. Every screen is
checked at those sizes: 1366x768 and 1920x1080 for the PC, 1024x768 for
the tablet, 390 wide for a guest's phone (tokens.md, "Layout").

## v1 scope — the spine

This is what makes it a real POS. Nothing ships until all of it works.

1. Staff lock and PIN sign-in
2. Register: menu, modifiers, cart, open tickets, holds
3. Kitchen display with station routing
4. Payment: cash, card-as-tender, UPI; split by amount, by item, by guest
5. Tax-compliant numbered invoice, thermal print, cash drawer kick
6. Floor plan and table state
7. Shift lifecycle: open, blind drawer count, Z-report, tips
8. Back office: menu and item editing, basic inventory, core sales reports
9. Guest QR ordering at the table via the relay

## v2 and later — explicitly not now

Loyalty tiers, gift cards, memberships, promotions engine, purchasing and
suppliers, wastage workflows, staff rota and timesheets, aggregator channel
integrations, self-order kiosk hardware, multi-venue reporting, the art wall /
gallery module.

Build the event model and the adapter interfaces so these bolt on without a
rewrite. Do not build them.

## Non-goals

- Driving card terminals directly. In India these are standalone Pine Labs /
  Ezetap boxes; the POS records the tender. Model payments as an adapter.
- Accounting. Export to Tally / CSV; do not become a ledger.
- Table reservations beyond a simple waitlist.
- Being beautiful before being correct.

## Success criteria for v1

- A cafe can run a full trading day on it with the internet unplugged.
- A fresh Windows PC goes from download to first sale in under ten minutes.
- The Z-report reconciles to the paise, every time.
- Nobody has to open a terminal window to use it.

## Licensing

Three layers, and what is sold is none of what runs at the venue
(ADR-312):

- **The till is AGPL-3.0, in the binary, free for ever**: the register,
  the kitchen display, the menu, staff, stock, reports, invoices and
  printing. Nothing in the binary checks a licence, so there is no
  feature to unlock and none to crack.
- **The relay is AGPL-3.0, in this repository**, documented for anyone
  who will run a server. It decides who it carries by an entitlement: a
  file for a self-hoster.
- **mez Cloud is a service**, in a private repository that imports this
  module: accounts, billing, the hosted relay, the off-site backup and
  the back office over it when the venue's computer is off. It sells
  reach and continuity, which only a server that is always on can give.
  A venue whose subscription lapses loses those and never the till, the
  back office on its own network or its own backup.

The AGPL keeps a hosted fork contributing back. The guest ordering app
(`web/guest/`) is under it like the rest (ADR-342): it is served by the
venue and is not made to be put into other people's pages.
