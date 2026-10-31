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
- **Ordering from phones**, a card over the list (phase 11, Task 2b,
  ADR-205; `TunnelCard.tsx`, `tunnel-logic.ts`, `use-tunnel.ts`): whether a
  guest's phone can reach the venue. Its title in the serif, a chip at the
  right, and under them:
  - a sentence saying what the state means for guests, and for the two states
    waiting will not mend, what to do;
  - **Trying again in 20 s** (or "in 4 min", or "now"), when an attempt is due;
  - "What it said:" and the client's own account of the last failure, small
    and muted, for whoever is helping the owner.

  | State | Chip | The sentence |
  |---|---|---|
  | `off` | **Not set up**, neutral | "Guests order at the counter. Ordering from a phone needs a relay address and key in this venue's settings file." |
  | `connecting` | **Connecting**, info | "Reaching the relay. Guests cannot order from their phones until it answers." |
  | `connected` | **Connected**, success | "Guests can order from their phones. Connected since 14:02." |
  | `waiting` | **Reconnecting**, warning | "The relay cannot be reached, so guests cannot order from their phones. The counter is not affected." |
  | `refused` | **Key refused**, danger | "The relay does not accept this venue's key, ... Check the relay key in the settings file, or ask whoever runs the relay for a new one." |
  | `not_entitled` | **Not carried**, danger | "The relay is not carrying this venue, so guests cannot order from their phones." then why, from the relay's code ("The last payment for mez Cloud did not go through.", "The free trial has ended.", "Whoever runs the relay has not turned it on for this venue.", or "Whoever runs the relay can say why."), then "mez asks again every minute and comes back by itself once it is. The counter is not affected." |
  | `replaced` | **In use elsewhere**, danger | "Another computer is connected with this venue's key, and guests reach that one. Only one can be: stop the other, or give this one a key of its own." |
  | `stopped` | **Stopped**, neutral | "The link to the relay is stopped. It starts again when mez does." |

  Under the sentence, when the relay has said where the venue stands with
  it (phase 14, Task 3b, ADR-316; `cloudLine`), one more line in the same
  face: "Cloud: paid until 12 Nov 2026.", "Cloud: free trial until ...",
  "Cloud: payment failed, 5 days left." (the days are to when the relay
  stops, not to what was paid), "Cloud: cancelled. Carried until ...",
  "Cloud: stopped. It was paid until ...". "Cloud" is said only of a relay
  that keeps accounts and so gives a state; one that keeps a file gives a
  date at most, and reads "The relay carries this venue until 19 Nov
  2026.", and nothing at all with no date.

- **Back office from anywhere**, a card under that one (phase 13, Task 4,
  ADR-310; `RemoteCard.tsx`, `remote-logic.ts`): whether the back office can
  be opened away from the venue, and where. Its title in the serif, a chip
  at the right, a sentence, the address when there is one (in the numeral
  face, a link, with **Copy**), and a numbered list of what to do. It is not
  a switch: the setting is in the settings file and read when mez starts,
  and the card says how to turn it on. It reads the same request as the card
  above and shows nothing while that loads or fails.

  | Case | Chip | The sentence | The list |
  |---|---|---|---|
  | No relay | **Only at the venue**, neutral | "The back office opens only at the venue. Opening it from a phone or a computer elsewhere needs a relay first, which is also what lets guests order from their phones." | none |
  | A relay, and off | **Only at the venue**, neutral | "The back office opens only at the venue. It can be opened from a phone or a computer elsewhere too: the menu, tables, stock, staff and reports, and nothing that takes money." | a key that allows it; `relay_key` and `remote_backoffice = true`; restart |
  | On, relay reached | **Open**, success | "The back office can be opened away from the venue, at this address." | open it; pair with a code made here, as a Back office; sign in, managers only |
  | On, relay not reached | **Not reachable**, warning | "It is turned on, but the relay is not reached right now, so the address opens nothing until it is. The counter is not affected." | the same three |
  | On, relay reached, and the relay does not carry the back office for this venue | **Not reachable**, warning | "It is turned on here, but the relay does not carry the back office for this venue, so the address opens nothing. Guests' orders are not affected. Whoever runs the relay can turn it on." | none |
  | On, no address | **Open** or **Not reachable** | "It is turned on, but this venue has not said where it is reached, so there is no address to open." | set `public_url` and restart |
  | This page is the one opened from away | as above | "You are using it now: this is the back office, opened away from the venue." | none |

  At the card's foot, over a soft rule, **mez Cloud** (phase 14, Task 4c1,
  ADR-321; `CloudPairModal.tsx`, `cloud-logic.ts`): a line and one
  secondary button.

  | The venue | The line | The button |
  |---|---|---|
  | Not paired | "With a mez Cloud account, a code from it sets all of this up, and a backup with it." | **Connect to mez Cloud** |
  | Paired | "Paired to Priya Menon's mez Cloud account since 12 Oct 2026." | **Pair again** |
  | Opened from away, paired | the same line | none: a code is typed at the venue |
  | Opened from away, not paired | nothing | none |

  Under it, for a paired venue whose cloud sends the owner's WhatsApp
  (phase 15, Task 6d, ADR-364), one line and a link, read only, at the
  venue and from away alike: "Last report: yesterday 23:04." ("today",
  "yesterday", or the date with the time), or "No close-of-day report has
  gone yet."; and **Change what is sent**, which opens the venue's page in
  the mez Cloud account area in a new tab. A relay that says nothing of
  reports shows neither. The link is shown only when it is https.

  The button opens **Connect to mez Cloud**, a modal: two sentences on what
  pairing does ("The till works the same with or without it."), one field,
  **Pairing code**, in the numeral face and capitals, with "From your mez
  Cloud account, under this venue. It works once, for an hour." under it,
  and **Cancel** and **Connect** (the modal's own primary). A code that
  could not be one is refused under the field before anything is sent
  ("A pairing code is 8 letters and digits, with no 0, O, 1 or I."); what
  the server refuses is said in its words, under the field when it is
  about the code and over it when it is not. While it works the button
  reads **Connecting** and the hint says it can take up to twenty seconds.
  Then the modal becomes **Connected to mez Cloud**: who the venue is now
  paired to, a line for the backup ("A backup of the venue is now kept
  with mez Cloud...", "Your backup is left where it was...", or that it
  could not be started and the venue is paired all the same), "Restart mez
  to finish" when part of it waits for a start, the address the back
  office opens at from away, and **Done**. Both cards above have by then
  read the server again and say what is now so.

- **Away from the venue** (ADR-309) the head has no **Pair a device** but
  "A device is paired with a code made at the venue.", and a row's **...**
  has Revoke and no Rename.

- A line over the table for what an action was refused for.
- **The table**, in a card: those in service first, in the order paired,
  the revoked at the foot.

  | Column | Shows |
  |---|---|
  | Device | its name, "This device" beside the one the back office is running on, and its kind in words under it: **Terminal**, **Kitchen display**, **Back office** |
  | Paired | the day ("3 Oct 2026") |
  | Last heard from | "just now", "12 min ago", "3 h ago", a date, or "Not heard from since it was paired" |
  | State | a chip: **In service**, **Quiet** (not heard from for a day), **Revoked** |
  | | **...**: Rename, and Revoke; none for a revoked device, and no Revoke on this device |

- **The device called "mez Cloud"** (ADR-323, ADR-325; `cloud: true`), on
  a venue paired to a mez Cloud account: what the owner's sign-ins from
  their account are counted as coming from. Nobody holds it.
  - A cloud for its mark, and under the name, in place of its kind: "Your
    mez Cloud account's way in. Nobody holds it".
  - Its **...** has **Revoke** and no **Rename**.
  - **Revoke** asks in other words: "The back office can no longer be opened
    from your mez Cloud account, and anyone signed in that way is signed out
    at once. Guests' orders and the backup carry on. Pairing to mez Cloud
    again, here at the venue, turns it back on." The button reads **Shut mez
    Cloud's sign-in**, and the toast "mez Cloud's sign-in is shut."
  - Pairing again adds a new one; the revoked one stays at the foot.

- **Waiting to be paired**, a card, only when there are any: each code made
  and not yet used: the name it will have, its kind, and when it runs out.
  The code itself is not shown again. From 1280px it is a column beside the
  table at the back office's side width (`w-aside`, 307 to 380px; TECH-17);
  under that, under the table.
- With no device but this one: "Pair a terminal, a kitchen display or
  another back office to start."

**The forms** (modals):

- **Pair a device:** **Name** ("What the device will be called: Counter 1,
  Pass, Office."); **What it is**, Terminal, Kitchen display or Back office;
  **Make a code**. The code replaces the form (ADR-367). First, where to
  open mez on the new device: "On the new device, scan this with its
  camera, or open the address in its browser. Both must be on this
  network.", then a band for each address on a real adapter (the QR code,
  on white, 132px, and the address in the numeral face without its scheme,
  `192.168.1.10:7777/kitchen`; "On Wi-Fi" over it when there is more than
  one), and "Also at ..." small for virtual adapters' addresses. When the
  server knows no address: "On the new device, open this computer's
  address in a browser, with :7777 on the end..." Under that, **Pairing
  code** and the code, large, in the numeral face, in the groups it is read
  in (`7K3M-9QX2`), with "Scanned, it is typed in by itself; opened by
  address, type it in. It works once and runs out at 17:42." The QR code
  is the address with `#code=7K3M-9QX2` on it: the pairing screen reads the
  code out of the address, types it in and tries it. Closing the modal
  does not cancel the code.
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
- `POST /api/v1/devices/codes` `{name, kind}`: answers `{code, expires_at,
  open[]}`, the one time the code is shown; `open` is each address a device
  of that kind opens mez at (`adapter`, `virtual`, `url`, `svg`: the QR code
  of the address with the code in its fragment), real adapters first, and
  empty when the server knows none (ADR-367); `named_url` is the kind's
  address by the server's network name, `mez.local` (ADR-379), or absent.
  Only the code's hash is kept.
- `POST /api/v1/devices/{id}/rename` `{name}` and `POST
  /api/v1/devices/{id}/revoke`: each answers with the list as it now stands.

- `GET /api/v1/tunnel`, key `["tunnel"]`, `device.manage`: `state`, and when
  there is one `since`, `retry_at` and `detail`; `back_office`, always, and
  `back_office_url` when it is on and the venue's `public_url` is a hostname
  with nothing after it. Read again every 5 seconds
  while connecting or reconnecting, every 30 once settled, and every 5
  minutes when off, which cannot change under an open page.

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

- **Ordering from phones** loads and fails on its own: "Loading", or the
  reason with **Try again**, in its card. The list under it is not held up.

- **On a mirror** (ADR-330): "This is a copy to read. Devices are paired
  and unpaired at the venue." is where Pair a device was, no row has a
  menu, and "Ordering from phones" and "Back office from anywhere" are
  left out: both would be read off the mirror and said of the venue.

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

- The card for ordering from phones changes nothing. The relay's address
  and key are in the venue's settings file and are read at start. It never
  shows the key. A venue with no relay is "Not set up", never a fault.

## Reference

The design draws no devices screen: D22 lists "Printers & devices" as a
section of Settings and draws nothing under it. This one is ours, kept under
rule 4, on the table pattern of D19 and D20 (ADR-195). D24's setup wizard,
which will pair the first devices, is phase 12's.
