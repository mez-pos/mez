import { describe, expect, it } from "vitest";
import type { OrderEvent } from "../api-types";
import { humanise, percent, timelineEntries } from "./timeline-entries";

const inr = (minor: number) => ({ minor, currency: "INR" });
const at = "2026-09-21T13:05:00Z";
const base = { at, actor_id: 1, device_id: 1 };

// A whole order: opened, two lines, a void, a discount, a payment voided and
// paid again, invoiced, refunded, closed.
const events: OrderEvent[] = [
  { ...base, seq: 1, type: "order.opened", payload: { channel: "counter", currency: "INR", table_id: 4, covers: 2, opened_by: 1 } },
  { ...base, seq: 2, type: "line.added", payload: { item_id: 9, name: "Cappuccino", qty: 2, unit_price: inr(18_000), modifiers: [{ modifier_id: 1, name: "Oat milk", price: inr(4_000) }], course: 1, station: "bar", tax_rate: 500 } },
  { ...base, seq: 3, type: "line.added", payload: { item_id: 3, name: "Avocado toast", qty: 1, unit_price: inr(56_000), modifiers: [], course: 1, station: "kitchen", tax_rate: 500 } },
  { ...base, seq: 4, type: "line.qty_changed", payload: { line_id: 2, from: 2, to: 3 } },
  { ...base, seq: 5, type: "line.voided", actor_id: 2, payload: { line_id: 3, reason_code: "wrong_item", authorised_by: 1 } },
  { ...base, seq: 6, type: "discount.applied", payload: { scope: "order", kind: "pct", basis_points: 1250, reason: "Regular" } },
  { ...base, seq: 7, type: "ticket.sent", payload: { station: "bar", line_ids: [2] } },
  { ...base, seq: 8, type: "payment.tendered", payload: { method: "card", amount: inr(47_250), tip: inr(0), reference: "AUTH 1234", tendered_by: 1 } },
  { ...base, seq: 9, type: "payment.voided", payload: { payment_id: 8, reason: "Card declined after", authorised_by: 1 } },
  { ...base, seq: 10, type: "invoice.issued", payload: { invoice_no: "CAF/26-27/000042", series: "CAF", tax_snapshot: {} } },
  { ...base, seq: 11, type: "refund.issued", payload: { amount: inr(5_000), against_payment_id: 8, reason: "Cold coffee" } },
  { ...base, seq: 12, type: "order.closed", payload: { closed_by: 1 } },
];

const names: Record<number, string> = { 1: "Asha", 2: "Ravi" };

describe("timelineEntries", () => {
  const entries = timelineEntries(events, (id) => names[id]);
  const titles = entries.map((e) => e.title);

  it("describes every event, in order", () => {
    expect(titles).toEqual([
      "Order opened",
      "2 × Cappuccino",
      "1 × Avocado toast",
      "Cappuccino: 2 → 3",
      "Voided Avocado toast",
      "12.5% off the order",
      "Sent to bar",
      "Card payment",
      "Voided card payment",
      "Invoice CAF/26-27/000042",
      "Refund",
      "Order closed",
    ]);
  });

  it("returns only the entry's own fields", () => {
    expect(Object.keys(entries[4] ?? {}).sort()).toEqual(["amount", "at", "by", "detail", "seq", "title", "tone"]);
  });

  it("gives the details a person needs", () => {
    expect(entries[0]?.detail).toBe("Counter · Table 4 · 2 covers");
    expect(entries[1]?.detail).toBe("Oat milk");
    expect(entries[4]?.detail).toBe("Wrong item");
    expect(entries[6]?.detail).toBe("1 line");
  });

  it("carries amounts to render, never computed", () => {
    expect(entries[1]?.amount).toEqual(inr(18_000));
    expect(entries[8]?.amount).toEqual(inr(47_250)); // the voided payment's own amount
    expect(entries[10]?.amount).toEqual(inr(5_000));
    expect(entries[11]?.amount).toBeNull();
  });

  it("marks voids and refunds as danger, and keeps them", () => {
    expect(entries.filter((e) => e.tone === "danger").map((e) => e.seq)).toEqual([5, 9, 11]);
  });

  it("names who did it, and who authorised it when different", () => {
    expect(entries[0]?.by).toBe("Asha");
    expect(entries[4]?.by).toBe("Ravi, authorised by Asha");
    expect(timelineEntries(events)[0]?.by).toBe("Staff 1");
  });

  it("falls back to the line id for a line it never saw", () => {
    const orphan: OrderEvent = { ...base, seq: 2, type: "line.qty_changed", payload: { line_id: 99, from: 1, to: 2 } };
    expect(timelineEntries([orphan])[0]?.title).toBe("line 99: 1 → 2");
  });
});

describe("percent", () => {
  const tests = [
    { bp: 1250, want: "12.5%" },
    { bp: 1000, want: "10%" },
    { bp: 1850, want: "18.5%" },
    { bp: 5, want: "0.05%" },
    { bp: 10000, want: "100%" },
    { bp: 1234, want: "12.34%" },
  ];
  for (const tt of tests) {
    it(`${tt.bp} basis points`, () => expect(percent(tt.bp)).toBe(tt.want));
  }
});

describe("humanise", () => {
  it("turns a code into words", () => expect(humanise("wrong_item")).toBe("Wrong item"));
  it("returns nothing for nothing", () => expect(humanise("")).toBeNull());
});

describe("timelineEntries: a line changed before the kitchen had it", () => {
  it("says what it was and what it is", () => {
    const [, , changed, emptied] = timelineEntries(
      [
        events[0]!,
        events[1]!,
        { ...base, seq: 3, type: "line.modified", payload: { line_id: 2, from: [{ modifier_id: 1, name: "Oat milk", price: inr(4_000) }], to: [{ modifier_id: 2, name: "Dairy", price: inr(0) }] } },
        { ...base, seq: 4, type: "line.modified", payload: { line_id: 2, from: [{ modifier_id: 2, name: "Dairy", price: inr(0) }], to: [] } },
      ],
      (id) => names[id],
    );
    expect(changed).toMatchObject({ title: "Changed Cappuccino", detail: "Oat milk → Dairy" });
    expect(emptied).toMatchObject({ detail: "Dairy → nothing" });
  });
});
