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
beside it the roles.

- **The head:** "Staff & roles" in the serif, and "9 people · 2
  deactivated". At the right, **Add a role** and **Add staff**
  (terracotta).
- **The staff table:**

  | Column | Shows |
  |---|---|
  | Person | initials in a round mark, the name, "(you)" after your own, and "Since 2026-09-12" under it |
  | Role | the role's name |
  | State | a chip: **Active**, or **Deactivated** |
  | | **...**: Edit, New PIN, Deactivate, or Bring back for someone deactivated |

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

- **Add staff, Edit:** Name; Role (a select); on Add, a **PIN**, 4 to 8
  digits, and **The PIN again**.
- **New PIN:** the PIN twice, and "They will be signed out of every terminal and
  back office now."
- **Add a role:** Name, and the permissions as on the panel.
- **Deactivate** asks first: "They leave the sign-in screen and are signed
  out now. Their name stays on everything they did. You can bring them
  back."

## What you may give is what you hold (ADR-115)

- A switch for a permission you do not hold is shown as it is and cannot be
  moved: "You don't have the greyed ones, so you can't give them."
- A role that can do things you cannot is shown and not yours to change:
  "This role can do things you cannot, so it is not yours to change."
- A role you could not give is listed for a person as "Waiter (not yours
  to give)", and cannot be chosen.
- The screen mirrors the server's rule; the server decides.

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

## States

- **Loading:** "Reading the staff" under the title; "Loading the people".
- **Error:** the server's words, with **Try again**.
- **Nobody:** "Nobody is on the staff list."
- **Refused:** the server's words under the field or over the form: a role
  name taken; a PIN not 4 to 8 digits; "That reaches permissions you don't
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
