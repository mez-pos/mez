# Back office: Tips

A closed shift's tips, pooled and split among the people who worked it
(phase 09, Task 4b; ADR-101), and each person's tip report.

## Purpose

After close, a manager decides who shares the night's tips, types in the
hours when the venue splits by hours, sees each share to the paisa before
committing, and pools them once. Staff see what they took and what they
get.

## Layout

- **Opening it:** **Tips** beside Print a copy on a Z-report in the back
  office. The page is `/shifts/{id}/tips`, with **The Z-report** back.
- **Header:** "Tips" in Instrument Serif; Meta: the rule in words ("Split
  evenly", "Split by hours worked", "Split by role weight") and the
  trading day. Then the pool as the one total: "Rs. 70.00 in tips".
- **Not yet pooled:** a table of people, first those the server suggests
  (everyone who took a tip or rang up an order), then the rest of the
  staff:
  - **In the pool**: a toggle button per row ("In" / "Out", word and
    colour).
  - **Hours**, only when the rule is by hours: hours and minutes fields.
  - **Role** and its **weight**, only when the rule is by role weight.
  - **Took**: tips on their own payments, in mono.
  - **Share**: the server's preview for the people in, redrawn after each
    change; "–" for someone out.
  - Under the table, why there is no preview when there is none ("Enter
    hours for everyone in the pool."). **Pool the tips** (the one accent),
    asking first: "Pool Rs. 70.00 among 3 people? A pool can't be changed
    once it's made."
- **Pooled:** the same table read-only, with who was in, their hours or
  weight, what they took and their share; a line under it: "Pooled at
  01:52. It can't be changed." Someone who took a tip and was left out is
  listed with a share of nothing.

## Data

- `GET /api/v1/shift/{id}/tips`, key `["tips", id]`: the pool, or a
  preview with the suggested members.
- `GET /api/v1/staff`: everyone who can be added.
- `POST /api/v1/shift/{id}/tips/preview` `{members}`, key
  `["tips", id, "preview", members]`: the shares for the people in.
- `POST /api/v1/shift/{id}/tips` `{members}`: pools; needs shift.manage.
- Members are `{staff_id: "stf_...", minutes}`.

## Interactions

| Action | Does | Then |
|---|---|---|
| In / Out | Adds or takes a person out | The shares redraw |
| Hours, minutes | Sets their time | The shares redraw |
| Pool the tips, confirm | Records the pool | The read-only table |

## States

- **Loading:** "Loading the tips".
- **No tips:** "No tips were taken in this shift." Pooling a pool of
  nothing is allowed, and says so.
- **Refused:** the server's reason in words: "You don't have permission
  to do that. Ask a manager.", "Enter hours for everyone in the pool.",
  "This shift's tips are already pooled."
- **Offline:** "Can't reach the Nofiltr server to load the tips." with
  Try again.

## Constraints

- No arithmetic: every share is the server's; the screen only turns hours
  and minutes into minutes.
- Every number mono and right-aligned; every target 44px.
- One accent: Pool the tips.

## Reference

None in the archived design; this screen is new.
