# Back office: Devices

Every till, kitchen display and back office paired to the venue: who they are,
when each was last heard from, and the three things an owner does with them:
pair another, rename one, and unpair one that is lost, sold or broken. Built in
phase 10 Task 6k (ADR-140).

## Purpose

Know what is connected to the venue, and take a device off it in one step when
it should not be, without anyone opening a database.

## Layout

Paper surface, inside the back office shell, reached from **Devices**
(`/devices`, key `g e`; needs `device.manage`). Content is `max-w-4xl`.

- **Header row:** the title "Devices" in the serif, and **Pair a device** on the
  right, the one primary button.
- **In service**, a `Card`: one row per device that is not revoked, in the order
  they were paired:
  - its name (semibold). For the device the back office is running on, "This
    device" beside it, in words.
  - its kind in words, never the code: **Till**, **Kitchen display**,
    **Back office**; when it was paired ("Paired 3 Oct 2026").
  - **When it was last heard from**: "Seen just now", "Seen 12 min ago", "Seen 3 h
    ago", "Seen 2 Oct 2026", or "Not heard from since it was paired". A device
    not heard from for a day is shown "Seen 2 Oct 2026" in the warning ink, with
    the word, not the colour alone.
  - **Rename** and **Revoke** (danger), 48px. On "This device" Revoke is shown
    disabled with the reason beside it: "You are using it. Revoke it from another
    back office."
- **Waiting to be paired**, a `Card`, only when there are any: a line for each
  code made and not yet used: the name it will have, its kind, and when it runs
  out ("runs out in 7 min"). The code itself is not shown again: it was shown
  once, when it was made.
- **Revoked**, a `Card`, only when there are any, below the others and muted: the
  name struck through, its kind, and "Revoked 2 Oct 2026". No actions: a revoked
  device keeps the name its history knows it by.

### Pair a device (`Modal`)

- **Name**: what the device will be called ("Counter 1", "Pass", "Office").
- **What it is**, a choice: Till, Kitchen display, Back office.
- **Make a code** (primary). The code replaces the form in the same modal: large,
  in the numeral face, in the groups it is read in (`7K3M-9QX2`), with "Enter this
  on the new device. It works once and runs out at 17:42." and **Done**. Closing
  the modal does not cancel the code: it is listed under Waiting until used or
  expired.
- A refusal shows under its field: "a name of 1 to 64 characters is needed".

### Rename (`Modal`)

One field, the name, with Save. The device keeps working: a rename changes its
label on every screen and in the history, not its pairing.

### Revoke (`Confirm`)

"Revoke Counter 1?" "It stops working at once and shows the pairing screen. Orders
it took stay as they are. To use it again it needs a new code." **Revoke device**
(danger) and Cancel.

## Data

- `GET /api/v1/devices` (`device.manage`): `devices[]` (`id`, `name`, `kind`,
  `paired_at`, `last_seen_at`?, `revoked_at`?, `this`) and `waiting[]` (`name`,
  `kind`, `expires_at`). React Query key `["devices"]`, read again every 30
  seconds, since a device is "last seen" at most once a minute, and after every
  change.
- `POST /api/v1/devices/codes` `{ name, kind }`: answers `{ code, expires_at }`,
  the one time the code is shown.
- `POST /api/v1/devices/{id}/rename` `{ name }` and `POST
  /api/v1/devices/{id}/revoke`: each answers with the list as it now stands.
- All need a signed-in staff member holding `device.manage`.

## Interactions

| Action | Does | Then |
|---|---|---|
| Pair a device, Make a code | Creates a code for the name and kind | The code is shown; the device waits |
| Rename, Save | Changes the device's name | The list shows it; the device is unaffected |
| Revoke, confirm | Unpairs the device | It moves to Revoked; its next request is refused as revoked |
| Revoke on this device | Nothing | Disabled, with the reason |

## States

- **Loading:** "Loading the devices". **Could not load:** the reason, with **Try
  again**.
- **No devices but this one:** the one row, and "Pair a till, a kitchen display or
  another back office to start." under it.
- **Refused:** the server's words in a line under the card heading for a revoke or
  a rename that failed ("That device has been revoked, so it cannot be renamed.",
  "You are using this device: revoke it from another back office, or leave it."),
  and under the field in a form.
- **Not allowed:** the shell's "That is not yours to open", with `device.manage`
  in the numeral face.

## Constraints

- One primary: **Pair a device**. Revoke is the danger button, always confirmed.
- Every control 48px; nothing under 13px; the pairing code large enough to read
  across a counter.
- Status is words: "This device", "Revoked", "Not heard from", never colour alone.
- A pairing code is a secret while it lives: the screen shows it once, and never
  puts it in the list or the address.

## Reference

The archive's Devices screen (`mez Desktop Management`, "Devices", print routing)
lists printers and tablets with a state dot. Printers are their own screen here
(`backoffice-printers.md`); this one is the paired devices, a different thing:
what is allowed to talk to the venue's server.
