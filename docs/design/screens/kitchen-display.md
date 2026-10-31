# Kitchen: the board

The pass: every ticket a station has to make, in three columns it crosses from
left to right, read from two or three metres with wet hands. Built in phase 08
(ADR-090, ADR-091), rebuilt from D10 and D11 in the rebuild's Task 15
(ADR-175 to ADR-181), and fitted to the venue's screens in Phase 17 Task 10
(ADR-381). `web/terminal/src/kitchen/`: `KitchenApp.tsx`, `CookBoard.tsx`,
`KitchenBoard.tsx`, `BoardHead.tsx`, `TicketCard.tsx`, `BoardFoot.tsx`,
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

Full screen, on espresso night to start, with a header, the board and a foot
(ADR-381, over ADR-177's paper: tokens.md names ink for the
kitchen; paper stays a choice in the foot). Only the columns scroll, each on
its own. On a screen 820px tall or less (`short:`) the header, the strips, the
bands and the foot give up their padding so the tickets keep the height; no
type gets smaller.

- **Which station is this?** (first run, or Change station): the venue's
  stations as buttons at least 96px tall, in the serif.
- **Header, chrome, 92px (56px under `short:`):** the logo; in the middle the
  **station's name** in the serif with a chef's hat, at `text-total` (32px),
  `text-display` from 1536px wide; at the right the date and time from
  1280px, and the connection indicator. The station is what a cook checks from
  across the pass, so it is the title. D10's "Kitchen Display", its tagline
  ("Fresh orders. Happy people.", "Espresso night") and its script lines are
  gone (ADR-381): decoration is for screens that wait.
- **All day**, a strip, when there is anything to make: how many of each
  dish, new and in preparation together, most first: "3 Masala dosa 2 Upma".
  The counts are terracotta, tabular. **One line, always:** a dish that does
  not fit is hidden whole, and the line ends "+3 more".
- **Coming next**, a strip of the same kind under 1536px wide, only when a
  later course is held back for this station (ADR-381): where, the course as
  a chip, and the dishes, "T2 Dessert 1 × Lemon tart". One line; its "+N
  more" counts what does not fit and what the server holds past its list.
- A line for someone who may only look: "You can see the board. Starting and
  finishing tickets is the kitchen's."
- **Three columns**, each a card with a band on top, 64px (48px under
  `short:`): an icon, its name in the serif, and its count in a round mark.

| Column | Band | Holds | Order | Its button |
|---|---|---|---|---|
| New orders | terracotta | sent, untouched | oldest first | **Start preparing** (green) |
| Preparing | mustard | a cook has it | oldest first | **Mark ready** (outlined) |
| Ready for pickup | green | on the pass, while it can be recalled | latest on top | **Recall** (green tint) |

- **From 1536px wide, a fourth column** at the `w-aside` width (380px at
  1920; it was a fixed 340px, TECH-17): Coming next when it has rows, and
  under it today's stats in one line (below). Under 1536px the stats are not
  on the board (ADR-381, over ADR-178's line under the all-day strip): on a
  768 screen they cost a ticket's height.
- **Foot, chrome, 76px (56px under `short:`, its buttons still 48px):**
  **Change station**; then switches that say On or Off: **Sound** (a switch
  like its neighbours, KDS-16), **Group by table**, **Skip preparing** (KDS-17)
  and **Night mode**; and on a terminal, at the right, **Lock · Ravi**. No
  script line (ADR-381).

**At the venue's sizes**, with one counter ticket of three lines:

| Screen | Columns | What fits |
|---|---|---|
| 1024x768 | three of about 320px | Header 56, All day 42, bands 48, foot 56: a column has about 540px for tickets. The first ticket is whole, its button in view. "Counter · Z2ZE" holds one line; the timer sits under it. |
| 1366x768 | three of about 430px | The same chrome. A three-line ticket and its button, with room for its choices and a note. Where it goes and a calm timer share a row; Waiting or Late wraps under. |
| 1920x1080 | three of about 485px, and the fourth | The full chrome. The station's name, where each ticket goes and its timer at 56px; the lines at about 34px. |

**A ticket card** has a rule down its left and a border in its column's
colour, and 16px of padding (12px under `short:`):

- **Where it goes**, in the serif at `text-display`: "T4", "T4 + T7", or for
  a counter order its short id, with "Counter ·" before it at `text-kitchen`.
  The short id is what tells two counter orders apart, and the pair then holds
  one line in a 1024 column.
- **The timer** beside it as a pill, the digits at `text-display`, tabular:
  "7:42". When the column has no room for both, it wraps under where it goes.
  The timer has its own tone and word against the venue's late time: plain
  under half of it, mustard and **WAITING** from half, the danger tint and
  **LATE** from the late time. The word is `text-xl`, on the digits' line. A
  ready card has "Ready · 2m" instead, at `text-xl`.
- Under them, one line: **New** for a minute after it arrived, a chip for a
  later send of the order to this station (its course when a later one,
  "Mains", or **Add-on**; KDS-12, ADR-392), a small icon for how it leaves
  the pass, then who, how and when it was placed: "4 guests · Dine-in ·
  12:20 AM". D10's icon
  tile is gone (ADR-381): on a 300px column it pushed where it goes onto two
  lines.
- The order's notes in a mustard box (`chip-warning`) that starts with
  **Note:**, at `text-kitchen`.
- **The lines:** the quantity and the name at `text-kitchen`, semibold;
  choices under them at `text-kitchen`, normal weight; each of a line's notes
  in a **Note:** box, so a note differs from a choice by more than its colour.
  A line voided after it was sent has its quantity and name struck through
  and a danger chip, **Void**, that is not; the void reaches the board as it
  happens, and chimes two falling notes (KDS-13, ADR-392). A ready card
  shows names, quantities and notes; not the choices.
- In Preparing, a bar towards late, mustard, then terracotta once late.
- The column's button, full width, 64px.

**Today's stats** (`BoardStats`, from 1536px wide), the station's trading day
so far, folded into one line under Coming next: "Today", then **Total
tickets**, **In progress**, **Completed**, **Avg. prep time** ("11 min", or
"Not yet") and **Slowest today** ("Biryani · 19 min").

**Coming next (N)** (ADR-181): what this station will be sent when a later
course is fired. Each: where ("T2"), the course as a chip ("Dessert"), and the
dishes ("1 × Lemon tart"). Drawn only when it has rows (ADR-381): in the
fourth column from 1536px, with "And 3 more behind these." past the list; as
a strip under All day below that.

**Espresso night** (D11) is the same board on the ink surface: every token
under it turns (tokens.md). Paper is the same board again. Nothing else
differs.

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
  display starts on espresso night (ADR-381, over ADR-177's paper), with
  sound on (ADR-391), in the order tickets were sent. A display that kept its
  choices before ADR-391 takes sound on once, and keeps the rest. A display that kept any of its switches
  before ADR-381 keeps the surface it had until Night mode is turned on.
  Night is set before anything is drawn, so a dark pass is never lit by a
  flash.
- The timers tick every second, on the device's clock.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| Choose a station | Keeps it on the device | | The board. The wall display reloads, to listen to that station alone; a terminal does not need to |
| Start preparing | `.../start` | `ticket.started` | The card moves to Preparing |
| Mark ready | `.../bump` | `ticket.bumped` | The card moves to Ready for pickup; the order's lines read Ready on the order screen |
| Recall | Asks for a second tap: "Tap again to recall", for 4 s (KDS-15); then `.../recall` | `ticket.recalled` | The card is back in New orders |
| Sound | Toggles the chime | | Two rising notes, twice, half a second apart, when a ticket arrives in New orders (ADR-391) |
| Group by table | Toggles | | In each column an order's tickets sit together, at the place of its first |
| Skip preparing | Toggles, off to start | | New orders' button is **Mark ready**: one tap puts the ticket on the pass, for a bar (KDS-17, ADR-392) |
| Night mode | Toggles the surface, on to start | | Off is paper, for a bright kitchen. Kept across a reload |
| Change station | Asks for a second tap: "Tap again to change station", for 4 s (KDS-15); then back to the choice | | |
| Lock (a terminal) | Signs the cook out | | Staff login, on paper |

One move at a time: while one is on its way every button waits, and its own
says "Starting", "Marking" or "Recalling". A tap within 700 ms of the board
moving is not taken: it is a double tap landing on the card that slid into
place (KDS-3, ADR-391). On night every action button has a 2px edge in the
muted ink, 5:1 on the card (KDS-9).

A ticket that arrived in the last minute says **New** before its meta line;
when one is under New orders' fold, a terracotta button at the column's foot
says "1 new below" and scrolls to it (KDS-1).

## States

- **No stations:** "This venue has no stations yet. Add them with the menu."
- **Loading:** "Loading the stations", then "Loading the tickets".
- **An empty column:** "Nothing new.", "Nothing on the go.", "The pass is
  clear."
- **Nothing held back:** Coming next is not drawn (ADR-381).
- **No ticket ready yet today:** the two times read "Not yet".
- **Error:** over the columns, in the kitchen's words, naming the ticket:
  "T4 has already moved on another screen.", "T4 did not move: no answer
  from the server. Tap again." The card stays where it was, and the line goes
  after 8 seconds or at the next move (KDS-14, ADR-391). A board that cannot
  be read says "The tickets could not be read. Trying again."
- **Offline:** the board last read stays; the connection indicator says what
  is wrong, with Retry now, and after 10 seconds without the live stream a
  brick band under the header says "Not up to date: no word from the server
  for 12 seconds. New tickets may be missing." until it is back (KDS-2).
- **Sound held back:** sound is on and the browser has not let the page play
  it (after a reload, before anyone has touched it): a terracotta band says
  "Sound is blocked by the browser. Tap here to turn it on, so new tickets
  chime." The first tap anywhere on the board turns it on (KDS-1).
- **Permission denied:** a person with `kitchen.view` and not `kitchen.work`
  sees the board with no buttons and the line saying so.
- **Storage blocked:** the station and the switches last until the page is
  reloaded.

## Constraints

- **Type for two metres and more** (ADR-381, over ADR-176's 32px and 20px):
  `text-display` for where a ticket goes and its timer, `text-kitchen` for
  its lines, choices and notes. Both grow with the screen's shorter side (44
  to 56px and 26 to 36px), so a large screen at the pass reads from further
  off. Not the design's 20 and 16. `short:` trims chrome, never these.
- A ticket's button is 64px tall; nothing in the foot is under 48px.
- The column gives a card its colour; the timer keeps its own tone **and its
  word**. The danger tint here means Late, only.
- A card is never removed under a finger: it leaves on its own move, and the
  others close up.
- The chime is made by the browser's own oscillator: no sound file, so it
  works with no internet. A browser lets a page make sound only after a
  touch; the band says so until one comes. A display turned on to a full
  board stays quiet.
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
- **"Kitchen Display", the tagline and the script lines** (ADR-381): the
  station's name is the title, and a working screen carries no decoration.
- **The icon tile on a card** (ADR-381): a small icon in the line under where
  it goes stands for it, so where it goes and the timer can share a row.
- **The spot art.**

Changed: "Total orders" is **Total tickets**. D11's "Tonight's stats" and
"Longest wait" are the day's stats under the same words as D10, since night
is a surface, not another board. The header's "Kitchen Online" is the
connection indicator, which says more when it is not. Today's stats is one
line under Coming next, not a card of its own, and only from 1536px
(ADR-381).

Kept, though not drawn (rule 4): the station's name as the title, the
all-day strip, Recall on each ready card, the timer's Waiting and Late, the
bar towards late, and Lock for a cook on a terminal.
