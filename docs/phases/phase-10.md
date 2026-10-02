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

- [x] **Task 5 — Reports.**
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
  - [x] **Task 5e: Discount and void analysis.** Corrections over a range
    by reason, by who did them and by who authorised them (server and
    screen, with CSV), linking down to the audit trail.

    Split into two (the figures first, then the section over them):

    - [x] **Task 5e1: Corrections added up on the server.** The audit
      trail's entries over a range, grouped: by kind, by kind and reason,
      by who did them and by who authorised them, each a count and what
      they were worth. Its own route beside the trail, its CSV, generated
      types. `reports.view`. Go tests.
    - [x] **Task 5e2: The corrections section.** Spec first (in
      `backoffice-reports.md`). A section of the Sales screen over 5e1,
      with its CSV; each row links to the audit trail already filtered,
      so the audit screen learns to open on a range, a person and a kind.
  - [x] **Task 5f: Closing stock.** The close's stock counts by shift:
    expected against counted, the variance in units and at cost, with CSV.

    Split into three (stock has no cost yet, so that comes first):

    - [x] **Task 5f1: What stock costs.** A stock item's cost, per
      kilogram, litre or each, entered on the Stock screen; every stock
      movement keeps the cost as it was when it was written, so a past
      variance is never valued at today's cost. Server and the item
      form. Go tests.
    - [x] **Task 5f2: Closing stock on the server.** The close's counts
      by shift over a range of trading days: expected, counted, the
      variance in units and at the cost kept with the count; its CSV.
      `reports.view`. Go tests.
    - [x] **Task 5f3: The closing stock section.** Spec first. A section
      of the Sales screen over 5f2, with its CSV.

- [ ] **Task 6 — Settings.**
  Venue details, tax profile selection and configuration, invoice series,
  currency and rounding rule, timezone, financial year start, printers and
  station mapping, device management, receipt templates, shift and tip rules,
  backup configuration.

  Split into twelve (the write paths on the server first, then the
  screens over them). Printers and their station mapping, and the floor
  plan, are built already (phases 07 and 08) and stay where they are.
  Currency is not a setting: changing it would reinterpret every amount
  recorded, so every screen shows it and none edits it.

  - [x] **Task 6a: The settings service and venue details.** Server. One
    read of everything the venue is set to (`GET /api/v1/settings`), and
    one write path for the `settings` table that knows every key and its
    type, so a value that would later fail as a venue config error is
    refused when it is saved. Venue details through it: name, legal
    name, timezone (must load as an IANA zone). A venue's name readable
    by any paired device, for the lock screen. `settings.edit`. Go tests.
  - [x] **Task 6b: Tax profile and tax classes.** Server. Choose the
    profile and what it needs (registration id, pricing inclusive or
    exclusive, state code, jurisdiction), validated per profile. Create,
    rename, re-rate and retire tax classes; a rate change is for new
    lines only. A profile that needs HSN reports the items without one
    rather than refusing the change. Go tests.
  - [ ] **Task 6c: Invoice series, financial year and rounding.** Server.
    Series code, prefix and number template for invoices and credit
    notes, the financial year's first month, the cash rounding step. A
    template must parse and fit the profile's length limit; no change
    may ever make an issued number come round again. Go tests, in
    `internal/domain/invoice` where the rule lives.
  - [ ] **Task 6d: Shift and tip rules.** Server. The trading day's start
    becomes a venue setting, read by every place that uses the constant
    today (shifts, reports, audit, sold out); a change takes effect from
    the next shift opened. The variance threshold, the tip rule and its
    role weights, the minutes before a ticket is late. Go tests.
  - [ ] **Task 6e: Stations and courses.** Server. Create, rename, order
    and retire stations and courses, through the menu's write path so the
    version bumps and tills hear of it. A station with items or a
    printer mapped cannot be retired without saying where they go.
    Go tests.
  - [ ] **Task 6f: Receipt template.** Server. Header and footer lines
    and what a receipt shows beyond what the tax profile requires, kept
    as settings and read by the receipt and bill layouts. ASCII only
    until printing is not (ADR-079): refuse what would print as "?".
    Go tests.
  - [ ] **Task 6g: The Venue settings screen.** Spec first
    (`backoffice-settings.md`), from `tokens.md` and the mez Desktop
    Management reference. The screen's frame, its sections, how a
    section saves and says it saved, and the venue details section.
  - [ ] **Task 6h: Tax and numbering on the screen.** The tax profile and
    tax classes, invoice and credit note numbering with a live preview
    of the next number, financial year and rounding. A change of profile
    lists the items it leaves without an HSN and links to them.
  - [ ] **Task 6i: Shift, tips and receipt on the screen.** Day start,
    variance threshold, tip rule and weights, late ticket minutes, and
    the receipt template with a preview at the printer's width.
  - [ ] **Task 6j: Stations and courses on the screen.** Stations beside
    their printer on the Printers screen; courses where the spec of 6g
    puts them. The menu's item form picks from both already.
  - [ ] **Task 6k: Devices.** Server and screen, spec first
    (`backoffice-devices.md`). Every paired device with its kind and
    when it was last seen; rename, revoke, and issue a pairing code for
    a kind. A back office cannot revoke itself. `device.manage`. Split
    it again if the server half fills a session.
  - [ ] **Task 6l: Backup configuration.** Waits for phase 12, Task 4:
    there is nothing to configure until Litestream is in the binary.
    Build it after that task, or move it there (ADR-129).

## Done when
A new venue can be configured end to end from back office — menu, tax, printers,
staff, settings — and trade a full day, without anyone editing a config file.
