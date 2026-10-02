# Back office: Audit

Every void, comp, discount, refund, payment void and reopen: who did it,
who let it through, why, and what it was worth. Built in phase 10 Task 4d
over the server of Task 4c (ADR-117). The phase asks for it not to be
buried, so it has its own entry in the navigation.

## Purpose

Answer "who has been voiding coffees?" and "how much went out in comps
last week?" in one look, from the record itself.

## Layout

Paper, inside the back office shell, section **Audit**, path `/audit`,
under Reports in the navigation, `reports.view`, keys `g a`.

- **Header:** "Audit" in Instrument Serif; under it, one line in `fg-3`:
  "Every correction to a bill, from the record. Amounts are what the bill
  itself used."
- **Filters,** one row that wraps:
  - **From** and **To**: trading days (`<input type="date">`), the last
    seven by default, today included. A trading day runs 04:00 to 04:00.
  - **Person**: Anyone, or someone on the staff; it finds what they did
    and what they authorised.
  - The filters apply as they change; the page goes back to the newest.
  - The screen opens on the filter in its address when it has one:
    `/audit?from=&to=&staff=&kind=`, as the Sales reports' corrections
    link to it (phase 10 Task 5e2). Anything in the address that is not
    a day, or not a kind, is ignored for the default. Changing a filter
    afterwards does not rewrite the address. A person in the address
    who is not on the Person list (deactivated since) shows there as
    "Someone no longer active", so the trail never looks unfiltered
    when it is filtered.
- **Counts:** one chip per kind, the count in mono: "Voids 12", "Comps
  3", "Discounts 5", "Refunds 1", "Payment voids 0", "Reopens 2". A chip
  is a toggle (`aria-pressed`): pressed, the trail shows that kind only;
  pressed again, every kind. The counts cover the whole range, not the
  page.
- **The trail:** a dense table, newest first:

  | Column | Shows |
  |---|---|
  | When | the day and time, mono |
  | What | the kind in words; the item and quantity under it ("Chai × 1"), or "Whole bill" for a bill discount |
  | Amount | `Money`, mono, right-aligned; empty for a reopen |
  | By | who did it |
  | Authorised by | who let it through; "Not recorded" muted for a discount or refund from before ADR-117; "Themselves" when the same person |
  | Why | the reason, in words where it is a code ("wrong_item" reads "Wrong item") |
  | Bill | the order's id, mono |

- **Show older** under the trail loads the next page.

## States

- **Nothing in the range:** "No corrections between 24 Sept and 30 Sept."
- **Loading / offline / refused:** the words in place of the trail, with
  Try again; a refused filter (a range over a year, To before From) says
  the server's words under the filters.

## Data

- `GET /api/v1/audit?from=&to=&staff=&kind=&before=&limit=50`, an infinite
  query keyed by the filters; `GET /api/v1/staff` for the people to choose
  from (the active staff: anyone signed in may read it, and the trail's
  reader may not have `staff.manage`).

## Constraints

- No accent on this screen: nothing on it is an action that moves money.
- Every number mono, every amount right-aligned.
- A deactivated person is not in the Person list, but their rows are in
  the trail by name.

## Reference

Built from `docs/design/components.md` and the paper tokens. The archive
was not consulted.
