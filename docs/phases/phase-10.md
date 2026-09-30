# Phase 10 — Back office

## Goal
The owner can run the venue without touching a database: menus, items, stock,
people, reports, settings.

## Depends on
Phase 09.

## Tasks

- [ ] **Task 1 — Back office shell.**
  Paper surface. Navigation, dashboard skeleton, permission-gated sections.
  Spec it first. This surface is keyboard-driven and data-dense — different
  rules from the terminal, and that is correct.

- [ ] **Task 2 — Menu and item editor.**
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
  - [ ] **Task 2d: Availability and photos.** Sold out for today, apart
    from retiring an item; item photos stored beside the database and
    shown on the register.
    - [ ] **Task 2d1: Sold out for today.** Marked in the editor, shown on
      the register as sold out until the next trading day starts, with no
      menu edit needed to undo it the next morning.
    - [ ] **Task 2d2: Item photos.** Upload in the editor, stored beside
      the database, served with the menu, shown on the register tiles.

- [ ] **Task 3 — Inventory basics.**
  Stock items, recipes mapping menu items to stock, deduction on sale, manual
  adjustments, wastage with reason codes, low-stock alerts, closing stock at
  shift end. Keep it basic — purchasing and suppliers stay out of scope.

- [ ] **Task 4 — People.**
  Staff CRUD, roles, permissions, PIN reset. An audit view of who voided,
  comped, discounted and refunded, filterable by staff and date. This is one of
  the highest-value features for an owner; do not bury it.

- [ ] **Task 5 — Reports.**
  Sales over time, product mix, category performance, hourly heatmap, payment
  mix, channel mix, staff performance, discount and void analysis. Cursor
  paginated. Every number mono and right-aligned. CSV export on every report.

- [ ] **Task 6 — Settings.**
  Venue details, tax profile selection and configuration, invoice series,
  currency and rounding rule, timezone, financial year start, printers and
  station mapping, device management, receipt templates, shift and tip rules,
  backup configuration.

## Done when
A new venue can be configured end to end from back office — menu, tax, printers,
staff, settings — and trade a full day, without anyone editing a config file.
