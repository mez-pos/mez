# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Rebuild - the screens follow the mez design** (ADR-143)
File: `docs/phases/rebuild.md`. It runs before phase 11. The owner ran the
build on 2026-10-03 and rejected the screens: the redesign had recoloured the
old layouts and kept their flow. Read that file's flow map, words and rules
before any screen. Phases 09 and 10 still owe their human done-when checks.

## Last completed task

**Rebuild, Task 17h1: the fixes before the walk** (ADR-197, ADR-198). Task 17h is split
in two. What writing the specs found is fixed: the Orders sheet's Close the day is gone,
so closing has one door, under the profile, for `shift.manage`; about thirty messages
still in the old words (till, ticket where an order is meant, "Refunds come with Task
6.") now say order, order screen and terminal, five of them the server's; "ticket"
stays for the kitchen's. `print/PrintStatus.tsx` and `lastBumped` are deleted. The
specs that quote those words follow. Tests: seven assertions moved to the new words.
`make check`, vitest (1425) and `make build-web` pass. Not tried in a browser.

Before it, **Rebuild, Task 17g2: the specs for Settings, Printers, Devices, Floor plan, Tips and
the Z-report.** Task 17g is done, and with it every spec: 28 files in
`docs/design/screens/`, each written from the code as it stands (ADR-196).
`backoffice-settings.md` (D22's frame and its three sections), `backoffice-printers.md`
and `backoffice-devices.md` (Task 16k's tables), `backoffice-floor.md` and `tips.md`
(their layouts were left as they were, ADR-195), and `zreport.md` (D23's report and
slip in both apps, the list and its exports). `screens/README.md` lists every spec
against its screen, path and design screen, and the design screens that have none.
Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 17g1: the specs for Menu, Inventory and Staff.** Task 17g is split in
two (`rebuild.md`). Written from the code as it stands (ADR-196): `backoffice-menu.md`
(D18: the categories, the list and the inline editor, with the Choices and Stations and
courses views), `backoffice-stock.md` (Inventory, D19: the levels table, the forms, the
week's wastage, and Recipes) and `backoffice-people.md` (Staff and roles, D20: the
table, the role's switches, and what you may give). Each Reference was checked against
the design file's own words. Two paths in 17f's specs were put right (the stock read is
`GET /api/v1/stock`). Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 17f: the specs for the back office's frame, the dashboard, the
reports and Audit.** Written from the code as it stands (ADR-196): `backoffice-shell.md`
(the bar, the grouped sidebar, who sees what, the `g` chords, the guard),
`backoffice-dashboard.md` (new, D12), `backoffice-reports.md` (the shared range and
Export, then Sales, D15, Items and inventory, D16, and Staff, D17, each with what sits
under "In detail") and `backoffice-audit.md`. Each Reference was checked against the
design file's own words. Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 17e: the spec for the kitchen.** `kitchen-display.md`: the one
board and its two ways in (the wall display at `/kitchen`, a cook on a terminal at
`/pass`), the three columns and the ticket card, the all-day strip, today's stats,
Coming next, the foot's switches, espresso night, and who may move a ticket (D10, D11).
Its commit left this file and the phase file's tick behind; both were put right with
17f.

Before it, **Rebuild, Task 17d: the specs for the waiter's screens.** Five new files, written
from the code as it stands (ADR-196): `waiter-home.md` (T01, with seating and clearing
from a card), `waiter-order.md` (T02, Confirm and send, T04, and Course View; only what
differs from `terminal-register.md`), `waiter-table.md` (T03, with Transfer and merge,
T06), `waiter-bill.md` (T05) and `waiter-shift.md` (T08). Each Reference was checked
against the design file's own words. Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 17c: the specs for payment, Orders and the print queue.** Written
from the code as it stands (ADR-196): `terminal-tender.md` (Payment, D07; Split Bill,
D08a; the waiting, not recorded and received cards, D08b; the invoice and the paid
panel), `terminal-past-bills.md` (Orders and the refund modal, D09, with void, reprint
and bills from before today) and `terminal-print-queue.md`. Each Reference was checked
against the design file's own words. Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 17b: the specs for the order screen and Tables.** Written from the
code as it stands (ADR-196): `terminal-register.md` (the order screen, D03, its sheets,
and the item modal, D04) and `terminal-floor.md` (Tables, D05: the plan, the list, the
panel, and Move, Join and Part). Each lists what the design draws and the screen leaves
out, checked against the design file's own words. Docs only: no code changed. `make
check` passes.

Before it, **Rebuild, Task 17a: the specs for the way in and the frame** (ADR-196). Task 17 is
split in eight (17a to 17h, `rebuild.md`): seven of specs, then the walk. Written from
the code as it stands: `terminal-lock.md` (Staff Login and pairing, D01),
`terminal-shell.md` (new: where each role lands, the desktop's and the waiter's frame,
the paths, the shift gate), `terminal-open-shift.md` (new, D02) and `terminal-close.md`
(Close the day in two steps, D23). Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 16k: Printers, Devices and Audit on Task 16's patterns** (ADR-195).
Asked for by the owner before Task 17. Printers (`printers/PrinterTable.tsx`,
`PrinterRouting.tsx`) and Devices (`devices/DeviceTable.tsx`) are tables in a card
with a chip and a row menu; Audit has the reports' head and the range pill, now
`sales/RangePill.tsx`. Layout only: no server change. Seen in a browser as Priya at
1920x1080, and Printers at 1100. Tests: `printsText`, `printersSummary`, `stateOf`,
`listed`, `devicesSummary`, the `device` chips. `make check` and `make build-web`
pass.

Before it, **Rebuild, Task 16j: the Z-report and the close** (ADR-194). Task 16 is done. The
Z-report (`shared/zreport/ZReportView.tsx`, `ZPaper.tsx`, both apps) has D23's slip
beside the report in full, with "By how it was served", tips by who took the payment
and covers where a report has them. Close the day (`terminal/src/shift/
CloseDayScreen.tsx`) is two steps: count the drawer, then the Cash reconciliation card
and Close the day. Seen in a browser as Priya at 1920x1080: the count, the
reconciliation over its limit, the note, the close, and Z-report 1 on the order
screen's app and in the back office. Tests: `averageMoney`, `stampText`,
`serviceName`, `toleranceText`. `make check` and `make build-web` pass.

Before it, **Rebuild, Task 16i: Settings** (ADR-193). `/backoffice/settings/*` has D22's frame
(`settings/SettingsScreen.tsx`): the title, the rail as a card with icons, links out to
Printers, Devices and Tables and floor plan, the version box, and the section's cards
across the rest. A section still saves itself. Venue's fields are two across. Seen in a
browser as Priya at 1920x1080: the three sections, a draft and the "Leave without
saving?" prompt from a link out, Discard. Test: `relatedScreens`. `make check` and
`make build-web` pass.

Before it, **Rebuild, Task 16h: Staff and roles** (ADR-192). `/backoffice/people` is D20, called
Staff in the sidebar (`people/PeopleScreen.tsx`, `RolePanel.tsx`): the staff table
with a state chip and a row menu, and beside it the roles as pills with the chosen
one's permissions as switches, its name, Reset and Save role. The role cards and the
role edit modal are gone. Seen in a browser as Priya at 1920x1080: a person picked, a
permission switched on, reset, saved and put back, and the row menus. Tests:
`peopleSummary`, `staffRows`, `appliesText`, `roleChanged`, `chosenRole`. `make check`
and `make build-web` pass.

Before it, **Rebuild, Task 16g: Inventory** (ADR-191). `/backoffice/stock` is D19, now called
Inventory (`stock/StockScreen.tsx`, `StockLevels.tsx`, `WastageCard.tsx`,
`levels-logic.ts`): the ingredients with what is in stock, the level each is low at,
its cost, a level bar and a Healthy, Low or Out chip; a row's actions under its "...";
Stock count and Log wastage beside the title; the week's wastage beside the table.
Recipes is still a view of its own. Seen in a browser as Priya at 1920x1080: the
table, Log wastage asking which ingredient, a row's menu, and stock received. Tests:
`levelOf`, `levelPercent`, `summary`, `tableRows`. `make check` and `make build-web`
pass.

Before it, **Rebuild, Task 16f: Menu management** (ADR-190). `/backoffice/menu` is D18
(`menu/MenuScreen.tsx`, `CategoryRail.tsx`, `ItemList.tsx`, `ItemEditor.tsx`,
`browse-logic.ts`): categories with counts, the items with a chip for where each
stands, and the item open in an inline editor with its photo and tags beside it; the
modal form and the table are gone. Search, a filter and an order on the list. Choices
and Stations and courses are still views of their own. Seen in a browser as Priya at
1920x1080 and 1366x768: an item edited and saved, a search, Sold out today and back
from a row's menu, a new item refused for its HSN and then added, and retired. Tests:
`scopeRows`, `itemState`, `listItems`, `canReorder`, `emptyLine`. `make check` and
`make build-web` pass.

Before it, **Rebuild, Task 16d3: an item's cost and margin** (ADR-189). Task 16d is done. The owner
chose the ledger over today's recipe. A sold stock movement keeps its dish (migration
00035) and the ledger keeps how many of a dish it costed (`stock_dish_sales`, 00036);
`GET /api/v1/reports/items` gives each item's net sales, cost and margin, and a margin
only when the ledger costed every unit sold. Item performance has Net sales, Cost and
Margin (`sales/ItemsScreen.tsx`). Seen in a browser as Priya at 1920x1080: a dish fully
costed with its margin, one part costed, several not costed, and a trend arrow. Go
tests in the domain, both services and over HTTP; vitest for the rows and words. `make
check` and `make build-web` pass.

Before it, **Rebuild, Task 16e: Staff reports** (ADR-188). `/backoffice/sales/staff` is D17's staff
half (`sales/StaffScreen.tsx`, `staff-logic.ts`), the third report tab: staff
performance (role, orders, sales, average, tips), Removed in error, the tips total, and
"Who sold it" in full, moved from Sales reports. The report carries `tips_by`, tips by
who took the payment (16e1, server). Seen in a browser as Priya at 1920x1080 over seven
days. Go tests in the domain; vitest for `staffLines` and `removedLines`. `make check`
and `make build-web` pass.

Before it, **Rebuild, Task 16d2: Items and inventory** (ADR-187). `/backoffice/sales/items` is D16
(`sales/ItemsScreen.tsx`, `items-logic.ts`), a tab beside Sales: item performance with a
trend against the days before, wastage as bars with its total, stock health, and under
"In detail" what sold in full and the closing stock, moved from Sales reports. The range
is shared by the report tabs (`report-range.tsx`, `use-range-sales.ts`). Seen in a
browser as Priya at 1920x1080: seven days and Today, the tabs keeping the days, four
write-offs keyed into the dev venue. Tests: `itemRows`, `wasteBars`, `stockHealth`.
`make check` and `make build-web` pass. Cost and Margin are split off as 16d3.

Before it, **Rebuild, Task 16d1: the wastage report** (ADR-186). `GET
/api/v1/reports/wastage`: write-offs over a range of trading days by stock item and by
reason, at the cost each had. Go tests in the service and over HTTP.

Before it, **Rebuild, Task 16c2: Sales reports** (ADR-185). Task 16c is done. `/backoffice/sales` is
D15 (`sales/SalesScreen.tsx`, `SalesCharts.tsx`, `SalesHead.tsx`, `overview-logic.ts`):
the title with the range pill and Export, four figures against the same number of days
before, the trend stacked by service, sales by category and orders by type as rings,
top sellers and payments; the old tables sit under "In detail". Seen in a browser as
Priya at 1920x1080 and 1366x768: seven days, Today picked from the range pill, the
Export list. Tests: `previousRange`, `rangeText`, `trend`, `namedBars`, `slices`,
`ringGradient`. `make check` and `make build-web` pass.

Before it, **Rebuild, Task 16c1: the report by service type** (ADR-184). `by_service` on the
sales report's totals and on each period, and `sales.covers`, the guests on the bills.
Server only. Go tests in the domain.

Before it, **Rebuild, Task 16b: the dashboard** (ADR-183). Task 16 is split in ten (16a to 16j,
`rebuild.md`). `/backoffice/` is D12 (`dashboard/Dashboard.tsx` and its parts,
`dashboard-logic.ts`): today's sales, orders, active tables and average order beside
yesterday's, sales by the hour, top selling items, active tables, staff, low stock and
quick actions, each card for whoever may read it. No new endpoint; the sales report now
carries `roles` beside `names`. Seen in a browser as Priya at 1920x1080 and 1366x768
against the dev venue. Go test for the roles; vitest for `change`, `occupancy`,
`activeTables`, `topItems`, `staffRows`, `hourChart`, `axisTop`, `lowRows`,
`formatMoneyCompact`. `make check` and `make build-web` pass.

Before it, **Rebuild, Task 16a: the back office's frame** (ADR-182). The 72px bar (logo,
Online, the venue's date and time, who is signed in with Sign out under the name) and
the 240px sidebar with an icon on every entry (`shell/BackOfficeShell.tsx`,
`shell/SectionNav.tsx`). `ConnectionWord`, `ProfileMenu`, `use-disclosure` and
`status-logic` moved to `web/shared/shell`; `SignedInShell` is deleted. Seen in a
browser: the menu under the name, and Menu opened from the sidebar.

Before it, **Rebuild, Task 15f: Coming next** (ADR-181). Task 15 is done. Where D10 draws
"Upcoming", the board lists what the station will be sent when a later course is
fired: `order.Held` (`internal/domain/order/route.go`), `coming` and `coming_more` on
the board's response, and a card under Today's stats (`kitchen/BoardStats.tsx`,
`comingRows`). Seen in a browser at 1920 wide, on night: T2's starter sent and its
dessert listed as coming. Go tests in the domain, the service and over HTTP;
`comingRows` in vitest. `make check` and `make build-web` pass.

Before it, **A fix after 15e: moving a ticket is its own permission** (ADR-180, migration
00034). `kitchen.work` starts, finishes and recalls tickets; `kitchen.view` only reads
the board. The owner, the manager and the kitchen role hold it; a cashier and a waiter
do not, and the kitchen's API refuses them a move. On `/pass` someone without it sees
the board with no buttons. Seen in a browser: Ravi starting a ticket, Kabir's board
without actions. Go tests for the routes, the default roles and the migration.

Before it, **Rebuild, Task 15e: a cook signed in on a terminal has the board** (ADR-179). The
kitchen role lands on `/pass` (`kitchen/CookBoard.tsx`): the wall display's board with
the person's own session, outside the shell and the shift gate, with "Lock · name" in
the foot. The kitchen's API now also lets in a person signed in on any device who
holds `kitchen.view` (`kitchenAccess` in `internal/api/kitchen.go`), and their moves
carry their name. `/kitchen` is still the wall display with its own pairing. Seen in a
browser as Ravi: sign-in, the station chosen, a ticket started, night, and Lock back to
a paper sign-in. Go test over HTTP; `landingFor` in vitest. `make check` and `make
build-web` pass.

Before it, **Rebuild, Task 15d: today's stats** (ADR-178). The board's response carries
`stats` for the station's trading day: tickets sent, in the kitchen and ready, the
mean time from sent to ready, and the slowest by its dish (`kitchen.Stats` in
`internal/domain/kitchen/day.go`, the `StationDay` query, `Service.Board`). The board
shows them in a column from 1536px wide (`kitchen/BoardStats.tsx`), and as one line
under that. Seen in a browser as a kitchen display at 1920 wide against the dev venue.
Go tests in the domain, the service and over HTTP; `statRows` and `minutesText` in
vitest. `make check` and `make build-web` pass. The design's Upcoming is left out and
split off as 15f.

Before it, **Rebuild, Task 15c: espresso night and the board's foot** (ADR-177). The board
has a foot (`kitchen/BoardFoot.tsx`): Change station, Sound, Group by table and Night
mode, each kept on the device (`board-prefs.ts`, `mez.kitchen.prefs`). Night is the
same board on the ink surface (D11). Sound is a two-note chime for a new ticket
(`use-chime.ts`). Seen in a browser as a kitchen display at 1920x1080: night on and
off, the switches, and the choices kept across a reload. Tests: `readPrefs`,
`writePrefs`, `surfaceOf`, `groupByTable`, `arrivals`. `make check` and `make
build-web` pass.

Before it, **Rebuild, Task 15b: the kitchen's board** (ADR-176). `/kitchen` is D10 on paper
(`kitchen/KitchenBoard.tsx`, `TicketCard.tsx`, `boardColumns` in `kitchen-logic.ts`):
the header, the all-day strip, and three columns, New Orders with Start preparing,
Preparing with Mark ready and a bar towards late, Ready for pickup with Recall. Seen
in a browser as a kitchen display at 1920x1080 and 1366x768 against the dev venue: a
ticket started, marked ready and recalled. Tests: `boardColumns`, `metaText`,
`latePercent`, `agoText`. `make check` and `make build-web` pass.

Before it, **Rebuild, Task 15a: "Start preparing", from the server** (ADR-175). Task 15 is
split in five. A ticket a cook has picked up is in progress: `ticket.started`, `POST
/api/v1/kitchen/orders/{id}/tickets/{seq}/start`, and the board's tickets carry
`started_at`. A bump still works from fired; a recall puts a ticket back as new.
`ProjectionVersion` is 8. No migration. Server only: the display has no Start button
yet, and keeps showing a started ticket as one to make. Go tests in the domain, the
service, the hub and over HTTP. `make check` passes.

Before it, **Rebuild, Task 14c: the shift summary** (ADR-174). Task 14 is done. `/waiter/shift`
is T08 (`waiter/ShiftSummary.tsx`, `shift-summary-logic.ts`), under More, "Your shift":
who is signed in and how long they have been on, tables served, orders taken, sales
handled, tips collected, tips by table as bars, and End shift, which signs them out.
Seen in a browser at 1366x1024 as Kabir against the dev venue: the figures after the
bills of 14a, then a card payment with a tip at T3 and its bar. Tests: `shiftSpan`,
`onShiftText`, `tipBars`. `make check` and `make build-web` pass.

Before it, **Rebuild, Task 14b: one person's shift, from the server** (ADR-173). 14b was split
again: the server could say nothing per person. `GET /api/v1/shift/mine` answers for
whoever asks, in the open shift: the orders they opened, how many ended at a table,
what their payments settled, the tips on them, those tips by table, and when they first
touched an order (`shift.MineOf` in `internal/domain/shift/mine.go`, `Service.Mine`,
`MyShiftResponse` in `api-types.ts`). Server only, no screen. Go tests in the domain,
the service and over HTTP. `make check` passes.

Before it, **Task 14a: the waiter's bill** (ADR-172). Task 14 is split in two. For a
waiter `/pay/:id` is T05 (`waiter/WaiterBill.tsx`, `BillShares.tsx`, `BillCollect.tsx`,
`bill-logic.ts`): one column with the table and its total, Split equally with the
guests counted (it starts at one, "Whole bill"), a row for each share, the share being
collected by UPI, card or cash with a tip, what has been collected, Print receipt once
there is an invoice, and Close table, which returns to the tables. By item and Custom
open the desktop's split panel. Seen in a browser at 1366x1024 as Kabir: T5 split in
two, one share by UPI, one by cash with change, Paid in full, and Close table leaving
T5 to clear; and a bill already paid. Tests: `shareName`, `eachShare`, `shareRows`,
`tablePays`, `closeBlock`. `make check` and `make build-web` pass.

Before it, **Task 13b: transfer and merge** (ADR-170). Task 13 is done. Transfer and
Merge on the table's screen open T06 as a modal (`waiter/MoveTable.tsx`,
`move-logic.ts`): the party's items, total and guests, the two modes, every other
table as a tile (free ones can be taken, the rest say why not), and a button that says
what it will do. Seen in a browser at 1366x1024 as Kabir: T3 joined to T2 ("Table T2 +
T3", Merged), let go again, and the party moved to T5 with the screen following it.
Tests: `moveTiles`, `pickTile`, `moveLabel`, `moveActions`. `make check` and `make
build-web` pass.

Before it, **Task 13a: the table's own screen** (ADR-169). Task 13 is split in two.
`/waiter/table/:id` is T03 (`waiter/TableDetail.tsx`, `TableDetailOrder.tsx`,
`TableItems.tsx`, `OrderProgress.tsx`, `table-logic.ts`): the table with its guests and
how long it has sat, its notes and Add note, its items by course with each one's state
and Fire, Order Status in four steps, Bill and pay, Details, the totals and Send to
Kitchen (n), which opens the confirm. The top bar there is Back. A waiter's card with
guests at it leads here; a free table or one to clear still leads to the floor. Seen in
a browser at 1366x1024 as Kabir: the screen, a note added, Details, a send confirmed,
the list ungrouped, Add Item, and a free table's card. Tests: `detailOrder`,
`orderProgress`, `startedText`.

Before it, **Task 12c: Course View** (ADR-167). Task 12 is done. A waiter's rail has
two tabs, Current Order (n) and Course View (`register/CourseView.tsx`, `courseGroups`
in `send-logic.ts`).

Before it, **Task 12b: confirm and send** (ADR-166). A waiter's Send to Kitchen opens
T04 as a modal (`register/ConfirmSend.tsx`, `sendPreview` in `send-logic.ts`): how many
items go, how many the kitchen has already, how many wait for their course, the ticket
as the kitchen will read it, Back and "Send N items to kitchen"; then a toast. Seen in
a browser at 1366x1024 as Kabir: two new items beside one already sent, sent, the toast
and the button reading "Sent". Tests: `sendPreview`.

Before it, **Task 12a: the waiter's order screen** (ADR-165). Task 12 is split in three.
For a waiter `/pos` is T02, the order screen rearranged (`OrderScreen` and its parts
take a waiter's variant; `useWaiterFrame` says who): the top bar is the order's
(`waiter/WaiterOrderBar.tsx`: the way back, the table, the guests, how it is served),
the search sits over the categories, Popular is where it starts, three cards across,
and the rail has the table with its chips, each line's course, Clear, the totals,
Save and Send to Kitchen. Pay is "Bill and pay" under the rail's "...". Seen in a
browser at 1366x1024 as Kabir, beside the design: a seated table's order, items added,
Clear confirmed and carried through, Send, the "..." menu, and Save returning to the
tables. Tests: `clearableLines`, `courseNamer`. `make check` and `make build-web` pass.
The owner has not yet looked.

Before it: Task 11 (ADR-163, the waiter's home; ADR-164, the connection word in a
waiter's bar), Task 10c (ADR-162, the refund modal and the print queue on Orders), Task
10b (ADR-161, the Orders screen), Task 10a (ADR-160, the day's orders
from the server), Task 9 (ADR-159, split and the card and UPI states), Task 8 (ADR-156,
payment), Task 7 (ADR-155, tables), Task 6 (ADR-153, the item modal; 6b, ADR-154),
Task 5 (ADR-151, the order screen), Task 4 (ADR-150, Open the shift; `shift.open`,
ADR-152, migration 00033), Task 3 (ADR-149), Task 2 (ADR-148), Task 1 (ADR-145 to
ADR-147), Task 0 (ADR-144), the plan (ADR-143). The owner has not yet looked at 9, 10b,
10c, 11, 12 (a, b, c), 13 (a, b) 14 (a, c), 15 (b to f) or 16 (a to j).

## Next task

**Rebuild, Task 17h2: the walk.** In the dev server (`make dev`; `rebuild.md`, "How to
see it"), as Meera the cashier, Kabir the waiter, Ravi in the kitchen and Arjun the
manager, take an order from each side through to a paid bill, an invoice and a
Z-report, reading each screen against its spec (`screens/README.md` lists them). Fix
what breaks, or write it under Known broken. Then cut the "not seen by eye" lists below
to what is still unseen, and bring this file back under 60 lines by moving the "Before
it" history out. Split it by role if it will not fit one session. The phase's done-when
is the owner's own walk. The owner has
not yet looked at the kitchen's screens or anything of Task 16. Floor plan and Tips
are still their old layouts (ADR-195). Phase 11 waits for this phase. Phase 10's tasks are
all done (6l moved to phase 12, ADR-141); its done-when is a human check and stays
unticked below.

Agreed and built on 2026-10-05 (ADR-158): a free table has "Seat N guests" and "Seat
and order"; the totals name each tax (CGST, SGST) on the rail, the table panel and the
payment screen; Print receipt is on the paid panel. Seen in a browser at 1366x768.

Fixed on 2026-10-05, between Tasks 8 and 9 (ADR-157): a cancelled read left a pooled
read connection on an old snapshot, so reads missed rows just written until a restart.
The SQLite driver is now v1.46.1 and the module needs Go 1.24.

## Known broken / in progress

- After the fixes of 17h1 (ADR-197, ADR-198): typechecked, tested and built, none of
  it tried in a browser. In the walk, look at the Orders sheet as Meera (no Close the
  day), a refusal or two in the new words, Devices ("Terminal"), and the settings card
  "Shifts, tips and orders". Left: `/shift/close` typed by hand still lets someone with
  `payment.take` count; the printed Z-report labels its device "Till"
  (`escpos/zreport.go`); the kitchen board's clock and a ticket's sent time are read in
  the device's zone, not the venue's.
- Printers, Devices and Audit (ADR-195): the dev venue now has three printers that
  go nowhere (127.0.0.1:9197 to 9199), one retired and one set for receipts, so a
  receipt there queues and retries. Not seen by eye: a revoked or a quiet device,
  a refusal over the printers table, and Devices and Audit under 1280px.
- The Z-report and the close (ADR-194): the dev venue's first shift is now closed
  (Z-report 1, ₹282.00 over, counted by hand to try it) and a second is open with a
  ₹10,000.00 float. Not seen by eye: a width under 1920 (the slip under the report),
  a balanced drawer and one within its limit, a closing stock count on step two, a
  correction, a report from before the new figures, Print a copy carried through,
  and the Z-reports list.
- Settings (ADR-193): the Printers, Devices and Floor plan screens inside are still
  their old layouts (the design draws no screen for them). Not seen by eye: a width
  under 1920, a save, Leave carried through, the rail for someone without
  `device.manage`, and the Tax and Shift sections' cards beyond their headings.
- Staff and roles (ADR-192): no roster, shift column, phone or PIN column. An unsaved
  change to a role is dropped when another role or person is picked. After changing
  your own role the switches follow your old permissions until you sign in again.
  Not seen by eye: a width
  under 1920, a deactivated person shown and brought back, a person added, a new PIN,
  a role added, renamed and deleted, a refusal, and a manager's view (switches they
  cannot move).
- Inventory (ADR-191): no purchase orders or suppliers. A full bar is three times the
  low level, our own rule. After a count or a write-off started from the buttons
  beside the title, focus does not return to that button. Not seen by eye: a width under 1920, a retired ingredient
  shown, a count and a write-off carried through from the new buttons, History and
  Edit from a row's menu, the week's change against the week before (the dev venue
  has one week), the screen without `reports.view`, and the Recipes view after the
  change.
- Menu management (ADR-190): opening another item or leaving the screen drops an
  unsaved edit without asking, and an item changed on another screen while it is open
  is saved over. The list no longer shows each item's tax class, station, course or
  HSN (they are in the editor). No cost price, combos or drag to reorder. The dev
  venue has a retired "Test biscotti" in Cakes from the browser check. Not seen by
  eye: a photo chosen and saved, choices added to an item, Move up and down, a
  category added, renamed and retired, the Kitchen and tax tab's refusal, the
  Choices and Stations views after the change, and a link to `/menu?item=N`.
- An item's cost and margin (ADR-189): sales from before migration 00035 are never
  costed by dish; a dish with no recipe reads "Not costed". No CSV has cost. The dev
  venue now has recipes for Espresso, Cappuccino, Americano and Lemon tart and two
  orders rung up through the API on 2026-10-05; its Espresso and Cappuccino read "0
  of 2 costed" because one sale fell between the two migrations. Not seen by eye: a
  margin below zero, a range reaching before costs were kept (the note), a dish whose
  line was voided after a reopen, a failed read.
- Staff reports (ADR-188): no ratings, no customers. Tips are those taken, not the
  pool's shares. A tipped payment from before the days, voided in them, shows as a
  row below zero (it keeps the rows equal to the total). Not seen by eye: a guest's row, someone who only took payments, someone no longer
  on record, a range with nobody, a failed read, a width under 1920.
- Items and inventory (ADR-187): no purchase orders, no CSV for wastage. The dev venue now has four stock items and four write-offs keyed
  on 2026-10-05 to show the cards. Not seen by eye: a trend arrow (every item was new
  in its days), more than five write-offs, a day with none, the screen without
  `stock.manage` (no stock health card), a failed read, a width under 1920.
- Sales reports (ADR-185): no outlet or order type filter, no CSV for service types.
  "Total guests" counts only parties someone counted. Not seen by eye: a range of
  more than seven periods (the named bars), delivery on the trend, more than six
  categories ("Others"), a method that refunded more than it took, a failed read, a
  download carried through, From and To typed by hand, and "View all" scrolling to
  the items table. The range is not in the URL: a reload returns to seven days.
  In Safari and Firefox on a Mac a click on a pill may not focus it, and the shared
  disclosure can close a menu before the click lands; tried in Chromium only.
- The dashboard (ADR-183): bars are one colour (the report's hours are not cut by
  service type); "Total orders" is paid bills, not open orders; today so far is
  compared with the whole of yesterday; a table to clear counts as active. Not seen by eye: a low
  or out-of-stock row, more than five rows in a list, a day with nothing sold, a failed
  read, a manager's or a cashier's dashboard (cards by permission), a bill after 10 PM,
  and a width under 1280px.
- The back office's frame (ADR-182): no search, no bell. Not seen by eye: the bar
  with the server down, Sign out carried through, a `g` chord, a width under 768px
  (the sidebar as a row), and a screen 900px tall or more at 1366 wide.
- The board's foot (ADR-177): no Auto advance, no Mark all complete, no Settings
  screen. The chime was not heard (the check was by eye): try it on a real display,
  and after the tab has been idle. The dev server missed a file change once (Vite on
  the I: drive): touch the file if a screen looks stale. Not seen by eye: grouping
  with two tables' tickets interleaved, and night at 1366 wide.
- Moving tickets (ADR-180): a role the owner made that both sells and sees the
  kitchen is not given `kitchen.work` by the migration; the owner ticks it on People.
- A cook's board (ADR-179): nothing leads from `/pass`
  to the rest of the terminal, or back for a manager who opens it by its path. Not
  seen by eye: a role made by the owner landing there, a wall display and a signed-in
  cook at one station at once, and a manager at `/pass`.
- Coming next (ADR-181): every open order is projected on each board read; not
  timed with many open. Not seen by eye: more than five (the "more behind these"
  line), an order with no table, two courses held on one order, and the list going
  when a course is fired.
- Today's stats (ADR-178). The day is read whole on each board
  read; a busy station has not been timed. Not seen by eye: the one-line version
  under 1536px (the browser would not resize), the column on night, a day with no
  ticket ready ("Not yet"), and a slowest ticket with several dishes.
- The kitchen's board (ADR-176): a new ticket has no Mark ready of its own. The clock is the device's
  zone, not the venue's. `docs/design/screens/kitchen-display.md` describes the old
  board (Task 17 rewrites it). Not seen by eye: takeaway and delivery icons, a counter
  order, a voided line, a line's choices and notes, a calm and a Waiting timer (the dev
  venue's tickets were all late), a failed action, a column long enough to scroll at
  1080p, and two displays at one station.
- Start preparing (ADR-175): the ticket index cannot say what is in progress without
  the orders. Nobody is named as the one who started a ticket.
- The shift summary (ADR-174): no ratings, no Print summary, no area. Not seen by
  eye: several tables' bars, "No table", a table gone from the floor, a failed read,
  someone who has done nothing, and End shift carried through.
- One person's shift (ADR-173): read whole from the shift's
  orders on each ask; a busy shift has not been timed. A refund is not taken off
  "sales". A table the cashier opened counts for the cashier, not the waiter who
  served it.
- The waiter's bill (ADR-172): no Print bill before payment, no QR, no Wallet, no
  keyed tip. A toast sits over the foot's buttons for its few seconds. Not seen by eye:
  By item and Custom carried through from here, Change the split, a stale split,
  the keypad for cash, a refusal, Print receipt, an order with
  no table ("Close order"), and three or more shares (the column scrolls).
- Transfer and merge (ADR-170): a merge of several tables is one request each, in
  turn; if one is refused the ones before it stand. No note about QR sessions, no
  "Notify the kitchen". Not seen by eye: a refusal, several tables joined at once, join
  and let go in one go, a floor of more than sixteen other tables (the tiles scroll).
- The table's screen (ADR-169): a table with several orders is the floor's; a free
  one is seated and one to clear is cleared from the waiter's home (ADR-171). Notes carry no time or name (a note keeps neither). Every course wears the same
  icon. The order
  screen's back arrow returns here for an order at a table (ADR-171). Not seen by eye: a
  joined table ("Merged"), a saved order, a held course's Fire, a Ready item, a refusal,
  a table gone from the floor, and an order paid from here.
- Course View (ADR-167): the design draws the tab's name and nothing under it, so its
  layout is ours. A waiter fires only from here (ADR-168). Not seen by eye: a line
  with no course, a course "To send", a Ready line, three courses, a saved order.
- Confirm and send (ADR-166): no ticket number (the server gives it on sending), no
  choice of printer or screen, no "Notify me", no Undo. The ticket's face is Archivo,
  not a monospace. Not seen by eye: a held later course's line, an order note, a line's
  note and choices on the ticket, a refusal, and a ticket longer than the modal.
- The waiter's order screen (ADR-165): no Add More (the menu is beside the rail). The item modal is D04's, not
  T02's sheet with a photo on top. The card's "+" is terracotta, as on D03. A cleared
  line stays on the rail struck through, as a removed one does. Save leaves a table
  order open and returns to the tables; only an order with no table is parked (ADR-168).
  Items added and not sent stay unsent after Save, and the table's card does not say so. Not seen by
  eye: a new order with no table ("Pick a table"), takeaway and delivery chosen in the
  bar, a joined table's title, a failed Clear, a menu with nothing popular, the bar
  with the server down, and "Bill and pay" carried through.
- The waiter's home (ADR-163): every table is listed, not the waiter's own (no table
  is assigned to anyone), so the line reads "the tables", not "your tables". No bell,
  Table Requests, QR Orders (phase 11) or Reserved (out of scope). Active Orders counts
  every open order at the venue and opens the day's orders. A waiter has no status
  bar: no clock, venue or version on screen; the top bar says Online or what is wrong
  (ADR-164), without the retry countdown; seen by eye Online and with the server
  stopped. The other screens a waiter reaches are still the desktop's, under the
  waiter's bar (Tasks 12 to 14). Not seen by eye: a joined table's card, a floor of more
  than twelve tables (the cards scroll), a failed read, a width under 1280px (three
  columns, then two), the evening greeting, and Print queue opened from More.
- The refund modal (ADR-162): not seen by eye are a refund by amount, a full refund
  carried through, a bill with one payment, a wrong PIN, and a bill with nothing left.
  At 768px high the modal scrolls to reach Continue. As a refund is issued the modal
  asks for one more preview, which the server refuses (409) as it closes: nothing shows,
  but it is a wasted request. A refund larger than any one payment is made in parts
  (ADR-084); the modal says so and does not split it itself.
- The Orders screen (ADR-161): not seen by eye are a refunded or voided row, a bill
  from before today found by its number, a reopened bill and Take the balance, a void
  or a reprint carried through, a held order opened from here, a day with no orders,
  and a failed read. A bill paid and invoiced but not yet closed (New Order not tapped)
  reads "Open". No cashier or waiter names, no Filters, no Export, no Email (left out).
  An order from before 04:00 shows its date and wraps in the Time column. The list
  asks again on every order event from any till.
- The day's orders (ADR-160): the list projects every order it names on each ask, as
  the open tickets list does; a day of several hundred orders has not been timed. It has
  no cashier or waiter names and no payment method (the panel reads the order itself).
  An order closed with every line voided reads "voided"; no command voids an order as
  such. The day start is read as it stands when asked.
- Split and the payment states (ADR-159): not seen by eye are a stale split, a guest
  with nothing to pay, taking a guest away, a tip on a card payment's waiting card, and
  a server's own refusal (the one seen was a dropped connection). At 768px high the
  unassigned items sit below the fold of the mode column, which scrolls. Two lines of
  the same item have the same spoken name on their "Give" buttons. A split made and its
  shares paid are lost on a reload, as before. No QR and no card machine (left out).
- The payment screen (ADR-156): not seen by eye are a tip keyed on the keypad, a cash
  refusal, a discount given from here, a table order's title with a joined table, and "Issue the
  invoice first". At 1366x768 the payment step scrolls to reach the keypad.
  After the review: the quick amounts count the tip ("exact" is the bill and the tip),
  a tip is cleared when a split is taken or another share picked, and an amount field
  is named with its amount; typechecked and unit-tested, not tried in a browser. Left:
  on a tiny bill two tip percentages can round to the same amount and the lower one
  shows pressed; the last change shown stays until the next cash is keyed; tapping the
  method already chosen clears what was keyed (as before).
- Audit (2026-10-03): `FormModal` was typechecked and tested, not tried in a
  browser; look at one save, one refusal and Enter in a two-field modal. `mez
  setup` pairs a till by default, so a venue set up with defaults reaches the
  back office with `mez pair -device-kind back_office` (ADR-144); phase 12's
  wizard should pair it. Setup's closing line and expiry now follow the
  device kind and the venue's zone.

- Redesign (ADR-128): screens are restyled through the primitives only.
  The specs' layouts were not re-read against the mez screen files,
  only their vocabulary and rules. Not seen by eye: card and
  UPI tender, split, the invoice panel, a menu card with a photo, the
  discount, note, approval and tickets sheets, a past bill and its
  refund sheet, the print queue, the reconciliation after a count,
  closing stock, floor states other than free and ordered, move and
  join, the kitchen's waiting tone, order and line notes, a voided
  line and Recall, a toast. In the back office: menu
  choices, recipes, roles, Z-reports, tips, audit, printers, and the
  sales screen after the charts turned green. Charts are one colour;
  the design's green, terracotta, mustard order waits for a chart with
  more than one series. The
  drawer count's face values sit right-aligned in their column, as
  before the redesign. NumPad keys use `sunken`, a stand-in on night that matches
  the canvas: no night screen has a keypad yet. Menu table cells wrap at
  1366px (Task 7).

- Devices (ADR-140): revoking a device does not close its open live stream; it
  is refused at its next connect, and every request it makes is refused at once.
  A revoked till's name in the hub is the one it connected with. Rename and
  revoke carry no `Idempotency-Key`. A device is "last seen" at most once a
  minute. The pairing code is shown once and cannot be shown again from the
  list (only its hash is kept).
- Stations and courses view (ADR-139): it cannot show whether a station has a
  printer (that is `settings.edit` data), so a retire says the printer follows
  and the toast says whether it did. A kitchen display set to a retired station
  still shows its board and is set to another by hand (ADR-134). Ordering is by
  arrows, not drag.
- Shift, tips and receipt screen (ADR-138): the receipt preview is of a made-up
  bill and shows no credit note; it was checked against the layout, not a
  printer. A tip weight for a renamed or removed role is not shown and drops at
  the next save. The preview asks the server on every pause in typing.
- Tax and numbering screen (ADR-137): the live preview of the next numbers
  needs a second ask per pause in typing; the rounding step is saved but nothing
  applies it (ADR-132). The profile card's "leaves N items without an HSN" shows
  only after a save, not as a draft. A tax class's retire refusal names eight
  items. The shell's navigation still drops a dirty draft.
- Venue settings screen (ADR-136): the "Leave without saving?" prompt is not
  seen by eye (only a second section can raise it, from the rail), and the
  shell's navigation drops a draft without asking. Timezone suggestions come
  from the browser's list.
- Receipt template (ADR-135): no screen yet (6i). The till's on-screen invoice
  preview does not show the header and footer, and no logo (ADR-079). A
  duplicate of an old receipt prints with today's template. Not tried on a
  printer: tested through the layout decoder at both paper widths.
- Stations and courses (ADR-134): no screen yet (6j). A retired station's
  kitchen display keeps its board and must be set to another by hand. Retiring
  a course leaves a gap in the numbers until the next ordering. Server and
  item form only, not tried in a browser.
- Rules (ADR-133): no screen yet (6i). A renamed role orphans its tip
  weight, and nothing checks. A day start change re-buckets past days in
  the reports, audit trail and CSV (events are placed by time); only
  shifts keep the day they opened under. Not tried in a browser: server
  only, Go tests and the HTTP round trip.
- Numbering (ADR-132): the cash rounding step is saved but no sale or
  payment applies it yet (a payment path owes that). The financial year
  start cannot move once a number is issued. A tax profile change does
  not start a new invoice series; one series spans it (ADR-131).
- Audit (ADR-117): discounts and refunds recorded before Task 4c show
  "Not recorded" for who authorised them. The Person filter lists active
  staff only (it reads `GET /api/v1/staff`); a deactivated person's rows
  show by name but cannot be filtered on. The audit screen has no CSV
  button yet (the route exists, ADR-119). The analysis (ADR-122) sums
  what each correction was worth to its bill, not money out of the drawer.
- CSV (ADR-119): each sheet reads the range again. The Tally files skip
  the formula guard and the byte order mark the report files have.
- Sales screen (ADR-120, ADR-121): tried with one day of sales and no
  credit notes, refunds or discounts, so the heatmap was seen with one
  cell only (its layout over many hours is tested in vitest, not by eye).
  A CSV is in the server's order, not the table's sort.
- Corrections (ADR-123): the browser run had one person, so a row for a
  guest, "not recorded" or someone removed was not seen by eye. Back from
  the trail returns `/sales` to seven days. A deactivated person's link
  shows "Someone no longer active" in the Person box (not seen by eye).
  A sales download and a corrections download can run at once. The range is not in the URL, so a reload returns to
  seven days. A CSV has not been opened in Excel itself. Left open past
  04:00, the range stays yesterday's and no quick range shows pressed.
- Reports (ADR-118): the screen lacks 5d to 5f. A range projects every order
  it touches from its first event; a year at a busy venue may take seconds.
  Items and categories are named as the menu stands now. The day start is
  the venue's setting as it stands when asked (ADR-133).
- Stock cost (ADR-124): movements from before migration 00028 have no
  cost. Nothing shows a cost's history; a new cost is for stock moved
  from then on.
- Closing stock (ADR-125, ADR-126): a shift with no closing count is not
  listed. Items are named as they are now. The browser run had one close
  on one day; several shifts and a partly costed item were tested in Go
  and vitest, not by eye. No subtotal per shift on screen.
- Stock (ADR-111 to ADR-114): The
  closing count cannot be skipped while items are marked. A close uses today's
  recipes, not the ones when the line was rung up. A sold movement needs a
  real staff id: the guest channel (phase 11) must supply one. A movement
  entered by hand after the closing drawer count records no shift (the
  closing count names its own, ADR-114).
- Photos (ADR-110): `<data dir>/photos/` is not backed up by Litestream
  (phase 12 must add it); unreferenced photo files are never swept; the
  guest app does not show photos yet.
- Sold out (ADR-109): only the back office can mark it, not the till; the
  server does not refuse a sold-out item rung up (AddLine is unchecked);
  a till with a wrong clock ends it at the wrong time.
- Menu editor (ADR-106, ADR-107): no `Idempotency-Key` on
  its routes, as with printers. Duplicate item names are allowed.
  Stations, courses and tax classes have a write path (ADR-134). An item's
  groups save in a second request after the item (ADR-108). A negative
  choice price is not checked against the items offering it. A venue
  whose tax profile changes to one needing
  HSN keeps items without one until each is edited; the tax settings list
  them (ADR-131). The table scrolls sideways below about 1100px of content width.
  Retiring an item shows no "open tickets keep it" note. Form controls
  use the shared `inputClass`, which does not set the 16px control size
  tokens.md asks for (the printers form has the same gap).

- Shell (ADR-105): tried as owner and manager (Task 4b); a cashier sees
  the dashboard alone (vitest). Printers, floor and Z-reports still carry
  their own "Back office" link above the title, now redundant with the
  navigation. Permissions changed mid-session reach the shell at the next
  sign-in or reload.

- Carried from phase 04, owned by no task yet:
  - The done-when script against the real binary. The same loop now runs
    in process on the real stack (ADR-092); the binary itself is left for
    phase 12's installer smoke test.
  - A `float64` lint rule for `internal/domain` (`docs/02-domain.md`).
  - A guard that fails the build when `order.State` changes without a bump
    to `order.ProjectionVersion`; today a forgotten bump serves stale
    snapshots.
- Exports (ADR-103, ADR-104): not yet imported into a real TallyPrime;
  its ledgers are mapped by name once in Tally. Shifts closed before
  migration 00020 have no files. A shift whose files cannot be written
  does not close, as with its Z-report.
- Z-report (ADR-097 to ADR-100): categories are the menu's when the
  report is built, not when the item was sold. The printout has no
  by-item lines; the screen does. The till shows only the report its own
  close issued, until reload; the back office list was typechecked and
  API-tested, not tried in a browser. No screen issues a correction yet
  (the API does). A report that cannot be built blocks the close; there is no
  way round that short of fixing the data.
- Tips (ADR-101, ADR-102): hours are typed in at pooling (no timesheets
  until phase 10); a pool is once per shift with no correction; the rule
  and weights are set through the rules route (ADR-133), with no screen yet (6i). The browser run had
  one staff member, so a multi-person or by-hours pool was tested in Go
  and vitest, not by hand; staff can now be added (Task 4a).
- Shifts (ADR-093 to ADR-096): terminals learn of a shift opening,
  counted or closed by polling every 30 seconds, not the stream. Opening
  needs shift.open (ADR-152) and closing shift.manage: a cashier who cannot
  close is told to ask a manager, with no override sheet. No till screen for cash in or out yet (the API has
  it). The day start is a venue setting (ADR-133). A payout larger than the drawer is
  not refused. Closing leaves open tickets to carry into the next shift.
  A counted drawer cannot be recounted. The variance threshold is set
  through the rules route (ADR-133), with no screen yet (6i). Denomination
  tables exist for INR, USD, EUR and GBP only; others count one total.
- `AddLine` is not checked against the menu (price, tax, modifier min/max);
  the menu is now written over HTTP (ADR-106), so this matters more. GST place of supply is always
  the venue's state, so no IGST. `mez setup` cannot finish a US venue.
- `orders.table_id` has no foreign key (ADR-085). Guests left closes and
  clears in two steps, not one transaction; a retry repairs it (ADR-087).
- Nothing stops seating a dirty table over the API; the floor only offers
  Seat on a free one. The plan and the list share `/tables`: a reload returns to the plan.
- A table with two tickets cannot be moved from the floor; the timeline
  names tables by number (ADR-088).
- Never swept: idempotency keys, expired staff sessions, sign-in attempts.
  The pairing rate limiter is per process.
- A bill printed before payment has no number: numbers come with the
  settling payment (ADR-046, ADR-078). Task 4's pre-payment bill layout
  must say it is not a tax invoice.
- A split lives only on the tender screen (ADR-077); by guest picks seats
  there until lines carry them (phase 08).
- The server does not cap a tender's `reference`; the till caps it at 40.
- Printing is ASCII only (ADR-079): non-Latin item names print as "?".
  No physical printer has run the driver yet.
- The Windows spooler's write path is untested on a real printer (ADR-080).
- The cash drawer opens only with the settling receipt, not on a cash part
  payment (ADR-081).
- The ticket rail marks a line Ready but not sent or held by course. No
  `collected` step (ADR-090). A display's station is per device; a void
  on a sent line reaches it within 20 seconds, not at once (ADR-091).
- The till refunds to one payment per credit note (ADR-084); past bills
  search only exact numbers and invoice sequences.
- `make build` does not build under cmd or PowerShell; for the browser,
  `CGO_ENABLED=0 go build -tags frontend -o bin/mez ./cmd/mez`.
- A data dir path containing `%` cannot be opened: `internal/db/open.go`
  does not escape the path in its `file:` DSN.
- To try the apps: `make dev` (ADR-144) prints a pairing code for each; pair
  in the browser, sign in as the owner or a demo person and count a float to
  open the till. A code for the wrong app is refused but its device stays
  (ADR-068).
- The tables screen (ADR-155): not seen by eye are clearing a table, Guests left, join
  and part, a table with two orders, the paying and to-clear states, a held order, the
  Notes tab with notes, a refusal, and the Terrace.
  A floor of seven or more columns scrolls sideways at 1366px.
  After the review: Seat waits for the floor, the order is read again each time its
  table is picked, and Send waits while this till still has items going to that order;
  those three and the tabs' arrow keys were typechecked, not tried in a browser. A move
  or join keeps the party's table as it was when the mode began. A tile's spoken name
  leaves out what it owes.
- The item modal (ADR-153, ADR-154): a note refused after its line was added is lost
  with a toast. A tap meant for a card's "+" that lands on the card opens the modal. Not seen by eye: a photo, the Popular pill, a refusal, a screen under 1024px.
- The order screen (ADR-151): not seen by eye are a photo card, a sold-out card, a
  held order's banner, a delivery order, a discounted order's totals and a line the
  kitchen has seen (its bin and plain quantity). The diet filters reset on a reload.
- The open-shift screen's sold-out card was not seen by eye, and its float panel and
  buttons sit below the fold at 768px high (ADR-150).
- `go test -race` needs cgo, which is banned; `make check` runs without it.
- The domain import test checks direct imports only (ADR-016).
- Tooling: `make` needs `sh` (Git Bash; GnuWin32 make 3.81 works, at
  `C:/Program Files (x86)/GnuWin32/bin/make.exe`). `make check` does not
  rebuild the web bundles: run `make build-web` before trying the browser. `make
  generate` needs sqlc v1.31.1 and can dirty `go.sum` with goose's test
  modules: restore it with `git checkout go.sum`. `bin/mez` has no
  `.exe`, so kill a stray server with `taskkill /F /IM mez`.

## Open questions for the human

- A data directory from before the rename: move it to the new default
  and rename `nofiltr.db` to `mez.db`. Paired browsers pair again.

---

## Phase progress

- [x] 00 — Repo skeleton, docs bootstrap, design tokens
- [x] 01 — Go server skeleton, config, logging, embed, cross-compile
- [x] 02 — SQLite, migrations, sqlc, money type, IDs
- [x] 03 — Domain core: order events, projections, invoice series, tax
- [x] 04 — HTTP API, WebSocket hub, TypeScript type codegen
- [x] 05 — Frontend shell: Vite, Tailwind tokens, router, primitives, auth
- [x] 06 — Terminal: register, modifiers, cart, open tickets
- [x] 07 — Payment, split, invoice, print queue, cash drawer
- [x] 08 — Floor plan, tables, kitchen display, station routing
- [ ] 09 — Shift, blind drawer count, Z-report, tips
- [ ] 10 — Back office: menu editor, inventory, reports
- [ ] 11 — Relay service, outbound tunnel, guest QR ordering
- [ ] 12 — Installer, Windows Service, Docker, Litestream, docs
