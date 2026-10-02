# Phase 10 — Back office

## Goal
The owner can run the venue without touching a database: menus, items, stock,
people, reports, settings.

## Depends on
Phase 09.

## Tasks

- [x] **Task 1 — Back office shell.**
  Paper surface. Navigation, dashboard skeleton, permission-gated sections.
  Spec it first. This surface is keyboard-driven and data-dense — different
  rules from the terminal, and that is correct.

- [x] **Task 2 — Menu and item editor.**
  Categories, items, modifier groups, pricing, tax class and HSN, station
  assignment, availability, item photos. A price change takes effect for new
  lines only — open tickets keep their snapshot. Make that visible in the UI so
  the owner understands it.

  Split into four (the schema is phase 02's; nothing writes it over HTTP):

  - [x] **Task 2a: Categories and items on the server.** A menu write path
    that cannot forget the version bump or the stream nudge; the back
    office's read of the whole menu, retired rows included; create, edit,
    retire and bring back categories and items (price, tax class, HSN,
    station, course, order). `menu.edit`. Go tests.
  - [x] **Task 2b: The editor for categories and items.** Spec first
    (`backoffice-menu.md`). Categories down the side, items in a dense
    table, an item form. A price change says, where it is made, that open
    tickets keep the price they were rung up at.
  - [x] **Task 2c: Modifier groups.** Server and screen: groups, their
    choices and bounds, and which items offer them.
  - [x] **Task 2d: Availability and photos.** Sold out for today, apart
    from retiring an item; item photos stored beside the database and
    shown on the register.
    - [x] **Task 2d1: Sold out for today.** Marked in the editor, shown on
      the register as sold out until the next trading day starts, with no
      menu edit needed to undo it the next morning.
    - [x] **Task 2d2: Item photos.** Upload in the editor, stored beside
      the database, served with the menu, shown on the register tiles.

- [x] **Task 3 — Inventory basics.**
  Stock items, recipes mapping menu items to stock, deduction on sale, manual
  adjustments, wastage with reason codes, low-stock alerts, closing stock at
  shift end. Keep it basic — purchasing and suppliers stay out of scope.

  Split into four (a ledger first, so every later part only appends to it):

  - [x] **Task 3a: Stock items and the stock ledger.** Server. Stock
    items with a unit and a low-stock level; an append-only ledger of
    movements (received, adjusted, wasted with a reason code, counted),
    on hand as its sum; a `stock.manage` permission. Go tests.
  - [x] **Task 3b: Recipes and deduction on sale.** Which stock each menu
    item and modifier uses, and sold movements written when an order
    closes, in the same transaction (closed, not invoiced: an all-comp
    bill closes with no invoice, ADR-112). What a void, comp or credit
    note does to stock.
  - [x] **Task 3c: The Stock screen.** Spec first
    (`backoffice-stock.md`). Stock items with on hand, receive, adjust,
    waste with a reason, a movement history, and recipes on the item form.
  - [x] **Task 3d: Low stock and closing stock.** Low-stock alerts where a
    manager sees them; a stock count as part of closing the shift.

- [x] **Task 4 — People.**
  Staff CRUD, roles, permissions, PIN reset. An audit view of who voided,
  comped, discounted and refunded, filterable by staff and date. This is one of
  the highest-value features for an owner; do not bury it.

  Split into four (people first, then the trail they leave):

  - [x] **Task 4a: Staff and roles on the server.** Add, rename, move to
    another role, deactivate and bring back staff; set and reset a PIN;
    roles with their permission sets. Guards: the venue always keeps
    someone active who can manage staff, and a deactivated person's
    sessions end. `staff.manage`. Go tests.
  - [x] **Task 4b: The People screen.** Spec first (`backoffice-people.md`).
    Staff with their roles, add and edit, PIN reset, and a role editor
    showing each permission in words.
  - [x] **Task 4c: The audit trail on the server.** Every void, comp,
    discount and refund, with who did it, who authorised it, why, the
    order and the amount; filterable by staff and date, cursor paged.
  - [x] **Task 4d: The audit screen.** Its own entry in the navigation,
    not a tab inside People: the trail an owner reads most.

- [ ] **Task 5 — Reports.**
  Sales over time, product mix, category performance, hourly heatmap, payment
  mix, channel mix, staff performance, discount and void analysis. Cursor
  paginated. Every number mono and right-aligned. CSV export on every report.

  Split into six (the figures on the server first, then the screens over
  them):

  - [x] **Task 5a: Sales reports on the server.** One pass over the orders
    touched in a range of trading days, through the Z-report's own builder
    so a day's report and its Z-report agree: the range's totals, sales
    over time (by day, week or month), product mix, category performance,
    payment mix, channel mix, staff performance and an hour by weekday
    heatmap. `reports.view`. Go tests.
  - [x] **Task 5b: CSV on the server.** Every report of 5a, and the audit
    trail, as a CSV download a spreadsheet opens: money in major units with
    the currency named once, days as trading days.
  - [x] **Task 5c: The Sales screen.** Spec first (`backoffice-reports.md`).
    A range picker, the headline figures, sales over time drawn without a
    chart library, payment and channel mix.
  - [x] **Task 5d: What sold, who sold it, and when.** Product mix,
    category performance and staff performance tables, and the hourly
    heatmap, each with its CSV.
  - [ ] **Task 5e: Discount and void analysis.** Corrections over a range
    by reason, by who did them and by who authorised them (server and
    screen, with CSV), linking down to the audit trail.

    Split into two (the figures first, then the section over them):

    - [x] **Task 5e1: Corrections added up on the server.** The audit
      trail's entries over a range, grouped: by kind, by kind and reason,
      by who did them and by who authorised them, each a count and what
      they were worth. Its own route beside the trail, its CSV, generated
      types. `reports.view`. Go tests.
    - [ ] **Task 5e2: The corrections section.** Spec first (in
      `backoffice-reports.md`). A section of the Sales screen over 5e1,
      with its CSV; each row links to the audit trail already filtered,
      so the audit screen learns to open on a range, a person and a kind.
  - [ ] **Task 5f: Closing stock.** The close's stock counts by shift:
    expected against counted, the variance in units and at cost, with CSV.

- [ ] **Task 6 — Settings.**
  Venue details, tax profile selection and configuration, invoice series,
  currency and rounding rule, timezone, financial year start, printers and
  station mapping, device management, receipt templates, shift and tip rules,
  backup configuration.

## Done when
A new venue can be configured end to end from back office — menu, tax, printers,
staff, settings — and trade a full day, without anyone editing a config file.
