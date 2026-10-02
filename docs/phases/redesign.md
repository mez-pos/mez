# Redesign - the mez design language

## Goal

Every screen wears the mez design language (`docs/design/tokens.md`,
`docs/design/reference/mez/`). Looks only: no function changes. ADR-128.

This runs beside phase 10. Read `docs/design/tokens.md` and the one
reference file a task names. Do not read the whole reference folder.

## Tasks

- [x] **Task 1 - Rename.** Nofiltr POS becomes mez (ADR-127).
- [x] **Task 2 - Foundation.** Tokens, preset, fonts, logo, the shared
  primitives (Button, Card, StatusChip, Modal, Sheet, fields), the shell
  header, lock, pair and splash, the back office navigation.
- [x] **Task 3 - The remaining primitives.** NumPad, Qty, SearchField,
  Toast, EmptyState, Timeline, DataTable (52px rows, soft dividers, hover
  fill, no striping), ConnectionIndicator. Rewrite `tokens-demo.html`.
- [x] **Task 4 - Register and tender.** `mez Desktop POS.dc.html`:
  category chips as pills with a green active fill, menu cards, the
  ticket rail, the tender screen, modifiers sheet.
- [x] **Task 5 - Floor, past bills, print queue, close the day.** Table
  cards and status pills from the same file.
- [ ] **Task 6 - Kitchen display.** `mez Desktop KDS and Dashboard.dc.html`,
  espresso night: column headers, the ticket's left rule by age, timer
  pill. Read it from three metres.
- [ ] **Task 7 - Back office.** `mez Desktop Management.dc.html`:
  dashboard stat cards, menu, stock, people, reports and charts (green,
  terracotta, mustard, in that order), audit, Z-report, printers, floor.
- [ ] **Task 8 - Guest.** `mez Mobile Customer.dc.html`, inside the 60 KB
  budget.
- [ ] **Task 9 - Screen specs.** Bring `docs/design/screens/*.md` in line
  with what was built, and retire the old archive's citations.

## Done when

A person can walk register, kitchen, payment, invoice and Z-report and
see no screen in the old language, and `make check` and `make build-web`
pass.
