# Back office: Shell

The frame every back office screen sits in: where you are, where else you
can go, and only the places you are allowed to go. Built in phase 10 Task 1
(ADR-105). Replaces the row of buttons that was the back office home.

## Purpose

An owner or manager at a laptop moves between menu, stock, people, reports
and settings without hunting, and never lands on a page that will only
answer "you do not have permission".

## Rules for this surface

The back office is not the till. It is paper, dense and keyboard-driven,
and that is correct:

- Rows and tables are `--text-sm` where the terminal is `--text-base`.
- Every section is reachable from the keyboard (see Keys). Focus is always
  visible.
- Controls stay 48px: the back office is used on a tablet as often as a
  laptop. Density comes from text size and rhythm, not from small targets.
- Numbers tabular and right-aligned, as everywhere.

## Layout

Paper surface, inside `SignedInShell` (header: the logo, then `<section> ·
<device>`, the connection, the staff member, **Sign out**).

- **Navigation**, a column 240px wide on the left of the page, scrolling on
  its own; above the content as a wrapped row on a narrow screen.
  Grouped, each group under a meta label:
  - (no label) **Dashboard**
  - **Run the venue**: Menu, Stock, People
  - **Reports**: Z-reports, Sales reports
  - **Settings**: Floor plan, Printers, Venue settings, Devices
- Each entry is 48px, sentence case, with its key chord on the right in
  tabular, faint: `g m`. The current one has `aria-current="page"`, the
  terracotta fill and semibold text, as a pill. Terracotta is the
  sidebar's voice (tokens.md); green is kept for the action that commits.
- An entry whose screen is not built yet is muted, carries a faint
  uppercase **SOON** label, and is not a link. Not a `StatusChip`: "not
  built" is not a status of anything the venue owns. Its task sets
  `built: true` in `nav-logic.ts`.
- **Content** fills the rest and scrolls on its own, padded 32px.

## Who sees what

| Entry | Path | Needs | Built in |
|---|---|---|---|
| Dashboard | `/` | signed in | this task |
| Menu | `/menu` | `menu.edit` | Task 2 |
| Stock | `/stock` | `stock.manage` (ADR-111) | Task 3 |
| People | `/people` | `staff.manage` | Task 4 |
| Z-reports | `/reports` | `reports.view` | phase 09 |
| Audit | `/audit` | `reports.view` | phase 10, Task 4d |
| Sales reports | `/sales` | `reports.view` | Task 5 |
| Floor plan | `/floor` | `settings.edit` | phase 08 |
| Printers | `/printers` | `settings.edit` | phase 07 |
| Venue settings | `/settings` | `settings.edit` | Task 6g |
| Devices | `/devices` | `device.manage` | Task 6k |

- An entry you lack the permission for is **not shown**. A group with no
  entries left is not shown either. A cashier signed in here sees
  Dashboard alone.
- Typing the path of a section you may not open shows **"That is not yours
  to open"**, the permission it needs in the numeral face, and **Back to the
  dashboard**. The screen's own requests are not made.
- The permissions are the ones the server sent with the session
  (`GET /api/v1/sessions/current`, `POST /api/v1/sessions`). Hiding is a
  courtesy: every endpoint still checks for itself.

## Dashboard

The skeleton the reports task fills.

- The title "Back office" (the serif), and under it the signed-in
  name: "Signed in as Asha".
- **Today**: a `Card` saying "Sales, covers and open tickets for today
  arrive with reports." No placeholder numbers: a dash that looks like a
  zero is a lie on a money screen.
- **Low stock** (phase 10, Task 3d): for whoever has `stock.manage`,
  what is at or under its level (`backoffice-stock.md`).
- **Your sections**: one `Card` per group you can see, each entry with a
  one-line description and a link to it. The same list as the navigation,
  so the two never disagree.

## Keys

| Keys | Does |
|---|---|
| `g` then a letter | Go to that section: `d` Dashboard, `m` Menu, `k` Stock, `p` People, `r` Z-reports, `a` Audit, `s` Sales reports, `f` Floor plan, `t` Printers, `v` Venue settings, `e` Devices |
| `Tab` | Moves through the navigation, then the content |

- The second key must come within 1.5 seconds of `g`, or the `g` is
  forgotten.
- Ignored while typing: in an input, a select, a textarea or anything
  editable, and whenever Ctrl, Alt or Meta is held.
- A chord for a section you cannot see or that is not built does nothing.

## States

- **Loading, offline:** the shell stays; each screen says its own words.
- **Permissions changed mid-session** (a role edited in Task 4): the shell
  learns at the next sign-in or reload, and until then the server refuses
  as usual.

## Reference

Built from the primitives in `docs/design/components.md` and the paper
tokens. The archive was not consulted for this task.
