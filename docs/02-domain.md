# Domain

This is the most important document in the repository. Read it before touching
anything that involves money, orders, invoices or tax.

## Money

```go
type Amount struct {
    Minor    int64  // paise, cents — never fractional
    Currency string // ISO 4217, "INR"
}
```

Rules:

- Floats are banned. Not for totals, not for tax, not for percentages applied to
  money. A lint rule should fail the build on `float64` in `internal/domain`.
- Percentages are stored as basis points (`int32`, 1850 = 18.50%).
- Addition and subtraction require matching currency or they return an error.
- Rounding happens **once**, at the last possible moment, using banker's
  rounding (half-to-even). The rounding rule is a venue setting because tax
  regimes differ.
- Every rounding operation that loses money records a `rounding_adjustment` on
  the order so the Z-report still balances to zero.

### The tip ceiling

A tip is never part of the bill, a card or UPI tip is never counted against
the drawer, and every tip is paid out through the pool. So a tip over the
venue's ceiling, `tips.max_percent` of the payment's amount (50 unless set,
0 to 1000, 0 meaning every tip), rounded up to the next minor unit, is
refused with `ErrTipNeedsApproval` unless a manager holding `tip.approve`
let it through (ADR-261). The service enforces it, since the projection does
not know the venue's rules; the approver is `authorised_by` on the event.

### The split-payment invariant

The sum of all tenders must equal the order total exactly. When a three-way
split of ₹1,000.00 produces ₹333.34 / ₹333.33 / ₹333.33, the largest-remainder
method decides who carries the extra paisa, deterministically, and the result is
asserted in tests. No division that can silently lose a unit.

## Orders as events

An order is an append-only stream. The current state is a projection. Nothing
mutates an emitted event, ever.

```
orders          (id, venue_id, channel, opened_at, closed_at, table_id, ...)
order_events    (id, order_id, seq, type, payload_json, actor_id, at, device_id)
order_snapshots (order_id, seq, state_json)   -- optimisation only, rebuildable
```

`(order_id, seq)` is unique. `seq` is dense and starts at 1.

### Event types

| Type | Payload |
|---|---|
| `order.opened` | channel, service (dine_in, takeaway, delivery), table, covers, opened_by, taxes: how the venue was taxed as it opened, which the order is projected under for good (ADR-374) |
| `line.added` | item_id, qty, unit_price, modifiers[], course, station |
| `line.qty_changed` | line_id, from, to: only before the line reaches the kitchen |
| `line.modified` | line_id, from[], to[]: modifiers, only before the line reaches the kitchen |
| `line.voided` | line_id, reason_code, authorised_by |
| `line.comped` | line_id, reason_code, authorised_by |
| `discount.applied` | scope (line/order), kind (pct/amount), value, reason |
| `discount.removed` | seq of the discount, reason, authorised_by: the discount stays in the stream and takes nothing off from here on (ADR-373) |
| `note.added` | scope, text |
| `course.fired` | course number |
| `ticket.sent` | station, line_ids[] |
| `payment.tendered` | method, amount, tip, reference, tendered_by, authorised_by: who let through a tip over the venue's ceiling (ADR-261) |
| `payment.voided` | payment_id, reason, authorised_by |
| `invoice.issued` | invoice_no, series, tax_snapshot, prices (seller, lines, totals at issue) |
| `invoice.reprinted` | invoice_no, reason, authorised_by: a copy, never a new number; allowed after a close |
| `credit_note.issued` | credit_no, series, against_invoice_no, reason, authorised_by, lines (units or a share, with tax by component), totals: money back on an invoiced bill, never a second invoice; allowed after a close |
| `refund.issued` | amount, against_payment_id, reason |
| `order.closed` | closed_by |
| `order.reopened` | reason, authorised_by |
| `order.held` | none: parks an open order, which then takes only notes and a resume |
| `order.resumed` | (none) |
| `order.table_changed` | from, to: the ticket moves to another table; from is where the till saw it |
| `order.tables_joined` | tables[]: one party spread over more tables, still one order and one bill |
| `order.tables_parted` | tables[]: joined tables let go again |
| `ticket.bumped` | ticket_seq, station: on the pass, ready; allowed after a close and on hold |
| `ticket.recalled` | ticket_seq, station: a bump undone, back in the kitchen as new |
| `ticket.started` | ticket_seq, station: a cook has picked it up, in progress; allowed after a close and on hold |

Two consequences worth naming: the **order timeline** screen in the design is
just this list rendered, and **voids are visible forever**, which is exactly what
a tax audit and an owner checking on staff both need.

### Projection

`internal/domain/order` exposes a pure `Project(events []Event) State`. It takes
no database and no clock — the clock is an event field. This is the function
that gets the heaviest test coverage in the repo.

## Prices and the snapshot rule

A line stores the price **at the time it was added**, not a reference to the
current menu price. If the owner changes a price at 7pm, tickets opened at 6pm
settle at the old price. Never join to `items` to compute a historical total.

The same applies to tax rates, modifiers and item names. An issued invoice
carries a full snapshot of everything used to compute it: `tax_snapshot` (the
tax result) and `prices` (the seller, each active line as billed, and the
totals). A preview or a reprint renders from those, never from today's menu,
venue settings or projection (ADR-078).

**A refund on an invoiced bill is a credit note** (ADR-083): its own gapless
series, naming the invoice, with the refunds that pay it out after it; the
balance is `Total - Credited - NetPaid`. A reopen corrects payments only: it
never allows a second invoice.

**An invoiced bill is frozen.** The payment that brings the balance to zero
issues the invoice in its own transaction. From then until a reopen, no event
may change what was sold (`line.*`, `discount.applied`): the projection
refuses it with `ErrOrderInvoiced`. Notes, kitchen events, refunds, the close
and `invoice.reprinted` still may.

## Invoice series

- Monotonic, **gapless**, per venue, per financial year, per series.
- Allocated inside the same transaction that issues the invoice, via a row lock
  on a `invoice_series` counter row. Never `MAX(n)+1`.
- Format is a template: `{prefix}/{fy}/{seq:06d}`.
- A number is never reused, never reassigned, and never issued for an order that
  fails to commit. If the print fails, the invoice still exists — reprint is a
  separate, logged action.
- Financial year boundary is a venue setting (India: 1 April).

Gaps in an invoice series are a compliance problem in most jurisdictions. Treat
an off-by-one here as a production incident, not a bug.

## Tax profiles

Tax is pluggable from day one. A profile decides how tax is computed, displayed
and reported.

```go
type TaxProfile interface {
    Name() string
    Compute(lines []Line, ctx VenueContext) (TaxResult, error)
    InvoiceFields() []Field      // what must be printed
    RegisterColumns() []Column   // what the tax register export needs
}
```

Ship three:

- **GST (India)** — CGST/SGST split for intra-state, IGST inter-state, HSN codes
  per item, inclusive or exclusive pricing per venue, GSTIN on invoice, tax
  register export.
- **VAT (UK/EU)** — single rate per item, inclusive display, VAT number.
- **US sales tax** — exclusive, added at checkout, rate by jurisdiction.

Do not hardcode GST anywhere outside its profile. The first non-Indian user will
otherwise force a rewrite.

## Identifiers

- Internal primary keys: `int64` autoincrement. Fast, small indexes.
- Anything that leaves the building (order codes, invoice references, device
  pairing, guest tokens): a short, unambiguous, non-sequential public ID.
  Use a Crockford base-32 alphabet with I/L/O/U removed — staff read these aloud
  across a noisy room.
- Pickup codes are three or four characters and are called out loud. Filter a
  profanity list, in English and in the venue's language.
- A secret that says who is asking is not a public ID: a device's token, a
  staff session's, and a guest phone's session for the sitting (`gst_...`,
  ADR-262) are 256 random bits, kept as their SHA-256. A table's code
  says which table and lasts until it is renewed, so it opens the menu and
  nothing that is the sitting's: ordering to the kitchen, the bill and paying
  it need the phone's session too, which ends when its ticket closes.

## Shifts and the close of day

A shift is the reporting unit, not the calendar day. A bar closing at 2am
belongs to the previous trading day.

Close sequence, in order, each step recorded:

1. **Blind drawer count** — staff enter counted cash *before* the system shows
   expected. Non-negotiable; a visible expected figure defeats the control.
2. Variance computed and stored, over/short, with a reason field.
3. Card batch reconciled against recorded tenders.
4. Wastage and closing stock posted.
5. Z-report generated, immutable, numbered. Orders still open are not closed
   with the shift: they carry into the next one, and the report records how
   many and what was still to pay on them, read at the close (ADR-384).
6. Tips pooled and split per the venue's configured rule.
7. Tax register and accounting export queued.

A Z-report, once generated, is immutable. A correction is a new document
referencing the old one.

## Channels

`counter`, `qr_table`, `web`, `kiosk`, `aggregator`. Channel affects routing,
prepay requirements and reporting — **not** the shape of an order. One ticket
model, regardless of origin. This is the core product promise; do not fork it.

## Service type

How an order is served is its **service type**: `dine_in`, `takeaway` or
`delivery` (ADR-146). It is not the channel: a counter order may be any of
the three. It is a field of `order.opened`, always written by the service.

- An order opened before the field existed reads as `dine_in` when it has a
  table and `takeaway` when it has none. The events on disk are not touched.
- Takeaway and delivery have no table. A dine-in order may open with none, the
  table still to be picked.
- Seating a takeaway (`order.table_changed` from 0) makes it dine-in, so the
  service never disagrees with the table. A delivery cannot be seated
  (`ErrServiceNoTable`).
- Guests are `covers` on the same event. There is no event to change them
  after the order opens.

## States

**Order:** open → (held) → open → settling → closed → (reopened) → closed
(a counter sale with no table goes settling → closed in the one
transaction of its settling payment, which also sends what waits: ADR-383)
**Table:** free → seated → ordered → settling → dirty → free
**Ticket at a station:** queued → fired → in progress → ready → collected,
plus fired → ready (bumped straight off the display) and ready → fired
(recalled), ADR-090
**Payment:** pending → captured → (voided | refunded)

Illegal transitions return typed errors, not panics, and are tested explicitly.
