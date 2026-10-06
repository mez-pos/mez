# History

> What each task did, newest first. Moved out of `STATE.md` on 2026-10-05 (rebuild
> Task 17h3) so that file stays short. **Not read at the start of a session.** Read
> an entry here only when a task needs to know what an earlier one did; the
> reasons are in `DECISIONS.md` under the ADR each entry names.
>
> When a task is done, its entry in `STATE.md` under "Last completed task" replaces
> the one before it, and the one before it is put at the top of this file.

**Phase 11, Task 4f3: paying online from the phone** (ADR-222).
`POST /order/api/t/{code}/bill/charge` and `GET .../bill/charge/{id}`; the bill
says `online`. The return address is `public_url`, a new setting, never a
request's. The phone has Pay online (`charge-logic.ts`, `use-charge.ts`).

Before it, **Phase 11, Task 4f2: Razorpay, its keys, and the sweeper** (ADR-221).
`internal/gateway/razorpay` uses payment links. `gateway`, `gateway_key_id` and
`gateway_key_secret` are configuration (`MEZ_GATEWAY_KEY_SECRET`), not settings.
`charges.Sweeper` settles open charges every 20 seconds.

Before it, **Phase 11, Task 4f1: the gateway interface, and a charge becoming one payment**
(ADR-220). `internal/gateway` is two questions, Start and Status.
`internal/service/charges` starts a charge for what a table owes and, when the
gateway says paid, tenders it once (`charge:<id>` as the key); money a bill cannot
take is kept as `unapplied`. Tested against a fake.

Before it, **Phase 11, Task 4e4b: "I have paid" on the screens** (ADR-219). The phone's bill
has the button, and says staff were told from the bill's `paid_told`. The
terminal's pill, sheet ("From the tables") and chime carry the notice, with Take
payment opening `/pay/{order}` and No payment found putting it away.

Before it, **Phase 11, Task 4e4a: "I have paid", on the server** (ADR-218).
`POST /order/api/t/{code}/bill/paid` keeps a notice (`paid_notices`, migration
00040): a message to staff, never a payment. The bill says `paid_told`. Notices
ride on `GET /api/v1/guest-requests` as `paid`; staff put one away with
`POST /api/v1/paid-notices/{id}/seen`, and a recorded payment puts it away itself.

Before it, **Phase 11, Task 4e3: the QR on the phone's bill** (ADR-217).
`GET /order/api/t/{code}/bill/upi.svg` draws the bill's UPI link as a QR code, by
an encoder of our own (`internal/domain/qr`, no dependency), checked module for
module against python-qrcode and read back by a decoder. The phone shows it under
"Pay from another phone". No phone has scanned one.

Before it, **Phase 11, Task 4e2: the UPI address in the back office** (ADR-216). A card,
"Paid from a phone", under the venue's details on Settings: the address, the name
a guest sees, a standing warning that mez cannot check the address, and what a
guest's bill now offers. `web/backoffice/src/settings/PaymentsCard.tsx`.

Before it, **Phase 11, Task 4e1: a UPI pay link on the phone's bill** (ADR-215). The venue's
UPI address is a setting (`GET`/`PUT /api/v1/settings/payments`). The server builds
the `upi://pay` link for the balance (`payment.UPIAccount.Link`) and the bill
carries it; the phone shows "Pay by UPI". Staff confirm the payment on the
terminal, as at the till. 4e split in five.

Before it, **Phase 11, Task 4d: the bill on the phone** (ADR-214).
`GET /order/api/t/{code}/bill` is the table's open ticket with the till's parts
taken out; `#bill` shows its lines, tax, total, paid and left to pay.
`web/guest/src/Bill.tsx`, `bill-logic.ts`; spec `guest-bill.md`. The prepay setting
moved to 4e.

Before it, **Phase 11, Task 4c: Place Order on the phone** (ADR-213). The cart sends itself
with one key for one cart; the answer is kept by table code and shown at `#order`
as waiting, with the kitchen, or not accepted; a waiting order asks again every 8
seconds. `web/guest/src/order-logic.ts`, `use-order.ts`, `Order.tsx`; spec
`guest-order.md`.

Before it, **Phase 11, Task 4b2: the request on the terminal** (ADR-212). A pill in the top
bar on every screen while a request waits, a sheet with Accept and send and
Decline, the kitchen's chime on arrival and every 30 seconds, and a mark on the
table. `web/terminal/src/requests/`, spec `terminal-requests.md`. Task 4b done.

Before it, **Phase 11, Task 4b1: answering a guest's request, on the server** (ADR-211).
`GET /api/v1/guest-requests`, and `POST .../{id}/accept` and `.../decline`, asking
`order.open`. Accept finds or opens the table's ticket and appends the lines as
stored. A request arriving or answered moves the floor version, so terminals hear
`floor_changed`. A request lasts as long as its shift.

Before it, **Phase 11, Task 4a: a guest's cart becomes an order, on the server** (ADR-210).
`POST /order/api/t/{code}/orders` with ids, counts and a required `Idempotency-Key`;
`menu.Menu.Ring` prices each line. A "round" (`guest_rounds`, migration 00039,
`svcorder.PlaceRound`) joins the table's open ticket and is sent, or with none open
is kept as a request: no order and no event. Task 4 split in seven.

Before it, **Phase 11, Task 3c: a dish with its choices, and the cart** (ADR-208). The sheet
uses the till's own rules (`web/shared/modifier-selection.ts`, moved). The cart is ids
and counts in `localStorage` by table code, priced from the menu each time. Nothing is
sent. Then photo frames once any dish has a photo, and the decision that a phone order
is a request staff accept (ADR-209). 36.1 KB of 60.

Before it, **Phase 11, Task 3b: the guest's landing and menu** (ADR-207), `web/guest/src`: the
table and venue, then every dish with a search, categories and diet filters, and a
screen for a code that leads nowhere or a venue that cannot be reached. No router:
the code is read from the end of the path and the screen is the hash. Specs:
`guest-landing.md`, `guest-menu.md`.

Before it, **Phase 11, Task 3a: the guest's way in** (ADR-206). Each table has a code
(`internal/service/tablecodes`, migration 00038, `mez table-codes`). The guest API is
under `/order/api/` (`t/{code}`, `t/{code}/menu`, `photos/{name}`), with no device or
session, 300 requests a minute an address. The tunnel leaves `/order/...` paths alone,
so the bundle's files load through the relay; proved at `democafe.jskin.in`.

Before it, **Phase 11, Task 2b: the tunnel in the back office** (ADR-205). `GET /api/v1/tunnel`
and a card on Devices, "Ordering from phones": a chip, a sentence saying what guests
can do, when it tries next and why it failed. Health reads the same status
(`api.WithTunnel`). Seen in the browser reconnecting and connected, at 1366 and 400
wide. Task 2 done.

Before it, **Phase 11, Task 1c: the relay is live** (ADR-204) at `https://relay.jskin.in`, venues
at `<venue>.jskin.in`: the owner's Hetzner server (wiped, renamed `mez`) behind
Cloudflare, Caddy for TLS with one wildcard certificate, the files in `build/relay/`.
Proved with `bin/mez` on the owner's PC and a guest page fetched over the internet.
`jskin.in` is for testing; the owner will buy `mez.ac` for production.

Before it, **Phase 11, Task 1b: a hostname per venue** (ADR-203). `RELAY_VENUE_DOMAIN` serves
each venue at `<venue>.<domain>`, the whole path its own, and stops serving
`/v/<venue>/`. Venues dial `relay.<domain>`. Reserved names are never venues. Unset,
the relay routes by path as before.

Before it, **Phase 11, Task 2a: the tunnel client** (ADR-202). `internal/tunnel` dials the relay
with the new `relay_key`, serves guests through the server's own handler, reconnects
with backoff (1 s to 60 s, 5 minutes after a refused key or a replaced socket) and a
heartbeat, and is `checks.tunnel` in health. Every guest path lands under `/order/`,
out of reach of the staff API. `TestTheCounterSellsWhileTheRelayIsDown` kills a real
relay mid-service; also done by hand with both binaries. Task 2 was split: 2b is the
back office view.

Before it, **Phase 11, Task 1: the relay service** (ADR-201). `cmd/relay` over `internal/relay`:
a venue dials `GET /agent` with a signed key (`relay issue-key`, an HMAC under
`RELAY_SECRET`, nothing stored), guests reach it at `/v/<venue>/...`, and requests
ride one socket as frames (`internal/relay/wire`). Per-venue token bucket, 256
streams, 1 MiB bodies, a slow guest cut before the socket goes unread, JSON logs in
UTC, `/healthz`, `build/relay.Dockerfile`, and `make build` makes `bin/relay`. Venues
share one origin: cookies and redirects are kept to the prefix, but that is not
isolation (open question 7). Tests over real sockets, 92% of the package.

Before it, **Rebuild, Task 17h3: STATE.md after the walk** (ADR-200). Task 17 is done: 28 screen
specs written from the code (`docs/design/screens/README.md`), the old words off the
screens, one door for closing the day, and a walk as each role (ADR-196 to ADR-199).
`STATE.md` is short again: each task's history moved here and the known-broken list to
`docs/KNOWN.md`, whole. There, "not seen by eye" lost only what the walk saw, in
fifteen places, and three statements no longer true. The owner started phase 11
before the walk; the rebuild and phases 09 and 10 still owe their human checks.

Before it, **Rebuild, Task 17h2: the walk** (ADR-199). On a fresh demo venue run from the current
build (the embedded bundles), at 1920x1080 and for the waiter 1366x1024: Meera opened
the shift and took a counter order through cash, a tip and its invoice; the wall
display started and readied its tickets; Kabir seated T3, ordered over two courses,
sent, fired, split the bill by UPI and cash, closed and cleared the table; Arjun
refunded an item, counted the drawer blind, explained ₹5.00 short, closed, and read
Z-report 1; Ravi landed on `/pass`; the back office showed the day on the dashboard,
the Z-report with its exports, the reports and Audit. Every figure agreed, the drawer
to the rupee. Two things it found are fixed: a refund now says who authorised it (Go
test), and the Z-report's Tax card says it is after credit notes (`taxNote`, vitest).
17h1's fixes were seen: no Close the day on the Orders sheet, the new words. `make
check`, vitest (1426) and `make build-web` pass.

Before it, **Rebuild, Task 17h1: the fixes before the walk** (ADR-197, ADR-198). Task 17h is split
in three. What writing the specs found is fixed: the Orders sheet's Close the day is gone,
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
