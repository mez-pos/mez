// Made-up data for the gallery. Shapes are the generated API types, so a
// server change that breaks them breaks this file too.

import type { Money, OrderEvent, OrderStatus } from "../../../shared/api-types";

export const inr = (minor: number): Money => ({ minor, currency: "INR" });

const base = { actor_id: 1, device_id: 1 };
const at = (minute: number) => `2026-09-21T07:${String(minute).padStart(2, "0")}:00Z`;

export const sampleEvents: OrderEvent[] = [
  { ...base, seq: 1, at: at(2), type: "order.opened", payload: { channel: "counter", currency: "INR", table_id: 4, covers: 2, opened_by: 1 } },
  { ...base, seq: 2, at: at(3), type: "line.added", payload: { item_id: 9, name: "Cappuccino", qty: 2, unit_price: inr(18_000), modifiers: [{ modifier_id: 1, name: "Oat milk", price: inr(4_000) }], course: 1, station: "bar", tax_rate: 500 } },
  { ...base, seq: 3, at: at(3), type: "line.added", payload: { item_id: 3, name: "Avocado toast", qty: 1, unit_price: inr(56_000), modifiers: [], course: 1, station: "kitchen", tax_rate: 500 } },
  { ...base, seq: 4, at: at(5), type: "line.voided", actor_id: 2, payload: { line_id: 3, reason_code: "wrong_item", authorised_by: 1 } },
  { ...base, seq: 5, at: at(5), type: "ticket.sent", payload: { station: "bar", line_ids: [2] } },
  { ...base, seq: 6, at: at(18), type: "discount.applied", payload: { scope: "order", kind: "pct", basis_points: 1000, reason: "Regular" } },
  { ...base, seq: 7, at: at(21), type: "payment.tendered", payload: { method: "upi", amount: inr(39_600), tip: inr(0), tendered_by: 1 } },
  { ...base, seq: 8, at: at(21), type: "invoice.issued", payload: { invoice_no: "CAF/26-27/000042", series: "CAF", tax_snapshot: {} } },
  { ...base, seq: 9, at: at(34), type: "refund.issued", payload: { amount: inr(5_000), against_payment_id: 7, reason: "Cold coffee" } },
  { ...base, seq: 10, at: at(35), type: "order.closed", payload: { closed_by: 1 } },
];

export const staffNames: Record<number, string> = { 1: "Asha", 2: "Ravi" };

export interface SampleOrder {
  id: string;
  table: string;
  status: OrderStatus;
  opened: string;
  total: Money;
}

const statuses: OrderStatus[] = ["open", "held", "settling", "closed", "reopened"];

export const sampleOrders: SampleOrder[] = Array.from({ length: 23 }, (_, i) => ({
  id: `ord_${String(1000 + i)}`,
  table: i % 4 === 0 ? "Counter" : `Table ${(i % 12) + 1}`,
  status: statuses[i % statuses.length] ?? "open",
  opened: `${String(8 + Math.floor(i / 3)).padStart(2, "0")}:${String((i * 7) % 60).padStart(2, "0")}`,
  total: inr(((i * 7919) % 400_000) + 12_000),
}));
