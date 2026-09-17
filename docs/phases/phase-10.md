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
