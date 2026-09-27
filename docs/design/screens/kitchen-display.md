# Kitchen display

The pass, on a tablet on the wall: every ticket a station has to make,
oldest first, with how long it has waited, read from three metres with wet
hands. Built in phase 08 Task 4b (ADR-091), on the kitchen API of Task 4a
(ADR-090).

## Purpose

Make the next ticket, bump it when it is on the pass, and see at a glance
which tickets are late and how many of each dish are still to go.

## Where it lives

`/kitchen` on the server, served by the terminal's bundle. It pairs as a
**kitchen** device (`nofiltr setup -device-kind kitchen`, or a code a
manager makes with kind kitchen) and never asks for a PIN: a screen on a
wall has nobody to type one. A code for a terminal is refused here, as
anywhere (ADR-068).

## Layout

Ink surface, full screen, no shell chrome but a thin header.

- **Choose a station** (first run, or Change station): the venue's
  stations as buttons at least 96px tall, "Kitchen", "Bar". The choice is
  kept on the device.
- **Header** (64px): the station's name in Instrument Serif, the time, the
  connection chip, **Recall** (the last ticket bumped, "Recall T4") and
  **Change station**.
- **All day**: a strip of counts over the tickets still to make, by dish,
  most first: "3 Masala dosa · 2 Upma · 1 Avocado toast". Mono numbers.
- **Tickets**: cards in a grid, oldest first, at least 280px wide. Each:
  - where it goes, very large: "T4", "T4 + T7", or "Counter" with the
    order's short id
  - its timer, very large and mono, "7:42", and its covers
  - the timer's tone: the surface under half the venue's late time, gold
    ("Waiting") from half, vermilion ("Late") from the late time. Colour
    and the word, both.
  - the lines: quantity and name large; modifiers under them; notes in
    italic; a line voided after it was sent struck through, "Void"
  - the order's own notes at the top, when it has any
  - **Bump**, full width, at least 72px tall, at the card's foot
- A card is never removed under a finger: a bumped card goes, the others
  close up.

## Interactions

| Action | Does | Then |
|---|---|---|
| Choose a station | Keeps it on the device, listens to that station alone | The board |
| Bump | `POST .../bump` | The card leaves; Recall names it |
| Recall | `POST .../recall` on the last bumped | The card comes back where its time puts it |
| Change station | Back to the choice | |

## Data

- `GET /api/v1/menu` for the stations (a paired device reads it).
- `GET /api/v1/kitchen/{station}/tickets`: `tickets[]` (status, sent_at,
  ready_at?, table, covers, notes, lines), `recall_minutes`,
  `late_after_minutes`. React Query key `["kitchen", station]`; bump and
  recall answer with it and replace it.
- The stream, subscribed to `station:{code}` only: any event there reads
  the board again, and a resync does. A line voided or noted elsewhere
  carries no station, so the board is also read every 20 seconds.

## States

- **No stations:** "This venue has no stations yet. Add them with the
  menu."
- **Nothing to make:** "All clear." large, muted.
- **Offline:** the board last read stays; the connection chip says so, and
  Bump is disabled until the server answers.

## Constraints

- Nothing tappable under 72px except Change station (56px).
- Type: 44px for where and when, 26px for lines (`--text-display`,
  `--text-kitchen`).
- One accent: vermilion is "Late", only.
