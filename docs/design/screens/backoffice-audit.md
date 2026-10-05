# Back office: Audit

Every void, comp, discount, refund, payment void and reopen: who did it, who
let it through, why, and what it was worth. Built in phase 10 (ADR-117) and
given the reports' head in the rebuild's Task 16k (ADR-195).
`web/backoffice/src/audit/AuditScreen.tsx`, `audit-logic.ts`; the range pill
is `sales/RangePill.tsx`. Section **Audit**, path `/audit`, `reports.view`,
keys `g a`. It has its own entry in the sidebar so it is never buried.

## Purpose

Answer "who has been voiding coffees?" and "how much went out in comps last
week?" in one look, from the record itself.

## Layout

Inside the frame (`backoffice-shell.md`).

- **The head:** "Audit" in the serif, and under it "Every correction to a
  bill, from the record. Amounts are what the bill itself used." At the
  right, **the range** as one pill, as on the reports: the quick ranges,
  From and To, and "Both are trading days. A trading day runs 04:00 to
  04:00." The last seven days, today included, to start.
- **Person**, a select: Anyone, or someone on the staff. It finds what they
  did and what they authorised.
- **Kinds**, a pill each, with its count in the numeral face: **Voids**,
  **Comps**, **Discounts**, **Refunds**, **Payment voids**, **Reopens**. A
  pill is a toggle: pressed, the trail shows that kind only; pressed again,
  every kind. The counts cover the whole range, not the page.
- **The trail**, a table, newest first:

  | Column | Shows |
  |---|---|
  | When | the day and time, tabular |
  | What | the kind in words; the item and quantity under it ("Chai × 1"), or "Whole bill" for a bill discount |
  | Amount | `Money`, right-aligned; empty for a reopen |
  | By | who did it; "A guest" muted when nobody on the staff did |
  | Authorised by | who let it through; "Themselves" when the same person; "Not recorded" muted for a discount or refund from before ADR-117 |
  | Why | the reason, in words where it is a code ("wrong_item" reads "Wrong item") |
  | Bill | the order's id, in the numeral face |

- **Show older** under the trail loads the next fifty.

## Data

- `GET /api/v1/audit?from=&to=&staff=&kind=&before=&limit=50`, an infinite
  query keyed by the filters, with the counts by kind for the range.
- `GET /api/v1/staff` for the people to choose from: the active staff.
  Anyone signed in may read it, and the trail's reader may not have
  `staff.manage`.

## Interactions

| Action | Does |
|---|---|
| The range, a quick range or a day | Reads the trail for those days; the page goes back to the newest |
| Person | Narrows to what that person did or authorised |
| A kind's pill | Narrows to that kind, or back to every kind |
| Show older | Adds the next page under the rows shown |

The screen opens on the filter in its address when it has one:
`/audit?from=&to=&staff=&kind=`, as the reports' Corrections link to it.
Anything in the address that is not a day, or not a kind, is ignored for the
default. Changing a filter afterwards does not rewrite the address.

## States

- **Loading:** "Reading the record".
- **Nothing in the range:** "No corrections between 24 Sept and 30 Sept.",
  or "No voids between ..." with a kind pressed.
- **Offline or refused:** "Can't reach the mez server. Check this computer's
  connection.", or the server's words for a refused range (over a year, To
  before From), with **Try again**.
- **A person in the address who is not on the list** (deactivated since):
  the select shows "Someone no longer active", so the trail never looks
  unfiltered when it is filtered.

## Constraints

- No primary on this screen: nothing on it moves money, and nothing here
  can be changed. The record is the record.
- Every number tabular, every amount right-aligned.
- A deactivated person is not in the Person list, but their rows are in the
  trail by name.
- The range is this screen's own, not the reports': it is opened on a filter
  from a link, and must not carry that back to Sales.

## Reference

The design draws no audit screen; this one is ours, kept under rule 4, and
laid out on the reports' patterns (D15's title block and range pill) so it
reads as the same app (ADR-195). There is no CSV button here yet: the files
are on Sales reports, under Corrections.
