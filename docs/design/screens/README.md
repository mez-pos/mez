# Screen specs

One file for each screen, or each small family of screens, saying what it
holds and does. How it looks is `docs/design/tokens.md`, the mez design
language (ADR-128): a spec names roles ("the primary button", "the serif",
"tabular"), and tokens.md says what those are.

Every spec here was written from the screen's code as it stood at the end of
the rebuild (ADR-196), not from the design file and not from the spec before
it. **Read only the spec of the screen you are changing**, and change it in
the same task as the screen: a spec that describes the old layout is worse
than none.

The template is in `docs/design/components.md`. A spec written for a screen
not yet built is written at the start of the phase that builds it, not in
advance.

## What a spec holds

Purpose, Layout, Data, Interactions, States, Constraints, and **Reference**:
the design screen it was built from, by file and line, with

- what the design draws that the screen **leaves out**, and why;
- what it **changed**, and why;
- what the screen **keeps** that the design does not draw (the rebuild's
  rule 4).

So a difference from the design that is listed there is decided, not a bug.

## The specs

The design files are in `docs/design/reference/mez/`. "Ours" means the design
draws nothing for it.

### Terminal, every role

| Spec | Screen | Path | Design |
|---|---|---|---|
| `terminal-lock.md` | Staff Login, and pairing | `/lock`, `/pair` | D01 |
| `terminal-shell.md` | The frame: navigation, where each role lands, the shift gate | every path | D03, D05, T01 |
| `terminal-open-shift.md` | Open the shift | `/shift` | D02 |
| `terminal-close.md` | Close the day | `/shift/close` | D23 |
| `terminal-print-queue.md` | Print queue | a sheet | Ours |

### Terminal, cashier and manager

| Spec | Screen | Path | Design |
|---|---|---|---|
| `terminal-register.md` | The order screen, its sheets, the item modal | `/pos` | D03, D04 |
| `terminal-floor.md` | Tables: the plan, the list, the panel | `/tables` | D05, D06 (the list) |
| `terminal-tender.md` | Payment, Split Bill, the card and UPI states, the invoice | `/pay/:id` | D07, D08a, D08b |
| `terminal-past-bills.md` | Orders, and refunds | `/orders`, `/orders/:id` | D09 |

### Terminal, waiter

| Spec | Screen | Path | Design |
|---|---|---|---|
| `waiter-home.md` | The tables as cards | `/waiter` | T01 |
| `waiter-order.md` | Order taking, Confirm and send, Course View | `/pos` | T02, T04 |
| `waiter-table.md` | A table, and Transfer and merge | `/waiter/table/:id` | T03, T06 |
| `waiter-bill.md` | The bill at the table | `/pay/:id` | T05 |
| `waiter-shift.md` | Your shift | `/waiter/shift` | T08 |

### Kitchen

| Spec | Screen | Path | Design |
|---|---|---|---|
| `kitchen-display.md` | The board: the wall display, and a cook on a terminal | `/kitchen`, `/pass` | D10, D11 |

### Back office

| Spec | Screen | Path under `/backoffice` | Design |
|---|---|---|---|
| `backoffice-shell.md` | The frame: the bar, the sidebar, who sees what | every path | D12 |
| `backoffice-dashboard.md` | Dashboard | `/` | D12 |
| `backoffice-menu.md` | Menu, Choices, Stations and courses | `/menu` | D18 |
| `backoffice-stock.md` | Inventory, and Recipes | `/stock` | D19 |
| `backoffice-people.md` | Staff and roles | `/people` | D20 |
| `zreport.md` | The Z-report, in both apps, and the Z-reports list | `/reports`, `/reports/:n` | D23 |
| `tips.md` | A shift's tips | `/shifts/:id/tips` | Ours |
| `backoffice-audit.md` | Audit | `/audit` | Ours |
| `backoffice-reports.md` | Sales reports: Sales, Items and inventory, Staff | `/sales`, `/sales/items`, `/sales/staff` | D15, D16, D17 |
| `backoffice-floor.md` | Floor plan | `/floor` | Ours |
| `backoffice-printers.md` | Printers | `/printers` | Ours |
| `backoffice-settings.md` | Settings: Venue, Tax and numbering, Shift, tips and receipt | `/settings/:section` | D22 |
| `backoffice-devices.md` | Devices | `/devices` | Ours |

## Design screens with no spec

Not built, and why (`docs/phases/rebuild.md`, "Not in this phase"):

- **D06** reservations and waitlist, **D21** customers and offers: out of
  v1's scope.
- **D13** notifications, **T07** requests inbox: they need the guest channel
  (phase 11). Nothing rings the bell until then.
- **D14** offline sync is not a screen of its own: its word for the
  connection ("Online", or what is wrong) is in each app's frame
  (`terminal-shell.md`, `backoffice-shell.md`).
- **D24** setup wizard: phase 12.
- **M01 to M11**, the guest's phone: phase 11, built from the mez file then.

## Naming

`<surface>-<screen>.md`. A file keeps its name when its screen's name
changes, because code comments cite it: `terminal-register.md` is the order
screen, `terminal-tender.md` is Payment, `terminal-past-bills.md` is Orders,
`backoffice-people.md` is Staff, `backoffice-stock.md` is Inventory.
