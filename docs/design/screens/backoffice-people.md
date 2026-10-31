# Back office: Staff and roles

Who works here, what each may do, and their PINs. Built in phase 10 (ADR-115)
and rebuilt from D20 in the rebuild's Task 16h (ADR-192).
`web/backoffice/src/people/`: `PeopleScreen.tsx`, `RolePanel.tsx`,
`PeopleForms.tsx`, `people-logic.ts`, `use-people.ts`. Section **Staff**, path
`/people`, `staff.manage`, keys `g p`.

## Purpose

Add the new waiter before their first shift, take away a PIN the moment
someone leaves, and see in one place what each role is trusted with.

## Layout

Inside the frame (`backoffice-shell.md`). The staff table in a card, and
beside it the roles: from 1280px a column at the back office's side width
(`w-aside`, 307px at 1280, 328px at 1366, 380px from 1584; TECH-17), the
table the rest; under 1280px the roles sit under the table.

- **The head:** "Staff", the nav's name for it, in the serif (BO-17,
  ADR-404; D20 said "Staff & roles"), and "9 people · 2
  deactivated". At the right, **Add a role** and **Add staff**
  (terracotta).
- **The staff table:** under the card's top, "Pick a person to show their
  role's permissions.", which each person's button is described by; a
  chevron ends each person, green on the one whose role is shown, which is
  `aria-current` (BO-12, ADR-404).

  | Column | Shows |
  |---|---|
  | Person | initials in a round mark, the name, "(you)" after your own, and "Since 12 Sep 2026" under it, on the venue's clock (BO-4) |
  | Role | the role's name |
  | State | a chip: **Active**, **Locked** (and under it "Too many wrong PINs, 12 min ago"), or **Deactivated** |
  | | **...**: Unlock for someone locked, Edit, New PIN, Deactivate, or Bring back for someone deactivated |

  A row picks its person, and the panel beside shows their role. "Show the 2
  deactivated" under the table adds them. Your own row has no Deactivate.
- **The roles**, beside it:
  - Each role as a pill with how many people hold it; the chosen one
    pressed.
  - **Permissions · Waiter**, and "Applies to Rohan and Arjun."
  - **Role name**, a field.
  - **Every permission as a switch that says On or Off**, in five groups:
    - **Selling:** Take orders; Change what is on an order; Take payment;
      See the kitchen board.
    - **Kitchen:** Start tickets and mark them ready. "A kitchen display on
      the wall needs neither: it is paired, not signed in."
    - **Corrections**, "Others ask for these on the order screen.": Void items;
      Comp items; Give discounts; Void payments; Give refunds; Reopen
      closed bills; Reprint invoices.
    - **Shift and cash:** Open the shift; Close the shift; Move cash in and
      out of the drawer.
    - **Back office:** Edit the menu; Keep the stock; Manage staff and
      roles; See reports; Change venue settings; Pair and remove devices.
  - **Reset** and **Save role** (the primary); and **Delete this role**, in danger, for a
    role nobody holds.

**The forms** (modals):

- **Add staff, Edit:** Name; Role (a select, each role by its name as
  people read it, "Manager"; on Add it starts on "Choose a role" and is
  refused without one, "Choose their role.": BO-13, ADR-404); on Add, a **PIN**, 4 to 8
  digits, or 6 to 8 for a role that manages staff, devices or settings, and
  **The PIN again**. On Edit, moving someone to a role that needs the longer
  PIN says "This role needs a PIN of 6 digits or more. Ask them to set a
  6-digit PIN."
- **New PIN:** the PIN twice, with the length the person's role needs, and
  "They will be signed out of every terminal and back office now."
- **Add a role:** Name, and the permissions as on the panel.
- **Deactivate** asks first: "They leave the sign-in screen and are signed
  out now. Their name stays on everything they did. You can bring them
  back."
- **Unlock** asks first: "Their PIN was typed wrong too many times, so they
  were signed out and cannot sign in. Unlocked, they are back on the sign-in
  screen with the PIN they have. If it was not them typing, give them a new
  PIN as well."

## What you may give is what you hold (ADR-115)

- A switch for a permission you do not hold is shown as it is and cannot be
  moved: "You don't have the greyed ones, so you can't give them."
- A role that can do things you cannot is shown and not yours to change:
  "This role can do things you cannot, so it is not yours to change."
- A role you could not give is listed for a person as "Waiter (not yours
  to give)", and cannot be chosen.
- The screen mirrors the server's rule; the server decides.

## A PIN that keeps failing (ADR-260)

Five wrong PINs in a quarter hour lock a person for the rest of it. The third
such lockout in a day, at sign-in or at a manager's override, locks them until
someone who can manage staff unlocks them here. Locked, they leave the
sign-in tiles and are signed out everywhere; a PIN typed for them anyway is
refused as a wrong one, whether it is right or not, so the person at the
keypad learns nothing from it. Their row says **Locked**, why and when, and
its menu offers **Unlock**: "Kabir is unlocked." Nobody unlocks themselves.
Unlocked, the right PIN works at once; if the wrong PINs were not theirs,
give them a new one too. A role that manages staff, devices or settings
needs a PIN of six digits or more.

## The person mez Cloud signs in as (ADR-323, ADR-325)

A venue paired to a mez Cloud account has one person the pairing made,
named for the account, in the owner's role: who the account's owner is here
when they come in from mez Cloud. `cloud: true` on the person.

- Under the name: "Signs in from mez Cloud · Since 12 Oct 2026".
- The row's **...** has no **New PIN**. They have none and the server gives
  none; at a till the owner signs in as themselves.
- **Edit** says first: "This is who your mez Cloud account signs in as. They
  have no PIN. Pairing to mez Cloud again puts back the account's name and
  the owner's role." Moving them to another role is allowed, and is how a
  venue limits what its account may do from away.
- **Deactivate** asks in other words: "This is who your mez Cloud account
  signs in as. Deactivated, the back office can no longer be opened from
  mez Cloud, and anyone signed in that way is signed out now. Guests' orders
  and the backup are not affected. You can bring them back, and pairing to
  mez Cloud again does too."
- They are not on any sign-in screen, and are not counted when the server
  asks whether anyone is left who can manage staff.

## Data

- `GET /api/v1/people`, key `["people"]`: the staff and the roles in one
  read.
- The writes of ADR-115; each reads the list again. Your own permissions are
  the session's.
- A PIN is kept as a hash: none of it can be shown, only replaced.

## Interactions

| Action | Does | Then |
|---|---|---|
| A row, or a role's pill | Picks the role shown | An unsaved change to the last role is dropped |
| A switch | Changes the draft | Save role is offered |
| Reset | Puts the draft back as saved | |
| Save role | Saves the name and what it grants | A toast: "Saved. It applies to everyone with this role from their next step." |
| Delete this role | Removes a role nobody holds | "Deleted the Host role." |
| Add staff | Adds the person | "Added Kabir." They are on the sign-in screen at once |
| New PIN | Replaces it | "Kabir has a new PIN and is signed out." |
| Deactivate, Bring back | | "Kabir is deactivated." |
| Unlock | Clears a hard lock | "Kabir is unlocked." They are on the sign-in screen again |

## States

- **Loading:** "Reading the staff" under the title; "Loading the people".
- **Error:** the server's words, with **Try again**.
- **Nobody:** "Nobody is on the staff list."
- **Refused:** the server's words under the field or over the form: a role
  name taken; a PIN not 4 to 8 digits; "A PIN for a role that manages
  staff, devices or settings is 6 to 8 digits."; "You can't clear your own
  lock"; "That reaches permissions you don't
  have"; "Nobody would be left who can manage staff"; a role people still
  hold.
- **A role only deactivated people hold:** says so. It cannot be deleted,
  and the table hides them until asked.
- **After changing your own role:** the switches follow your old
  permissions until you sign in again.

## Constraints

- One primary: Save role. Add staff is terracotta.
- A switch says On or Off in words, not by its colour alone.
- A person is never deleted: their name stays on every order, payment and
  count they made.
- The last person who can manage staff cannot lose that, by any route.
- Permissions changed here reach a signed-in person at their next request
  to the server, and the screens they see at their next sign-in.

## On a mirror

Read and not changed (ADR-330). "This is a copy to read. Changes are made
at the venue." is where Add a role and Add staff were, and again where the
role panel's Reset and Save role were. No row has a menu. A person still
picks their role, and a role's pill still shows it: its name and every
switch are disabled, and the notes about what is not yours to give are not
shown.

## Reference

`mez Desktop Management.dc.html`, D20 Staff Roles (line 507). Left out, and
why:

- **Roster, the Shift column, and "4 on shift now":** there are no
  timesheets in v1.
- **The PIN column:** a PIN is kept as a hash. A new one is set from the
  row's menu.
- **A phone number.**

Changed: the design lists eight switches. A role here grants any of the
server's twenty-one permissions, so they are all listed, in their groups.
Its note, "Anything switched off asks for a manager PIN instead of being
hidden", is true of the corrections, and sits with that group as "Others ask
for these on the order screen."

Kept, though not drawn (rule 4): Add a role, the role's name, Delete this
role, and
Deactivate with Bring back.
