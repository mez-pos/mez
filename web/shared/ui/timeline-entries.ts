// An order's events as lines a person reads, apart from the component.
//
// The switch is exhaustive over the generated OrderEventType, so an event type
// added in Go fails the TypeScript build here until it has words. Voids and
// refunds are entries like any other: nothing is hidden (docs/02-domain.md).
//
// Line and payment ids are the seq of the event that created them, so names
// come from the events themselves, read in order; nothing else is fetched.

import type { DiscountApplied, Money, OrderChannel, OrderEvent, PaymentMethod } from "../api-types";
import type { ChipTone } from "./status";

export interface TimelineEntry {
  readonly seq: number;
  readonly at: string;
  readonly title: string;
  readonly detail: string | null;
  /** Rendered with <Money>; never computed with. */
  readonly amount: Money | null;
  readonly tone: ChipTone;
  /** Who did it, and who authorised it when that was someone else. */
  readonly by: string;
  /** The till it was done on, when the screen can name it. */
  readonly device: string | null;
}

/** Staff names by id, when the screen has them; "Staff 7" otherwise. */
export type StaffName = (id: number) => string | undefined;

/** Device names by id; an unnamed device is left out rather than guessed. */
export type DeviceName = (id: number) => string | undefined;

const channels: Record<OrderChannel, string> = {
  counter: "Counter",
  qr_table: "QR at table",
  web: "Web",
  kiosk: "Kiosk",
  aggregator: "Delivery app",
};

const methods: Record<PaymentMethod, string> = {
  cash: "Cash",
  card: "Card",
  upi: "UPI",
  other: "Other",
};

interface Context {
  readonly lines: Map<number, string>;
  readonly payments: Map<number, { method: PaymentMethod; amount: Money }>;
  readonly staff: StaffName;
}

export function timelineEntries(events: readonly OrderEvent[], staff: StaffName = () => undefined, device: DeviceName = () => undefined): TimelineEntry[] {
  const ctx: Context = { lines: new Map(), payments: new Map(), staff };
  return events.map((event) => {
    remember(ctx, event);
    const { authorisedBy, ...described } = describe(event, ctx);
    return {
      seq: event.seq,
      at: event.at,
      detail: null,
      amount: null,
      tone: "neutral",
      ...described,
      by: byLine(ctx, event.actor_id, authorisedBy),
      device: event.device_id ? (device(event.device_id) ?? null) : null,
    };
  });
}

function remember(ctx: Context, event: OrderEvent): void {
  if (event.type === "line.added") ctx.lines.set(event.seq, event.payload.name);
  if (event.type === "payment.tendered") ctx.payments.set(event.seq, { method: event.payload.method, amount: event.payload.amount });
}

type Described = Partial<Omit<TimelineEntry, "seq" | "at" | "by" | "title">> & { title: string; authorisedBy?: number };

function describe(event: OrderEvent, ctx: Context): Described {
  const line = (id: number) => ctx.lines.get(id) ?? `line ${id}`;
  switch (event.type) {
    case "order.opened": {
      const p = event.payload;
      // Covers only when counted: a counter sale's "0 covers" says nothing.
      const covers = p.covers > 0 ? `${p.covers} ${p.covers === 1 ? "cover" : "covers"}` : null;
      const parts = [channels[p.channel], p.table_id ? `Table ${p.table_id}` : null, covers];
      return { title: "Order opened", detail: parts.filter(Boolean).join(" · ") };
    }
    case "line.added": {
      const p = event.payload;
      const mods = p.modifiers.map((m) => m.name).join(", ");
      return { title: `${p.qty} × ${p.name}`, detail: mods || null, amount: p.unit_price };
    }
    case "line.qty_changed":
      return { title: `${line(event.payload.line_id)}: ${event.payload.from} → ${event.payload.to}` };
    case "order.held":
      return { title: "Put on hold", tone: "warning" };
    case "order.resumed":
      return { title: "Taken off hold" };
    case "order.table_changed":
      return {
        title: event.payload.from ? `Moved from table ${event.payload.from} to ${event.payload.to}` : `Seated at table ${event.payload.to}`,
      };
    case "order.tables_joined":
      return { title: `Joined ${tablesText(event.payload.tables)}` };
    case "order.tables_parted":
      return { title: `Let go ${tablesText(event.payload.tables)}` };
    case "ticket.started":
      return { title: `Started at the ${event.payload.station}` };
    case "ticket.bumped":
      return { title: `Ready at the ${event.payload.station}`, tone: "success" };
    case "ticket.recalled":
      return { title: `Recalled at the ${event.payload.station}`, tone: "warning" };
    case "line.modified": {
      const names = (mods: readonly { name: string }[]) => mods.map((m) => m.name).join(", ") || "nothing";
      return { title: `Changed ${line(event.payload.line_id)}`, detail: `${names(event.payload.from)} → ${names(event.payload.to)}` };
    }
    case "line.voided":
      return {
        title: `Voided ${line(event.payload.line_id)}`,
        detail: humanise(event.payload.reason_code),
        tone: "danger",
        authorisedBy: event.payload.authorised_by,
      };
    case "line.comped":
      return {
        title: `Comped ${line(event.payload.line_id)}`,
        detail: humanise(event.payload.reason_code),
        tone: "info",
        authorisedBy: event.payload.authorised_by,
      };
    case "discount.applied":
      return discount(event.payload, line);
    case "discount.removed":
      return { title: "Discount removed", detail: event.payload.reason ? humanise(event.payload.reason) : null };
    case "note.added":
      return { title: event.payload.line_id ? `Note on ${line(event.payload.line_id)}` : "Note on the order", detail: event.payload.text };
    case "course.fired":
      return { title: `Course ${event.payload.course} fired`, tone: "warning" };
    case "ticket.sent": {
      const n = event.payload.line_ids.length;
      return { title: `Sent to ${event.payload.station}`, detail: `${n} ${n === 1 ? "line" : "lines"}` };
    }
    case "payment.tendered": {
      const p = event.payload;
      return { title: `${methods[p.method]} payment`, detail: p.reference ?? null, amount: p.amount, tone: "success" };
    }
    case "payment.voided": {
      const paid = ctx.payments.get(event.payload.payment_id);
      return {
        title: paid ? `Voided ${methods[paid.method].toLowerCase()} payment` : "Voided a payment",
        detail: humanise(event.payload.reason),
        amount: paid?.amount ?? null,
        tone: "danger",
        authorisedBy: event.payload.authorised_by,
      };
    }
    case "invoice.issued":
      return { title: `Invoice ${event.payload.invoice_no}`, tone: "success" };
    case "refund.issued":
      return { title: "Refund", detail: humanise(event.payload.reason), amount: event.payload.amount, tone: "danger" };
    case "order.closed":
      return { title: "Order closed", tone: "success" };
    case "order.reopened":
      return { title: "Order reopened", detail: humanise(event.payload.reason), tone: "warning", authorisedBy: event.payload.authorised_by };
    case "credit_note.issued":
      return {
        title: `Credit note ${event.payload.credit_no}`,
        detail: `Against ${event.payload.against_invoice_no}: ${humanise(event.payload.reason) ?? event.payload.reason}`,
        amount: event.payload.total,
        tone: "danger",
        authorisedBy: event.payload.authorised_by,
      };
    case "invoice.reprinted":
      return {
        title: `Reprinted ${event.payload.invoice_no}`,
        detail: humanise(event.payload.reason),
        tone: "warning",
        authorisedBy: event.payload.authorised_by,
      };
  }
}

function discount(p: DiscountApplied, line: (id: number) => string): Described {
  const target = p.line_id ? line(p.line_id) : "the order";
  if (p.kind === "pct" && p.basis_points !== undefined) {
    return { title: `${percent(p.basis_points)} off ${target}`, detail: humanise(p.reason) };
  }
  return { title: `Discount on ${target}`, detail: humanise(p.reason), amount: p.amount ?? null };
}

/** Basis points as a percentage, by string, so 1250 is "12.5%" with no float. */
export function percent(basisPoints: number): string {
  const whole = Math.trunc(basisPoints / 100);
  const rest = String(Math.abs(basisPoints % 100))
    .padStart(2, "0")
    .replace(/0+$/, "");
  return rest ? `${whole}.${rest}%` : `${whole}%`;
}

/** "wrong_item" to "Wrong item". */
export function humanise(code: string): string | null {
  const words = code.replace(/_/g, " ").trim();
  return words ? words[0]!.toUpperCase() + words.slice(1) : null;
}

function byLine(ctx: Context, actor: number, authorisedBy: number | undefined): string {
  const name = (id: number) => ctx.staff(id) ?? `Staff ${id}`;
  if (authorisedBy && authorisedBy !== actor) return `${name(actor)}, authorised by ${name(authorisedBy)}`;
  return name(actor);
}

/** "table 7", "tables 7 and 8", "tables 7, 8 and 9". */
function tablesText(tables: readonly number[]): string {
  if (tables.length === 1) return `table ${tables[0]}`;
  return `tables ${tables.slice(0, -1).join(", ")} and ${tables[tables.length - 1]}`;
}
