# Terminal: Lock

The staff sign-in, and before it the pairing screen that a device sees once.
Built in phase 05, Task 6. The same two screens serve the back office on
paper: `web/shared/screens/LockScreen.tsx` and `PairScreen.tsx`.

## Purpose

Get the right person onto this till in two taps and four digits, and tell
them plainly when that cannot happen.

## Layout

**Lock**, one screen, nothing scrolls on a 12-inch tablet:

- Top right, fixed: the connection indicator. Staff must know the server is
  there before they trust a PIN refusal.
- Left column: the metadata trail (`MEZ · SIGN IN`), the device's name
  in the serif, one line of instruction, an expiry notice if the last
  session ended on its own, then the staff name tiles (initials and name,
  48px or taller, in a grid of 10rem columns).
- Right column, 22rem, a chrome panel: the chosen name, PIN dots, the NumPad
  at 64px keys, **Sign in** (primary), a reserved error line, and "Forgot
  your PIN? Ask a manager to reset it."
- Below `lg` the columns stack, tiles first.

**Pair**, one centred column, 28rem: trail, "Pair this terminal", what a code
is and how long it lasts, a notice if the device was just unpaired, the code
field (tabular, 20px, `XXXX-XXXX`), the error line, **Pair**, and the server's
address.

## Data

- `GET /api/v1/devices/me` at start, through `AuthStore.boot()`: is the
  stored device still paired? Refused sends the device to Pair with a reason.
- `GET /api/v1/staff`: the name tiles. React Query, key `["staff"]`.
- `POST /api/v1/sessions` `{ staff_id, pin }`: sign in. The session token goes
  to `sessionStorage`, the device token to `localStorage`, both under
  `mez.<app>.*` (`web/shared/auth/storage.ts`).
- `POST /api/v1/devices/pair` `{ code }`: pairing.
- `DELETE /api/v1/sessions/current`: Lock, from the signed-in header.
- WebSocket: none of its own. The indicator reflects the app's stream, and a
  stream refused as unpaired sends the screen to Pair.

## Interactions

| Action | Does | Then |
|---|---|---|
| Tap a name | Chooses it, clears the PIN | Pad enabled. With one name on the list it is chosen already |
| Tap a digit, or type one | Appends, up to 8 | A dot fills; past four, a dot is added |
| Delete / Clear keys, Backspace / Delete | Remove one digit / all | |
| Sign in, or Enter | `POST /sessions` once the PIN has 4 digits | Signed in: the Gate goes to `/` |
| Wrong PIN | | "That PIN isn't right. Try again." PIN cleared |
| Locked out (429) | | "Too many wrong PINs. Try again in 14:41." counting down from `Retry-After`; pad and Sign in disabled until it ends |
| Server unreachable | | "Can't reach the mez server at host. ... Your PIN wasn't checked." |
| Lock (signed in) | Ends the session, locally even if the server cannot be told | Back to Lock |

## States

- **Loading:** a splash ("Checking this device") until the stored device is
  confirmed, so no screen renders on a guess; then "Loading staff…" in the
  tile area.
- **Empty:** "Nobody can sign in here yet. Add staff in the back office."
- **Error:** the staff list failing says where the server should be, with
  Try again. Sign-in errors are listed above; each is one line, in the
  reserved space, so nothing below it moves.
- **Offline:** if the server is unreachable at start, the stored device and
  session are trusted and the indicator says the server is missing. A till
  that boots before its server must not have to pair again.
- **Session expired:** "Your session ended. Sign in again to carry on."
- **Unpaired:** Pair, with "This device was unpaired from the back office" or
  "The server no longer recognises this device".
- **Permission denied:** not on this screen; any signed-in staff member may
  sign in.

## Constraints

- The pad works one-handed: 64px keys, the whole pad within a thumb's reach
  of the bottom right on a landscape tablet.
- A physical keyboard types into the PIN from anywhere on the screen; the
  pad claims the keys it handles itself, so nothing is typed twice.
- The error line is always reserved (`min-h-6`), so an error never moves the
  Sign in button out from under a finger.
- Nothing tells an onlooker more than the dots already do. A wrong PIN and an
  unknown name read the same (ADR-056).

## Reference

The layout was first drawn from the Nofiltr archive
(`docs/design/reference/Nofiltr Terminal.dc.html`, block `s_lock`), kept as
history. Its look is now `docs/design/tokens.md` and
`docs/design/reference/mez/mez Desktop POS.dc.html`. Departures from the
first:

- **PIN dots grow from four.** The design has four fixed dots and opens on
  the fourth digit; PINs are 4 to 8 digits (ADR-056), so there is a Sign in
  button and Enter, not auto-submit.
- **"Sign in", not "Open shift".** Shifts arrive in phase 09; the button will
  open one then.
- **The device's name, not the venue's**, in the serif heading. No endpoint
  returns the venue's name yet.
- **No role under each name.** `GET /staff` returns names only.
- **Keys are 64px, not 62px:** the nearest step on the spacing scale.
- **The "n" monogram tile is left out** until there is a logo to put in it.
- A connection indicator is added top right. The design has none here, but a
  PIN refused by a server that is not there reads as a wrong PIN otherwise.
