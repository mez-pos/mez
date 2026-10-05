# Terminal: Staff Login, and pairing

The sign-in screen everyone starts a shift on, and before it the pairing
screen a device sees once. Rebuilt from D01 in the rebuild's Task 2 (ADR-148).
The same two screens serve the back office: `web/shared/screens/LockScreen.tsx`
with `login/BrandPanel.tsx`, `ProfileTiles.tsx` and `LoginPad.tsx`, and
`PairScreen.tsx`.

## Purpose

Get the right person onto this device in one tap and four digits, tell them
plainly when that cannot happen, and send them on to their own work.

## Layout

**Staff Login**, two columns, nothing scrolls at 1366x768:

- **Left, 46% wide, on the sunken ground** (`BrandPanel`): the logo, "Good
  food brings people together" in small caps, "Great Food / Brighter Days" in
  the serif, a script line under it, and along the foot the server's version
  ("mez v1.0.0") and a second small-caps line. A pale arch sits behind. It is
  decoration, hidden from a screen reader, and hidden altogether under 1024px.
- **Right, the rest:**
  - A script line top right ("Good Teams, Great Service"), from 768px wide.
  - The logo, only where the left panel is hidden.
  - **Staff Login** in the serif, and "Select your profile and enter your PIN".
  - A notice, when the last session ended on its own.
  - **The profiles** (`ProfileTiles`): four to a page. Each is a round avatar
    with the person's initials, their name, and their role under it. The
    chosen one has the terracotta ring and tint. With more than four people a
    round **More** follows the fourth and goes to the next page, round to the
    first.
  - **The pad** (`LoginPad`), 30rem at most: a well with the chosen person's
    name at its left ("Choose a profile" before anyone is) and the PIN dots in
    its middle; under it a 3 by 4 grid of keys: 1 to 9, delete, 0, and the
    green **Login** with its arrow as the twelfth key.
  - The error line, always reserved.
  - "Forgot your PIN? Ask a manager to reset it."
  - **The foot:** the venue's name, the connection indicator, and the venue's
    date and time.

**Pair**, one centred column, 28rem: the logo, the trail (`TERMINAL ·
PAIRING`), "Pair this terminal" in the serif, what a code is and how long it
lasts, a notice if the device was just unpaired, the code field (the numeral
face, 20px, `XXXX-XXXX`), the error line, **Pair**, and the server's address.
The words follow the app: terminal, back office, kitchen display.

## Data

- `GET /api/v1/devices/me` at start, through `AuthStore.boot()`: is the
  stored device still paired? Refused sends the device to Pair with a reason.
- `GET /api/v1/staff`, key `["staff"]`: the profiles, with each person's
  role. Read afresh every time the screen shows (ADR-115).
- `GET /api/v1/venue`: the venue's name and time zone, for the foot.
  `GET /api/v1/health`: the version.
- `POST /api/v1/sessions` `{ staff_id, pin }`: sign in. The session token goes
  to `sessionStorage`, the device token to `localStorage`, both under
  `mez.<app>.*` (`web/shared/auth/storage.ts`).
- `POST /api/v1/devices/pair` `{ code }`: pairing.
- `DELETE /api/v1/sessions/current`: Lock, from the profile menu.
- WebSocket: none of its own. The indicator reflects the app's stream, and a
  stream refused as unpaired sends the screen to Pair.

## Interactions

| Action | Does | Then |
|---|---|---|
| Tap a profile | Chooses it, clears the PIN | The pad is enabled and names them. With one person on the list they are chosen already |
| More | Shows the next four | Whoever is chosen stays chosen; the pad still names them |
| Tap a digit, or type one | Appends, up to 8 | A dot fills; past four, a dot is added |
| Delete, or Backspace | Removes one digit | |
| Login, or Enter | `POST /sessions` once the PIN has 4 digits | Signed in: the Gate goes to `/`, which lands the person by their role (`terminal-shell.md`) |
| Wrong PIN | | "That PIN isn't right. Try again." The PIN is cleared |
| Locked out (429) | | "Too many wrong PINs. Try again in 14:41." counting down from `Retry-After`; the pad and Login wait until it ends |
| Server unreachable | | "Can't reach the mez server at host. Check this device is on the venue's network. Your PIN wasn't checked." |
| Pair | `POST /devices/pair` once the code is whole | Paired: the Gate goes to Staff Login |

Enter signs in from anywhere on the screen, except on a focused button or
link, where it is that control's own click: a profile is chosen, More pages.

## States

- **Loading:** a splash (the logo and "Checking this device") until the stored
  device is confirmed, so no screen renders on a guess; then "Loading staff…"
  where the profiles go.
- **Empty:** "Nobody can sign in here yet. Add staff in the back office."
- **Error:** the staff list failing says where the server should be, with
  **Try again**. Each sign-in error is one line in the reserved space, so
  nothing under it moves.
- **Offline:** if the server is unreachable at start, the stored device and
  session are trusted and the indicator says what is wrong, with **Retry
  now**. A device that boots before its server must not have to pair again.
- **Session expired:** "Your session ended. Sign in again to carry on."
- **Someone removed while chosen:** they stop being chosen and what was typed
  goes.
- **Unpaired:** Pair, with "This device was unpaired from the back office" or
  "The server no longer recognises this device".
- **A code for another app:** "That code pairs a kitchen display, and this is
  the terminal. Ask for a terminal code."
- **Permission denied:** not on this screen; anyone on the list may sign in.

## Constraints

- Keys are 76px on a tall screen and shrink with its height,
  `clamp(48px, 7vh, 76px)`: never under the 48px a thumb needs (ADR-014). The
  spacing above the profiles and the pad tightens under 900px and 800px high.
- A physical keyboard types into the PIN from anywhere on the screen.
- The error line is always reserved (`min-h-6`), so an error never moves
  Login out from under a finger.
- Nothing tells an onlooker more than the dots already do. A wrong PIN and an
  unknown person read the same (ADR-056).
- The role under a name is the role's name as the owner wrote it; it decides
  nothing here.

## Reference

`docs/design/reference/mez/mez Desktop POS.dc.html`, D01 Login (line 36).
Departures:

- **The dots grow from four.** The design has four fixed boxes; PINs are 4 to
  8 digits (ADR-056), so Login is a key and Enter, not an auto-submit on the
  fourth digit.
- **Initials, not photographs.** Nobody has a photo yet.
- **No illustration.** The design's chef at the pass and the dark script card
  beside it are placeholders for artwork that does not exist.
- **Login is the twelfth key**, where the design has Clear: delete is enough
  for eight digits.
- **A connection indicator in the foot.** The design has none here, but a PIN
  refused by a server that is not there reads as a wrong PIN otherwise.
- **Pair is ours.** The design draws no pairing; D24's setup wizard is phase
  12's.
