# Back office: the frame

What is around every back office screen: where you are, where else you can
go, and only the places you are allowed to go. Built in phase 10 (ADR-105) and
rebuilt from D12's frame in the rebuild's Task 16a (ADR-182).
`web/backoffice/src/App.tsx` and `shell/`: `BackOfficeShell.tsx`,
`SectionNav.tsx`, `SectionGate.tsx`, `nav-logic.ts`; the profile menu and the
connection word are `web/shared/shell/`. Under `/backoffice`.

## Purpose

An owner or manager at a laptop moves between menu, stock, people, reports and
settings without hunting, and never lands on a page that will only answer
"you do not have permission".

## Rules for this surface

The back office is not the order screen. It is paper, dense and
keyboard-driven, and that is correct:

- Every section is reachable from the keyboard (see Keys). Focus is always
  visible.
- Controls stay 48px: the back office is used on a tablet as often as a
  laptop. Density comes from text size and rhythm, not from small targets.
- Numbers tabular and right-aligned, as everywhere.
- A list of things the venue owns is a table in a card, with a chip for
  where each stands and what can be done to it under the row's "..."
  (`RowMenu`). The page head is the title, a line of counts, and the action
  at the right.

## Layout

The frame is the height of the screen and never scrolls; the sidebar and the
screen each scroll on their own.

- **The bar, 72px, chrome:** the logo; at the right the connection as a word
  ("Online", or what is wrong); the venue's date and time in a pill, to the
  second, in the venue's zone, from 1024px, and between midnight and the day
  start the trading day it still is where the date was, "Trading day 31 Oct
  · 1:05 AM" (BO-3, ADR-385); under 1024px the trading day alone, "31 Oct";
  the profile (initials, name,
  role), whose menu is **Sign out**; and a script line from 1536px.
- **On a mirror** (ADR-329), the bar says so before the connection's word:
  "Mirror, as of 23:10", an info chip with the history icon, the time on
  the 24 hour clock in the venue's zone, with the day when it is not today
  there ("Mirror, as of 11 Oct, 23:10"). Under 768px it is a line of its
  own under the bar, the width of the screen. The connection's word is
  "Away from the venue" while the copy is within two minutes of the venue,
  and "Nothing new from the venue", in the warning colour with the
  no-signal icon, once it is not (ADR-342: not "offline", which a quiet
  venue is not).
- **The sidebar, 240px, chrome** (208px under 1280px): an icon and a name
  on every entry, each 48px, with its key chord at the right, faint, in the
  numeral face: `g m`, which says "Press g, then m, to open Menu" on hover;
  where the screen is taller than 820px, a line at the sidebar's foot says
  "Keys: press g, then the letter beside a section." (BO-14, ADR-388). These two widths are the frame's own and not `w-side`
  (TECH-17): an entry is a fixed run of icon, name and chord, "Sales reports
  g s" needs about 200px, and `w-side` is 184 to 191px from 1024 to 1366
  wide. The screens beside the sidebar take the token widths.
  The current one is the terracotta fill. Grouped, each group under a
  small-caps label:
  - (no label) **Dashboard**
  - **Run the venue:** Menu, Inventory, Staff
  - **Reports:** Z-reports, Audit, Sales reports
  - **Settings:** Floor plan, Printers, Settings, Devices
  - A script line at the foot, on a screen 900px tall or more.
  - **Under 768px there is no sidebar** (ADR-311). The bar has a menu
    button at its left, 48px, and the sections are a drawer it opens: from
    the left, 288px or 85% of the width, over the screen, with a scrim
    behind, and a **Close** at its top, which takes focus when the drawer
    opens (BO-18, ADR-404); closed, focus goes back to the menu button.
    Choosing a section, Close, the scrim and Escape all close it. Each
    entry is one row, and no key chords are shown. The bar there is the
    button, the logo's mark alone (the whole logo from 640px), the
    connection's word, which is cut with an ellipsis before it pushes
    anything out, and the profile as its initials, with the name and role
    in the panel it opens.
- **The screen** fills the rest, padded 24px from 1280px and 16px under
  that; on a screen 820px tall or less its top padding is 16px (`short:`),
  so a 768-tall laptop keeps the 8px for the screen's content.

Sign-in and pairing are the terminal's screens (`terminal-lock.md`), on a
device paired as a back office.

## Who sees what

| Entry | Path | Needs | Keys |
|---|---|---|---|
| Dashboard | `/` | signed in | `g d` |
| Menu | `/menu` | `menu.edit` | `g m` |
| Inventory | `/stock` | `stock.manage` (ADR-111) | `g k` |
| Staff | `/people` | `staff.manage` | `g p` |
| Z-reports | `/reports`, `/reports/:n`, `/shifts/:id/tips` | `reports.view` | `g r` |
| Audit | `/audit` | `reports.view` | `g a` |
| Sales reports | `/sales`, `/sales/items`, `/sales/staff` | `reports.view` | `g s` |
| Floor plan | `/floor` | `settings.edit` | `g f` |
| Printers | `/printers` | `settings.edit` | `g t` |
| Settings | `/settings/:section` | `settings.edit` | `g v` |
| Devices | `/devices` | `device.manage` | `g e` |

- An entry you lack the permission for is **not shown**. A group with no
  entries left is not shown either.
- Typing the path of a section you may not open shows **"That is not yours
  to open"**, the permission it needs in the numeral face, "Ask the owner to
  add it to your role.", and **Back to the dashboard**. The screen
  underneath never mounts, so none of its requests are made only to be
  refused.
- The permissions are the ones the server sent with the session. Hiding is a
  courtesy: every endpoint still checks for itself.
- The sidebar, the dashboard's shortcuts, the chords and the guard all read
  one list (`NAV` in `nav-logic.ts`), so they cannot disagree.
- Any other path goes to the dashboard.

## Keys

| Keys | Does |
|---|---|
| `g` then a letter | Goes to that section, by the table above |
| `Tab` | Moves through the sidebar, then the screen |

- The second key must come within 1.5 seconds of `g`, or the `g` is
  forgotten.
- Ignored while typing: in an input, a select, a textarea or anything
  editable, and whenever Ctrl, Alt or Meta is held.
- A chord for a section you cannot see does nothing.

## Data

- The signed-in person and their permissions, from the auth store.
- `GET /api/v1/venue`, for the zone the clock is read in.
- The app's stream, for the connection word.
- `X-Mez-Mirror` on every answer (`web/shared/mirror.ts`), for the mirror's
  chip. On a mirror and in view, `GET /api/v1/devices/me` every 20 seconds,
  and every query read again when the copy turns out to be a newer one.
- The range the three report screens share is held here, above the routes
  (`ReportRangeProvider`), so the days picked on one are the days on the
  next.

## States

- **Loading, offline:** the frame stays; each screen says its own words. The
  bar says "Server unreachable" or "Reconnecting" in its colour and its
  word.
- **Permissions changed mid-session** (a role edited on Staff): the frame
  learns at the next sign-in or reload, and until then the server refuses as
  usual.
- **A draft left by a sidebar link, a `g` chord or Sign out:** a screen whose
  form holds a draft (Settings, the menu's item editor) is asked about first:
  "Leave without saving?", "Your changes to Filter coffee are not saved.",
  **Keep editing** and **Leave** (ADR-387). A link or chord to the screen
  already open asks nothing. Closing or reloading the tab gets the browser's
  own prompt.

## Reference

`mez Desktop KDS and Dashboard.dc.html`, D12 Dashboard (line 280), for the
bar and the sidebar. Left out, and why:

- **Search in the bar, with Ctrl + K, and the bell:** nothing answers
  either.
- **Orders and Tables in the sidebar:** they are the terminal's screens,
  another app with its own pairing. But opened on the till itself (the
  back office shares the terminal's pairing there, ADR-148, and the till's
  Menu and Reports pills lead here), the bar has a **Till** pill before the
  profile, and the profile's panel **Back to the till**, both a plain link
  to the terminal at the root. Neither is drawn on a device paired as a
  back office, where the root would be a terminal's pairing screen.
- **Customers:** out of v1.

Changed: the design draws nine entries in one run. The back office has
eleven, so they keep their groups, each under its label. Its "Menu
Management" is **Menu**.

Kept, though not drawn (rule 4): Z-reports, Audit, Floor plan, Printers and
Devices as entries of their own; the key chords; the guard's "That is not
yours to open".
