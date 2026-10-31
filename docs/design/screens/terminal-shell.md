# Terminal: the frame

What is around every signed-in screen of the terminal: the top bar, the
navigation, the status bar, where a person lands, and the shift gate. Built in
the rebuild's Tasks 1 to 3 (ADR-148) and Task 11 for a waiter (ADR-163,
ADR-164). `web/terminal/src/App.tsx` and `shell/`: `TerminalShell.tsx`,
`NavPills.tsx`, `WaiterNav.tsx`, `StatusBar.tsx`, `ShiftGate.tsx`,
`landing.ts`, `nav-logic.ts`, `routes.ts`, `terminal-context.tsx`.

## Purpose

A person signs in and is on their own work, with the two or three places they
go from it one tap away, and can always see whether the server is there.

## Where a person lands

`/` sends a person on by their role (`landingFor`):

| Role | Lands on | Frame |
|---|---|---|
| Owner | `/pos`, the order screen | Desktop, with Menu and Reports in the navigation |
| Manager | `/pos`, the order screen | Desktop, with Reports. The manager role a venue starts with has no `menu.edit`, so no Menu, until the owner adds it |
| Cashier | `/pos`, the order screen | Desktop |
| Waiter | `/waiter`, the tables as cards | Waiter |
| Kitchen | `/pass`, the board | None: the board has its own header and foot |

The role's name decides for the five roles a venue starts with, because a
waiter and a cashier hold the same permissions (ADR-145). A role the owner
made or renamed is landed by what it can do: `order.open` on the order screen,
else `kitchen.view` on the board. Whoever lands on `/waiter` has the waiter's
frame on every screen they reach (`useWaiterFrame`).

## Layout

The frame is the height of the screen and never scrolls; each screen fills the
middle and scrolls inside itself, so an order's total never leaves the screen.

**Desktop frame:**

- **Top bar, 72px:** the logo; the navigation as pills, each 48px with an
  icon, the pressed one the green fill: **Orders**, **Tables**, and for those
  who may open them **Menu** (`menu.edit`) and **Reports** (`reports.view`).
  At the right: **Retry now** while the server is down; **N table
  requests** while a guest's request waits (`terminal-requests.md`); **Not
  printed N** while a print job has failed; the profile (initials, name, role, a chevron);
  and from 1536px wide a script line.
- **The screen.**
- **Status bar, 40px:** the version ("mez v1.0.0"), the connection ("Online",
  or what is wrong and when the next try is), the venue's name, and at the
  right the venue's date and time, to the second's tick, in the venue's zone.

**Waiter's frame** (T01), for a tablet in one hand:

- **Top bar, 88px:** the logo, then what the screen needs:
  - On most screens the navigation as tall tiles, an icon over a word:
    **Home**, **Orders**, **Menu**, **More**. More opens Floor plan, Your
    shift and Print queue.
  - On the order screen, the order's own bar: the way back, the table, the
    guests, how it is served (`waiter-order.md`).
  - On a table's screen, **Back** alone.
  - On the bill, nothing: the bill carries its own way back.
- At the right: the connection as a word ("Online", "Server unreachable"),
  Retry now, table requests, Not printed, the profile.

**The right-hand cluster gives way below 1280px** (TBS-3), so the screen's
own bar beside it (the waiter's order bar on a 1024 tablet) never runs under
it: its gaps close from 20px to 12px, and a waiter's connection, while it is
"Online", is its Wi-Fi icon alone, in green, the word still read out to a
screen reader. Any other state ("Reconnecting", "Server unreachable") keeps
its word at every width.
- **No status bar.** The top bar says whether the server is there.

**The profile menu** opens under the name: **Lock**; **Close the shift**, for
someone with `shift.manage` while a shift is open; **Print queue**.

## Paths

| Path | Screen |
|---|---|
| `/pair`, `/lock` | Pairing and Staff Login (`terminal-lock.md`) |
| `/pos` | The order screen. Not `/order`: the server gives that to the guest app |
| `/tables` | Tables; `?table=N` with one picked |
| `/orders`, `/orders/:id` | The day's orders, and one picked |
| `/pay/:id` | Payment; for a waiter, the bill at the table |
| `/shift` | Open the shift (`terminal-open-shift.md`) |
| `/shift/close` | Close the day (`terminal-close.md`) |
| `/shift/z/:n` | The Z-report a close just issued (`zreport.md`) |
| `/waiter`, `/waiter/table/:id`, `/waiter/shift` | The waiter's home, a table, their shift |
| `/pass` | The kitchen's board for a cook signed in here, outside the frame and the shift gate |
| `/kitchen` | The wall display: another app of this bundle, with a pairing of its own |

Anywhere else is landed again by role.

## Data

- The signed-in person, their role and permissions, and the device, from the
  auth store.
- `GET /api/v1/venue` (name, zone) and `GET /api/v1/health` (version), once.
- `GET /api/v1/shift`, key `["shift"]`, on start and every 30 seconds: the
  stream carries no shifts.
- `GET /api/v1/print-jobs`, for the Not printed pill.
- `GET /api/v1/guest-requests`, for the table requests pill and its chime.
- Held above the routes (`TerminalProvider`), so moving between screens never
  starts a fresh order: the order in front, the live floor, the shift, table
  names by id, and the number of the Z-report this device's close last issued.
- WebSocket: the app's one stream. The connection word and Retry now read its
  state.

## Interactions

| Action | Does | Then |
|---|---|---|
| Orders | Opens `/pos` | Pressed on the order screen, the day's orders and payment |
| Tables | Opens `/tables` | Pressed there |
| Menu, Reports | Leave for `/backoffice/menu`, `/backoffice/sales` | The back office is another app: it has its own pairing and sign-in |
| The profile | Opens its menu | Focus moves to the first choice; Escape, a tap elsewhere or Tab leaving closes it |
| Lock | Ends the session, locally even if the server cannot be told | Staff Login |
| Close the shift | Opens `/shift/close` | Close the day |
| Print queue, Not printed | Opens the print queue sheet | `terminal-print-queue.md` |
| Retry now | Tries the stream at once | The connection word follows |
| More (waiter) | Opens Floor plan, Your shift, Print queue | Pressed on the floor plan and the shift summary |

## The shift gate

Every screen inside the frame waits on the shift (`shiftGate`, ADR-094):

- **Not yet known:** "Checking the shift", and no screen behind it.
- **No shift open:** every path goes to `/shift`. A Z-report just issued can
  still be read at `/shift/z/:n`.
- **The drawer has been counted:** every path goes to `/shift/close`. The
  close is the only way on.
- **A shift open:** the screen asked for stands; `/shift` itself goes to the
  order screen.

## States

- **Loading:** "Checking the shift" in the middle of the frame.
- **Error:** "Can't reach the mez server to check the shift." with **Try
  again**.
- **Offline:** the frame stays. The status bar, or a waiter's top bar, says
  "Reconnecting", "Server unreachable" or "No network" in its colour and its
  word, with "retrying in 4s" beside it on a desktop, and Retry now is in the
  top bar. Nothing here needs the internet: the word is about the mez server.
- **Permission denied:** a pill is shown only to someone who can do something
  behind it, so none leads to a refusal. Permissions changed while someone is
  signed in reach the frame at their next sign-in.

## Constraints

- Everything tapped in the frame is 48px or taller. The pills are 48px, not
  the design's 44, and a waiter's tile label is 15px, not 12.5 (ADR-014).
- The connection is a colour and a word, never a colour alone. The one
  exception is a waiter's "Online" below 1280px, which is the icon alone:
  only the calm state gives up its word, and its icon is not the crossed-out
  one an offline state shows.
- The clock is the venue's, not the device's, so a tablet set wrong still
  shows the trading day right.
- Nothing in the frame is drawn that does nothing.

## Reference

`mez Desktop POS.dc.html`, D03 POS Order (line 175) and D05 Floor Plan (line
363) for the desktop frame; `mez Tablet Waiter.dc.html`, T01 Waiter Home (line
36) for the waiter's. Left out, and why:

- **Keyboard Shortcuts and Open Cash Drawer** in the status bar: neither
  exists. The drawer opens with a settling receipt (ADR-081).
- **The bell and Table Requests** are built, as a pill that is there only
  while a request waits (`terminal-requests.md`, ADR-212).
- **Search in the top bar:** each screen that searches has its own.

Kept, though the design does not draw it (rule 4): Lock, Close the shift and
Print queue under the profile; Floor plan, Your shift and Print queue under a
waiter's More; the Not printed pill, because a receipt that did not print must
not wait for someone to look for it.
