# Kitchen: the board

The pass: every ticket a station has to make, in three columns it crosses from
left to right, read from three metres with wet hands. Built in phase 08
(ADR-090, ADR-091) and rebuilt from D10 and D11 in the rebuild's Task 15
(ADR-175 to ADR-181). `web/terminal/src/kitchen/`: `KitchenApp.tsx`,
`CookBoard.tsx`, `KitchenBoard.tsx`, `TicketCard.tsx`, `BoardFoot.tsx`,
`BoardStats.tsx`, `kitchen-logic.ts`, `board-prefs.ts`, `use-chime.ts`,
`use-kitchen-board.ts`.

## Purpose

Start the next ticket, mark it ready when it is on the pass, and see at a
glance which are late and how many of each dish are still to go.

## Where it lives

One board, two ways in:

- **The wall display, `/kitchen`.** Another app of the terminal's bundle. It
  pairs as a **kitchen** device and never asks for a PIN: a screen on a wall
  has nobody to type one. A code for a terminal is refused here, as anywhere
  (ADR-068). Unpaired, it shows Pair (`terminal-lock.md`).
- **A cook signed in on a terminal, `/pass`** (ADR-179). The kitchen role
  lands here. It is the same board with the person's own session, outside
  the terminal's frame and its shift gate, so the kitchen works before a
  shift is open. Their moves carry their name.

Both keep the station on the device (`mez.kitchen.station`).

## Layout

Full screen, on paper to start, with a header, the board and a foot. Only the
columns scroll, each on its own.

- **Which station is this?** (first run, or Change station): the venue's
  stations as buttons at least 96px tall, in the serif.
- **Header, 92px, chrome:** the logo; in the middle "Kitchen Display" in the
  serif with a chef's hat, and under it the station's name and "Fresh
  orders. Happy people." ("Espresso night" on night); at the right the date
  and time from 1280px, the connection indicator, and a script line from
  1536px.
- **All day**, a strip, when there is anything to make: how many of each
  dish, new and in preparation together, most first: "3 Masala dosa · 2
  Upma". The counts are terracotta, tabular.
- **Today**, one line, under 1536px: the stats below, and how many are
  coming next.
- A line for someone who may only look: "You can see the board. Starting and
  finishing tickets is the kitchen's."
- **Three columns**, each a card with a 64px band on top: an icon, its name
  in the serif, and its count in a round mark.

| Column | Band | Holds | Order | Its button |
|---|---|---|---|---|
| New Orders | terracotta | sent, untouched | oldest first | **Start preparing** (green) |
| Preparing | mustard | a cook has it | oldest first | **Mark ready** (outlined) |
| Ready for pickup | green | on the pass, while it can be recalled | latest on top | **Recall** (green tint) |

- **Today's stats and Coming next**, a fourth column, 340px, from 1536px
  wide (below).
- **Foot, 76px, chrome:** **Change station**; **Sound on** or **Sound off**;
  **Group by table** and **Night mode** as switches that say On or Off; and
  at the right a script line, or on a terminal **Lock · Ravi**.

**A ticket card** has a rule down its left and a border in its column's
colour:

- An icon tile for how it leaves the pass (dine-in, takeaway, delivery).
- **Where it goes**, in the serif at 32px: "T4", "T4 + T7", or "Counter ·
  NMRN" with the order's short id. Under it "4 guests · Dine-in".
- At the right, when it was sent, and **the timer** as a pill, tabular:
  "7:42". The timer has its own tone and word against the venue's late time:
  plain under half of it, mustard and **WAITING** from half, the danger
  tint and **LATE** from the late time. A ready card has "Ready · 2m"
  instead.
- The order's notes in a mustard box.
- **The lines:** the quantity and the name at 26px; choices under them;
  notes in mustard ink; a line voided after it was sent struck through,
  "Void". A ready card shows names and quantities only.
- In Preparing, a bar towards late, mustard, then terracotta once late.
- The column's button, full width, 64px.

**Today's stats** (`BoardStats`), the station's trading day so far: **Total
tickets**, **In progress**, **Completed**; then under a rule **Avg. prep
time** ("11 min", or "Not yet") and **Slowest today** ("Biryani · 19 min").

**Coming next (N)** (ADR-181): what this station will be sent when a later
course is fired. Each: where ("T2"), the course as a chip ("Dessert"), and
the dishes ("1 × Lemon tart"). "And 3 more behind these." past the list.

**Espresso night** (D11) is the same board on the ink surface: every token
under it turns (tokens.md). Nothing else differs.

## Data

- `GET /api/v1/menu` for the stations and the courses' names.
- `GET /api/v1/kitchen/{station}/tickets`, key `["kitchen", station]`:
  `tickets[]` (status, sent_at, started_at, ready_at, table, covers,
  service, notes, lines), `stats`, `coming`, `coming_more`,
  `recall_minutes`, `late_after_minutes`. A kitchen device may read it, and
  so may a person with `kitchen.view` on any device.
- The moves, each answering with the board: `POST
  /api/v1/kitchen/orders/{id}/tickets/{seq}/start`, `.../bump`,
  `.../recall`. A kitchen device may make them; a person needs
  `kitchen.work` (ADR-180).
- The wall display's stream is subscribed to `station:{code}` alone; any
  event there reads the board again, and so does a resync. A line voided or
  noted elsewhere carries no station, so the board is also read every 20
  seconds.
- Kept on the device (`mez.kitchen.prefs`): night, sound, group by table. A
  display starts on paper, silent, in the order tickets were sent. Night is
  set before anything is drawn, so a dark pass is never lit by a flash.
- The timers tick every second, on the device's clock.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| Choose a station | Keeps it on the device | | The board. The wall display reloads, to listen to that station alone; a terminal does not need to |
| Start preparing | `.../start` | `ticket.started` | The card moves to Preparing |
| Mark ready | `.../bump` | `ticket.bumped` | The card moves to Ready for pickup; the order's lines read Ready on the order screen |
| Recall | `.../recall` | `ticket.recalled` | The card is back in New Orders |
| Sound | Toggles the chime | | Two rising notes when a ticket arrives in New Orders |
| Group by table | Toggles | | In each column an order's tickets sit together, at the place of its first |
| Night mode | Toggles the surface | | Kept across a reload |
| Change station | Back to the choice | | |
| Lock (a terminal) | Signs the cook out | | Staff Login, on paper |

One move at a time: while one is on its way every button waits, and its own
says "Starting", "Marking" or "Recalling".

## States

- **No stations:** "This venue has no stations yet. Add them with the menu."
- **Loading:** "Loading the stations", then "Loading the tickets".
- **An empty column:** "Nothing new.", "Nothing on the go.", "The pass is
  clear."
- **Nothing held back:** "Nothing is waiting for a later course."
- **No ticket ready yet today:** the two times read "Not yet".
- **Error:** the server's words over the columns. A move refused says why
  and the card stays where it was.
- **Offline:** the board last read stays; the connection indicator says what
  is wrong, with Retry now.
- **Permission denied:** a person with `kitchen.view` and not `kitchen.work`
  sees the board with no buttons and the line saying so.
- **Storage blocked:** the station and the switches last until the page is
  reloaded.

## Constraints

- **Type for three metres:** 32px for where a ticket goes, 26px for its
  lines (`--text-kitchen`), 20px for the timer. Not the design's 20 and 16.
- A ticket's button is 64px tall; nothing in the foot is under 48px.
- The column gives a card its colour; the timer keeps its own tone **and its
  word**. The danger tint here means Late, only.
- A card is never removed under a finger: it leaves on its own move, and the
  others close up.
- The chime is made by the browser's own oscillator: no sound file, so it
  works with no internet. A browser lets a page make sound only after a
  touch; turning Sound on is that touch. A display turned on to a full board
  stays quiet.
- A ready ticket stays for `recall_minutes`, then leaves the board. Nothing
  marks it collected (ADR-090).
- The counts are of tickets, which is what a station is sent: an order sent
  to twice is two.

## Reference

`mez Desktop KDS and Dashboard.dc.html`, D10 KDS Parchment (line 36) and D11
KDS Night (line 158). Left out, and why:

- **"Called Rohan" on a ready ticket:** nothing calls anyone, and the display
  knows no waiter. **Recall** stands there.
- **Settings:** a display has one setting besides the switches, its station,
  so the button says that.
- **Auto advance:** nothing moves a ticket on its own; one that left the
  board unmade would be a lost dish.
- **Mark all complete:** no ticket is "complete" here, and one tap that
  clears a pass has no undo.
- **"Upcoming (next 5)" with minutes to go:** nothing knows when an order
  will be sent. Coming next lists what is known, a later course held back,
  with the course where the design has minutes.
- **The spot art.**

Changed: "Total orders" is **Total tickets**. D11's "Tonight's stats" and
"Longest wait" are the day's stats under the same words as D10, since night
is a surface, not another board. The header's "Kitchen Online" is the
connection indicator, which says more when it is not.

Kept, though not drawn (rule 4): the station's name in the header, the
all-day strip, Recall on each ready card, the timer's Waiting and Late, the
bar towards late, and Lock for a cook on a terminal.
