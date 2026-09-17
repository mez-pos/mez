# Phase 06 — Terminal: the register

## Goal
A staff member can open a ticket, add items with modifiers, edit the cart, hold
and resume tickets, and see the order timeline. Not yet take payment.

## Depends on
Phase 05.

## Tasks

- [ ] **Task 1 — Menu data and read API.**
  Migration for `categories`, `items`, `modifier_groups`, `modifiers`,
  `item_modifier_groups`, `stations`, `courses`, including HSN and tax class per
  item. Seed data: a realistic cafe menu with drinks, food, modifiers and two
  stations. Read endpoints, cached client-side and invalidated over WebSocket.

- [ ] **Task 2 — Register screen.**
  Spec it first into `docs/design/screens/terminal-register.md` from the design
  reference. Then build: category rail, item grid, search. The running total is
  always visible and never scrolls away. Every target 44px minimum. Measure the
  taps to add a common item — if it is more than two, the layout is wrong.

- [ ] **Task 3 — Modifiers.**
  `ModifierSheet`: required and optional groups, min/max selection, price deltas
  shown live via `Money`, defaults preselected. Blocks confirmation until
  required groups are satisfied. Editing an existing line reopens with current
  selections.

- [ ] **Task 4 — Cart.**
  Line list with quantity controls, per-line notes, void with a reason code and
  authorisation, comp, line and order discounts. Every mutation emits the right
  domain event — check against `docs/02-domain.md`. Voided lines stay visible,
  struck through. The subtotal, tax and total block uses `Money` throughout.

- [ ] **Task 5 — Open tickets and hold.**
  Multiple concurrent open tickets. A ticket list with table, covers, elapsed
  time and total. Switch between them without losing cart state. Hold and
  resume. Elapsed time in mono, and past a venue-configured threshold it turns
  vermilion — this is a "now" signal, which is what vermilion means.

- [ ] **Task 6 — Order timeline.**
  Renders the event stream via the `Timeline` primitive: who did what, when, on
  which device, with reasons for voids and comps. This falls almost entirely out
  of the event model. Read-only.

## Done when
A full order — multiple items, modifiers, a void with reason, a discount, held
and resumed — is built at the terminal, survives a page reload, and appears
correctly on a second paired device in real time.
