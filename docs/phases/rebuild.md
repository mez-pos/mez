# Rebuild - the screens follow the mez design

## Goal

Every staff screen is laid out and flows as the mez design files show, not
only coloured like them. A person signs in and lands on their own work:
a cashier on the order screen, a waiter on their tables, the kitchen on its
board, a manager with the whole navigation. ADR-143.

This runs before phase 11. The server, the domain and the `*-logic.ts` files
stay; the screens over them are rebuilt.

## Why this phase exists

The redesign (ADR-128, `redesign.md`) changed tokens and primitives and kept
the old Nofiltr layouts. The owner ran the result on 2026-10-03 and rejected
it: no flow, kitchen jargon on the buttons, no way to tell a cashier's screen
from a waiter's. The design files were never rebuilt from, only borrowed from.

## The design source

`docs/design/reference/mez/` only. The PNG mockups under
`claude-design/design/uploads/` are not a reference: ignore them.

Each screen is a block carrying `data-screen-label`. Open one file, jump to
one line, read to the next label. Do not browse a whole file.

| File | Screen | Line |
|---|---|---|
| `mez Desktop POS.dc.html` (1920x1080) | D01 Login | 36 |
| | D02 Shift Open | 106 |
| | D03 POS Order | 175 |
| | D04 Item Modal | 293 |
| | D05 Floor Plan | 363 |
| | D06 Reservations (table list part only) | 478 |
| | D07 Payment | 574 |
| | D08a Split Bill | 700 |
| | D08b UPI Card States | 751 |
| | D09 Orders Refund | 790 |
| `mez Tablet Waiter.dc.html` (1366x1024) | T01 Waiter Home | 36 |
| | T02 Order Taking | 101 |
| | T03 Table Detail | 247 |
| | T04 KOT Preview | 377 |
| | T05 Bill Split Payment | 425 |
| | T06 Transfer Merge | 489 |
| | T07 Requests Inbox | 525 |
| | T08 Shift Summary | 552 |
| `mez Desktop KDS and Dashboard.dc.html` | D10 KDS Parchment | 36 |
| | D11 KDS Night | 158 |
| | D12 Dashboard | 280 |
| | D13 Notifications | 429 |
| | D14 Offline Sync | 462 |
| `mez Desktop Management.dc.html` | D15 Sales Reports | 36 |
| | D16 Item Inventory Reports | 176 |
| | D17 Staff Customer Reports | 224 |
| | D18 Menu Management | 286 |
| | D19 Inventory | 428 |
| | D20 Staff Roles | 507 |
| | D21 Customers Offers | 557 |
| | D22 Settings | 602 |
| | D23 Z Report | 707 |
| | D24 Setup Wizard | 770 |
| `mez Design System.dc.html` | The component sheet, no screens | - |

`mez Mobile Customer.dc.html` (M01 to M11) is phase 11's, not this phase's.

## The flow map

One sign-in screen for everyone (D01): tap your face, enter your PIN. Your
role is under your name. Where you land depends on the role, not the device.

```
                         D01 Login (profile + PIN)
                                   |
        +-------------+------------+-------------+----------------+
        |             |                          |                |
     Cashier        Waiter                    Kitchen       Manager / Owner
        |             |                          |                |
  shift open?    T01 My tables              D10/D11 board    same as cashier,
   no -> D02          |                                      plus Menu and
   yes |         tap a table                                 Reports in the
        v             v                                      top navigation
   D03 Order     T03 Table detail                            (back office)
   (pick table,       |
    Dine-in /    T02 Order taking -> Send to Kitchen (T04 KOT preview)
    Takeaway /        |
    Delivery)    T05 Bill, split, payment
        |
   D04 Item modal (choices, note, quantity)
        |
   Save Order  or  Pay
        |
   D07 Payment -> D08a Split -> D08b UPI / card -> Complete -> New Order
```

Top navigation on the desktop, from D05: **Orders, Tables, Menu, Reports**.

- Orders: the order screen (D03), and order history with refunds (D09).
- Tables: the floor (D05) and the table list.
- Menu, Reports: manager and owner only; they open the back office.
- The profile menu (top right) holds Lock, Close the shift, Print queue.

A 40px status bar on every desktop screen: version, Online or Offline, the
venue, Keyboard Shortcuts, Open Cash Drawer, date and time.

## Words

The design's words replace the old ones on every screen:

| Old | New |
|---|---|
| Till, Register | Order screen; "the shift" for open and close |
| Ticket | Order |
| Charge | Pay |
| Sent, Fire | Send to Kitchen (fire by course lives under Course View) |
| Hold | Save Order |
| Tickets (sheet) | Orders |
| "The till is closed" | Open the shift (D02) |

## Rules for every screen task

1. Read `docs/design/tokens.md`, then only the named line range of the one
   design file. The old `docs/design/screens/*.md` describe the old layouts:
   where they disagree with the mez file, the mez file wins.
2. Rebuild the layout as drawn: regions, order, sizes, icons, labels. Keep
   the hooks and `*-logic.ts` files and their tests; replace the JSX.
3. A control the design shows and the server cannot do is either built in
   Task 1 or left out. Never draw a control that does nothing.
4. A function the app has and the design does not show (hold and resume,
   fire by course, history, approvals, print queue, past bills) is kept and
   placed under the screen's "..." or "More Actions" menu. Note where.
5. Item cards must look finished with no photo: most venues will not have
   photos on day one.
6. The design is drawn at 1920x1080 (desktop) and 1366x1024 (tablet). The
   desktop screens must hold together down to 1366 wide.
7. Text floors from ADR-014 stand: 11px metadata, 13px sentences, 15px
   interactive labels, 16px controls, 48px touch targets.
8. Done means: a screenshot of the real screen beside the design's screen,
   both in the session's reply, and the owner has looked at it in the dev
   server. List anything left out and why.

## How to see it

```sh
make dev
```

It builds the binary, makes a demo venue in `.dev-data` the first time and
prints a pairing code for the terminal, the back office and the kitchen
display, then starts the server on `127.0.0.1:7791` and the two apps with hot
reload: the terminal on http://localhost:5173 (the kitchen at `/kitchen`) and
the back office on http://localhost:5174/backoffice/. Ctrl+C stops all three.

Sign in as Priya (PIN 1234), or as the demo staff: Arjun the manager (2222),
Meera the cashier (3333), Kabir the waiter (5555), Ravi in the kitchen (4444). Another code, for a
browser that is not paired yet: `bin/mez pair -data-dir .dev-data -device-kind
terminal`. A venue made before Task 0 has no floor or staff: delete
`.dev-data` and run `make dev` again.

A pairing code lasts 10 minutes. A Go change needs `make dev` run again; a
screen change does not.

## Tasks

One task per session. Tick it, update `STATE.md`, commit `rebuild/task-N`.

- [x] **Task 0 - The dev loop and the demo venue.** (ADR-144)
  `mez pair -device-kind K -device-name N`: a pairing code made at the
  machine itself, as setup's is (closes the "no screen to reach the back
  office from" gap in `STATE.md`). `-demo-menu` becomes a fuller demo: a
  floor of about twelve tables in two areas, and one staff member of each
  role with known PINs printed by setup. `make dev` starts the three
  processes above. Go tests for both.

- [x] **Task 1 - What the design needs from the server.** Split into
  sub-tasks before starting; each is domain code with tests.
  - [x] 1a (ADR-145). A `waiter` role (take orders, send, see own tables; no refunds, no
    shift close). `GET /api/v1/staff` returns each person's role name, so
    the login can show it.
  - [x] 1b (ADR-146). Service type on an order: dine-in, takeaway, delivery. A new field
    on the opening event, not a change to old events; old orders read as
    dine-in when they have a table and takeaway when not. Guest count on a
    seated order if the events do not carry it yet. Shown on the Z-report
    only if that is free; otherwise note it.
  - [x] 1c (ADR-147). Item fields the cards show: a short description, a dietary mark
    (veg, non-veg, vegan, gluten free), a "popular" flag. Menu editor form
    fields for them, plain, until Task 16 rebuilds that screen.
  Check first what exists: `internal/domain/order/payloads.go` has a
  `Channel`, which is where the order came from, not how it is served.

- [x] **Task 2 - The shell.** (ADR-148) Routes in the terminal app instead of state
  flags in `RegisterScreen.tsx`: `/order`, `/tables`, `/orders`, `/pay/:id`,
  `/shift`, `/waiter`, `/kitchen`. The 72px top navigation and 40px status
  bar from D05. Landing by role. Icons: the design uses lucide; add the
  icons needed to `web/shared/ui/icons.tsx` or take `lucide-react` in the
  terminal and back office only (never the guest bundle), and say which in
  an ADR. Decide and record how a manager on a terminal reaches Menu and
  Reports (one pairing should be enough for one browser).

- [x] **Task 3 - Login.** (ADR-149) D01. Left panel with the brand, right with profile
  tiles (avatar initials, name, role), PIN dots, keypad, Login.
- [x] **Task 4 - Open the shift.** (ADR-150) D02. A step in the cashier's flow with
  "Tonight's shift" beside it, not a wall. A waiter who signs in with no
  shift open is told who can open it.
- [x] **Task 5 - The order screen.** (ADR-151) D03. 240px category sidebar with icons,
  filter chips, item cards with "+", the 400px order rail: table picker,
  service type, guests, lines with steppers and delete, order note, totals,
  Save Order, Pay.
- [x] **Task 6 - The item modal.** (ADR-153) D04. Choices, add-ons, note, course,
  quantity, "Add to Order" with the price.
  - [x] 6b (ADR-154). The card opens the modal when there is anything to choose;
    its "+" adds with the defaults.
- [x] **Task 7 - Tables.** (ADR-155) D05, and the table list from D06. Areas, the
  five states, the selected table's panel: Add Items, Send to Kitchen, More
  Actions, Settle Bill. Reservations and the waitlist are out of scope.
- [x] **Task 8 - Payment.** (ADR-156) D07. Three steps (Payment, Split Bill,
  Complete), method tiles, amount received with quick amounts, change, tip,
  receipt choices, the success panel with New Order.
- [x] **Task 9 - Split and UPI or card states.** (ADR-159) D08a, D08b.
- [x] **Task 10 - Orders and refund.** D09. Past bills and the print queue
  move here. Split on 2026-10-05: the design lists the day's orders, open and
  paid together, with their type, items and status, and the server had no
  such list (ADR-160).
  - [x] 10a. The day's orders from the server: `GET /api/v1/orders/today`,
    every order of the trading day and any still open, each with its status
    (open, paid, refunded, voided), type, table, items and totals, and what
    they collected. Go tests.
  - [x] 10b. (ADR-161) The Orders screen: the title and counts, the status tabs, the
    search, the table of orders and the selected order's panel (people,
    payment, items, totals, Reprint, Refund, Void). `/orders/:id` and the
    past bills sheet fold into it.
  - [x] 10c. (ADR-162) The refund as D09's modal (selected items or the whole bill,
    the reason, the manager's PIN), and the print queue under Orders.
- [x] **Task 11 - Waiter home.** (ADR-163) T01.
- [x] **Task 12 - Waiter order taking.** T02, T04 (KOT preview). Split on
  2026-10-05: T02 is the order screen (D03) rearranged, and the preview and the
  course view are each a piece of their own.
  - [x] 12a. (ADR-165) The waiter's order screen (T02): the top bar carries the way back, the
    table, the guests and how it is served; the search sits over the categories;
    the rail has the table with its chips, each line's course, Clear, the totals,
    Save and Send to Kitchen. Pay moves under the rail's "...".
  - [x] 12b. (ADR-166) Confirm and send (T04): what will go to the kitchen and what has gone
    already, the ticket as the kitchen will read it, Back and "Send N items".
  - [x] 12c. (ADR-167) Course View: the rail's second tab, the order by course, with what has
    gone, what is held, and firing the next one.
- [x] **Task 13 - Waiter table detail, transfer and merge.** T03, T06. Split on
  2026-10-05, a screen each.
  - [x] 13a. (ADR-169) The table's own screen (T03): the table and how long it has sat, its
    notes, its items by course, where the order stands with the kitchen, the
    totals, Add Item, the bill, and Send to Kitchen. A waiter's card leads here.
  - [x] 13b. (ADR-170) Transfer and merge (T06): a modal from the table's screen that moves
    the party to a free table or joins free tables to it.
- [x] **Task 14 - Waiter bill and shift summary.** T05, T08. Split on
  2026-10-05, a screen each.
  - [x] 14a. (ADR-172) The waiter's bill (T05): a waiter's "Bill and pay" opens the bill
    under the waiter's bar, not the desktop payment screen. The table and what
    it owes, split equally, by item or by amount, each guest's share with
    Collect, how the chosen share is paid, what has been collected so far,
    Print bill and Close table. Over `tender-logic.ts` and `split-logic.ts`.
  - [x] 14b. (ADR-173) What one person did in the shift, from the server (split again on
    2026-10-05: the server could say nothing per person). `GET /api/v1/shift/mine`:
    the orders they opened, how many were at a table, what their payments
    settled, the tips on them, and those tips by table. Go tests.
  - [x] 14c. (ADR-174) The shift summary (T08) over it: the waiter, how long they have
    been on, the four figures, tips by table, Print summary if anything can
    print it, and End shift. Guest ratings are left out (phase 11).
- [x] **Task 15 - Kitchen display.** D10 and D11. Split on 2026-10-05: the
  design is a board of three columns (New Orders, Preparing, Ready for pickup)
  with the day's figures beside it, and the server knew a ticket only as sent
  or bumped.
  - [x] 15a. (ADR-175) "Start preparing", from the server: a ticket a cook has picked up
    is in progress (`ticket.started`), and is marked ready from there. `POST
    .../tickets/{seq}/start`; the board carries it. Go tests.
  - [x] 15b. (ADR-176) The board as drawn (D10), on paper: the header, the three columns,
    each card with where it goes, when it was placed, how long it has waited,
    its lines and its one action. Recall and Change station kept (rule 4).
  - [x] 15c. (ADR-177) Espresso night (D11) and the foot: Night mode kept on the device,
    Sound, and whichever of Auto advance, Group by table and Mark all complete
    the server can do.
  - [x] 15d. (ADR-178) Today's stats: what the server can say for a station
    today (tickets sent, in progress, completed, average and slowest time),
    on the board's response, then the column beside the board. Split on
    2026-10-05: the design's Upcoming is orders not yet sent, which nothing
    knows.
  - [x] 15e. (ADR-179) A cook who signs in on a terminal reaches the board (the flow
    map): today `/kitchen` asks that browser for a pairing of its own.
  - [x] 15f. (ADR-181) Coming next: what a station will be sent when a later course is
    fired (the lines waiting for their course, by order), under Today's stats
    where the design draws Upcoming. Check first what the order knows of a
    held course; if nothing useful, record that and leave it out.
- [ ] **Task 16 - Back office.** Split into one sub-task per screen before
  starting: D12 dashboard, D15 to D17 reports, D18 menu, D19 inventory,
  D20 staff and roles, D22 settings, D23 Z-report. D14's offline and sync
  states go with the shell's status bar if Task 2 did not cover them.
  Split on 2026-10-05, a screen each, with the frame every one of them sits
  in ahead of them. D21 is out of scope, so D17 is its staff half only.
  - [x] 16a. (ADR-182) The back office's frame (D12's bar and sidebar): the 72px bar
    with the logo, the date and time, the connection (D14's word) and who is
    signed in, whose menu holds Sign out; the 240px sidebar with an icon on
    every entry. The sections the design does not draw keep their places
    (rule 4). Search and the bell are left out: nothing answers them.
  - [x] 16b. (ADR-183) The dashboard (D12): today's sales, orders, active tables and
    average order beside yesterday's; sales through the day; top selling
    items; active tables; staff; low stock; quick actions. Each card only for
    whoever may read what is in it.
  - [x] 16c. Sales reports (D15). Split on 2026-10-05: the design cuts sales by how
    they were served and counts guests, and the report did neither.
    - [x] 16c1. (ADR-184) The report by service type, on the range and on each
      period, and the guests on its bills. Go tests.
    - [x] 16c2. (ADR-185) The screen: the range and Export beside the title, four
      figures against the days before, the trend, sales by category, top sellers,
      orders by type and payments; the old reports kept under "In detail".
  - [x] 16d. Item and inventory reports (D16). Split on 2026-10-05: the design
    draws wastage and what each item cost, and the server could say neither.
    - [x] 16d1. (ADR-186) What was written off over a range, by stock item and by
      reason, at the cost it had then. `GET /api/v1/reports/wastage`. Go tests.
    - [x] 16d2. (ADR-187) The screen, a tab beside Sales: item performance against
      the days before, wastage, stock health; what sold in full and the closing
      stock moved here from Sales reports.
    - [x] 16d3. (ADR-189) What an item cost to make and its margin. The owner
      chose the ledger over today's recipe: a sold movement keeps its dish, the
      ledger keeps how many of a dish it costed, and a margin is given only when
      it costed every unit sold. `GET /api/v1/reports/items`; the Cost and Margin
      columns.
  - [x] 16e. (ADR-188) Staff reports (D17, without the customers). Split on
    2026-10-05: the report had one tips total and the design a tips column.
    - [x] 16e1. Tips by who took the payment, on the report (`tips_by`). Go tests.
    - [x] 16e2. The screen, a third report tab: staff performance with tips,
      removed in error, the tips total; "Who sold it" moved here.
  - [ ] 16f. Menu (D18).
  - [ ] 16g. Inventory (D19).
  - [ ] 16h. Staff and roles (D20).
  - [ ] 16i. Settings (D22).
  - [ ] 16j. The Z-report (D23).
- [ ] **Task 17 - Close.** Rewrite `docs/design/screens/*.md` from what was
  built. Walk register, kitchen, payment, invoice and Z-report as each role.

## Not in this phase

- D06 reservations and waitlist, D21 customers and offers: out of v1 scope
  (`CLAUDE.md`, scope discipline).
- D13 notifications, T07 requests inbox: they need the guest channel
  (phase 11). The bell is not drawn until something rings it.
- D24 setup wizard: phase 12.
- M01 to M11, the guest phone: phase 11, built from the mez file then.

## Done when

The owner signs in as a cashier, a waiter, the kitchen and a manager in the
dev server, takes an order from each side through to a paid bill and a
Z-report, and says each screen matches its design. `make check` and
`make build-web` pass.
