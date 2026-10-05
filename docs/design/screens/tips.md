# Back office: Tips

A closed shift's tips, pooled and split among the people who worked it. Built
in phase 09 (ADR-101, ADR-102). The rebuild left its layout as it was
(ADR-195). `web/backoffice/src/tips/TipsScreen.tsx`. At `/shifts/:id/tips`,
`reports.view` to read and `shift.manage` to pool; it belongs to the
**Z-reports** section.

## Purpose

After close, a manager decides who shares the night's tips, types in the hours
when the venue splits by hours, sees each share to the paisa before
committing, and pools them once.

## Layout

Inside the frame (`backoffice-shell.md`).

- **Opening it:** **Tips** beside Print a copy on a Z-report in the back
  office. A ghost button at the right leads back to the report.
- **The head:** "Tips" in the serif; under it the rule in words ("Split
  evenly", "Split by hours worked", "Split by role weight") and the trading
  day. Then the pool as the one total, with "In tips" beside it.
- **Not yet pooled**, "Who shares the tips": a table of people, first those
  the server suggests (everyone who took a tip or rang up an order), then the
  rest of the staff:
  - **Name**
  - **In the pool:** a toggle per row, "In" or "Out", a word and a colour.
  - **Hours worked**, only when the rule is by hours: hours and minutes.
  - **Took:** tips on their own payments.
  - **Share:** the server's preview for the people in, redrawn after each
    change; a dash for someone out.
  - Under the table, why there is no preview when there is none ("Enter
    hours for everyone in the pool."). **Pool the tips** (the one primary)
    asks first: "Pool ₹70.00 among 3 people? A pool can't be changed once
    it's made."
- **Pooled**, "The pool": the table read-only: Name, Role, Hours or Weight,
  Took, Share; and under it "Pooled at 01:52. It can't be changed." Someone
  who took a tip and was left out is listed with a share of nothing.

## Data

- `GET /api/v1/shift/{id}/tips`, key `["tips", id]`: the pool, or a preview
  with the suggested members.
- `GET /api/v1/staff`: everyone who can be added.
- `POST /api/v1/shift/{id}/tips/preview` `{members}`: the shares for the
  people in.
- `POST /api/v1/shift/{id}/tips` `{members}`: pools; needs `shift.manage`.
- Members are `{staff_id, minutes}`.
- The rule and each role's weight are set under Settings, Shift, tips and
  receipt (`backoffice-settings.md`).

## Interactions

| Action | Does | Then |
|---|---|---|
| In, Out | Adds a person or takes them out | The shares redraw |
| Hours, minutes | Sets their time | The shares redraw |
| Pool the tips, confirm | Records the pool | A toast, "The tips are pooled."; the read-only table |

## States

- **Loading:** "Loading the tips".
- **No tips:** "No tips were taken in this shift." Pooling a pool of nothing
  is allowed.
- **Refused:** the server's reason in words: "You don't have permission to
  do that. Ask a manager.", "Enter hours for everyone in the pool.", "This
  shift's tips are already pooled."
- **Offline:** "Can't reach the mez server to load the tips." with **Try
  again**.

## Constraints

- No arithmetic: every share is the server's; the screen only turns hours
  and minutes into minutes.
- Every number tabular and right-aligned; every target 48px.
- One primary: Pool the tips.
- A pool is made once per shift and cannot be corrected.
- Hours are typed at pooling: there are no timesheets in v1.

## Reference

The design draws no tips screen. D17's "Tips are paid out weekly" and D20's
roles are the nearest it comes; how a venue shares tips is its own rule, so
the rule is a setting and this screen applies it. Ours, kept under rule 4.
What each person took is also on Staff reports and on the waiter's shift
summary (`backoffice-reports.md`, `waiter-shift.md`).
