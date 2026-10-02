# Back office: People

Who works here, what each role may do, and their PINs. Built in phase 10
Task 4b over the server of Task 4a (ADR-115). The audit trail of who
voided, comped, discounted and refunded is its own section (Task 4d).

## Purpose

Add the new barista before their first shift, give the closing manager
the till, take a leaver off the sign-in screen, and reset a forgotten PIN,
without being able to hand out more than you hold.

## Layout

Paper, inside the back office shell, section **People**, path `/people`,
`staff.manage`.

- **Header row:** "People" in the serif; **Add a person** (primary)
  on the Staff view, **Add a role** (primary) on the Roles view.
- **A switch, Staff and Roles** (`aria-pressed`), as Menu and Stock have.

### Staff

A table, the active first, by name:

| Column | Shows |
|---|---|
| Name | the name; "(you)" after your own; "Deactivated" (struck, muted) |
| Role | the role's name |
| Since | the date they were added, tabular |
| | **Edit**, **New PIN**, **Deactivate** or **Bring back** (48px) |

- An action the server would refuse because their role holds what yours
  does not is still shown; the refusal says why. You cannot deactivate
  yourself: your row has no Deactivate.
- **Add / Edit** (`Modal`, sm): Name; Role (a select of the roles, with
  any you could not give marked "(not yours to give)" and disabled); on
  Add, a PIN, 4 to 8 digits, typed twice. Save.
- **New PIN** (`Modal`, sm): the PIN twice, and the line "They will be
  signed out of every till and back office now." Save.
- **Deactivate** asks first (`Confirm`): "Deactivate Asha? They leave the
  sign-in screen and are signed out now. Their name stays on everything
  they did. You can bring them back."

### Roles

One `Card` per role, by name: the name, how many active people hold it
(tabular), and its permissions in words, grouped; **Edit**, and **Delete**
when nobody holds it.

- **The role form** (`Modal`, md): Name, then every permission as a
  checkbox in words, in four groups:
  - **Selling:** Take orders; Change orders on the ticket; Take payment;
    Use the kitchen display.
  - **Corrections**, which others ask for at the till: Void items; Comp
    items; Give discounts; Void payments; Give refunds; Reopen closed
    bills; Reprint invoices.
  - **Shift and cash:** Open and close the shift; Move cash in and out of
    the drawer.
  - **Back office:** Edit the menu; Keep the stock; Manage staff and
    roles; See reports; Change venue settings; Pair and remove devices.
- A permission you do not hold is shown, unchecked or checked as it is,
  and disabled, with "You don't have this, so you can't give it." under
  the group. It mirrors the server's rule (ADR-115); the server decides.

## Refusals

The server's words, over the form or under the field: a role name taken;
a PIN not 4 to 8 digits; "That reaches permissions you don't have";
"Nobody would be left who can manage staff"; a role people still hold.

## Data

- `GET /api/v1/people` (`["people"]`): staff and roles in one read.
- The writes of ADR-115; each reads the list again. Your own permissions
  are the session's (ADR-105).

## States

- **Loading / offline / refused:** the words in place of the list, with
  Try again.
- **Saved:** a toast: "Added Kiran." / "Asha has a new PIN and is signed
  out." / "Saved."

## Constraints

- One primary per view: its Add button.
- Dates tabular. A PIN field is `inputMode="numeric"`, `type="password"`,
  `autoComplete="new-password"`, never shown back.

## Reference

Built from `docs/design/components.md` and the paper tokens. The archive
was not consulted.
