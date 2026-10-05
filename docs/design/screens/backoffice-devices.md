# Back office: Devices

Every tablet, display and computer paired to the venue, and the way to pair
another or take one off. Built in phase 10 Task 6k (ADR-140) and laid out
again on Task 16's patterns in the rebuild's Task 16k (ADR-195).
`web/backoffice/src/devices/`: `DevicesScreen.tsx`, `DeviceTable.tsx`,
`DeviceModals.tsx`, with their logic and `use-devices.ts`. Section
**Devices**, path `/devices`, `device.manage`, keys `g e`. Settings links here
from its rail.

## Purpose

Know what is connected to the venue, and take a device off it in one step
when it should not be, without anyone opening a database.

## Layout

Inside the frame (`backoffice-shell.md`).

- **The head:** "Devices" in the serif, a line of counts under it, and
  **Pair a device** at the right.
- A line over the table for what an action was refused for.
- **The table**, in a card: those in service first, in the order paired,
  the revoked at the foot.

  | Column | Shows |
  |---|---|
  | Device | its name, "This device" beside the one the back office is running on, and its kind in words under it: **Till**, **Kitchen display**, **Back office** |
  | Paired | the day ("3 Oct 2026") |
  | Last heard from | "just now", "12 min ago", "3 h ago", a date, or "Not heard from since it was paired" |
  | State | a chip: **In service**, **Quiet** (not heard from for a day), **Revoked** |
  | | **...**: Rename, and Revoke; none for a revoked device, and no Revoke on this device |

- **Waiting to be paired**, a card, only when there are any: each code made
  and not yet used: the name it will have, its kind, and when it runs out.
  The code itself is not shown again.
- With no device but this one: "Pair a till, a kitchen display or another
  back office to start."

**The forms** (modals):

- **Pair a device:** **Name** ("What the device will be called: Counter 1,
  Pass, Office."); **What it is**, Till, Kitchen display or Back office;
  **Make a code**. The code replaces the form: large, in the numeral face,
  in the groups it is read in (`7K3M-9QX2`), with "Enter this on the new
  device. It works once and runs out at 17:42." Closing the modal does not
  cancel the code.
- **Rename:** one field. "Its label on every screen and in the history. It
  keeps working."
- **Revoke** asks first: "It stops working at once and shows the pairing
  screen. Orders it took stay as they are. To use it again it needs a new
  code." **Revoke device** (danger) and Cancel.

## Data

- `GET /api/v1/devices`, key `["devices"]`: `devices[]` (id, name, kind,
  paired_at, last_seen_at, revoked_at, this) and `waiting[]` (name, kind,
  expires_at). Read again every 30 seconds, since a device is "last seen" at
  most once a minute, and after every change.
- `POST /api/v1/devices/codes` `{name, kind}`: answers `{code, expires_at}`,
  the one time the code is shown. Only its hash is kept.
- `POST /api/v1/devices/{id}/rename` `{name}` and `POST
  /api/v1/devices/{id}/revoke`: each answers with the list as it now stands.

## Interactions

| Action | Does | Then |
|---|---|---|
| Pair a device, Make a code | Creates a code for the name and kind | The code is shown; the device waits |
| Rename, Save | Changes the device's name | A toast, "Saved."; the device is unaffected |
| Revoke, confirm | Unpairs the device | "Counter 1 revoked."; its next request is refused and it shows Pair |

## States

- **Loading:** "Loading the devices". **Could not load:** the reason, with
  **Try again**.
- **Refused:** the server's words over the table ("That device has been
  revoked, so it cannot be renamed."), or under the field in a form ("a name
  of 1 to 64 characters is needed").
- **Not allowed:** the frame's "That is not yours to open".

## Constraints

- No green primary on the list; Make a code is the modal's own. Revoke is
  the danger button, always confirmed.
- A state is a chip with a word. "Quiet", not "Offline": the server knows
  when a device last asked for something, not whether it is switched on.
- A revoked device stays on the list by the name its history knows it by,
  with nothing left to do to it.
- You cannot revoke the device you are using: revoke it from another back
  office.
- Revoking does not close a device's open live stream; it is refused at its
  next connect, and every request it makes is refused at once.
- A pairing code lasts 10 minutes and works once.

## Reference

The design draws no devices screen: D22 lists "Printers & devices" as a
section of Settings and draws nothing under it. This one is ours, kept under
rule 4, on the table pattern of D19 and D20 (ADR-195). D24's setup wizard,
which will pair the first devices, is phase 12's.
