// Words for a Z-report on screen (docs/design/screens/zreport.md). Nothing
// here adds money up: every figure is the stored report's.

import type { Money, PaymentMethod, Tally, ZNameResponse } from "../api-types";
import { formatMoney } from "../money";

/**
 * The tips a manager let through over the venue's ceiling (ADR-261), or null
 * when there were none. A report issued before they were counted has no
 * such field, whatever the type says.
 */
export function approvedTips(r: { tips_approved?: Tally }): Tally | null {
  const t = r.tips_approved;
  return t && t.count > 0 ? t : null;
}

/** A tax rate, thousandths of a percent, as "2.5%": by string, never a float. */
export function rateText(rate: number): string {
  const whole = Math.trunc(rate / 1000);
  const frac = String(Math.abs(rate % 1000))
    .padStart(3, "0")
    .replace(/0+$/, "");
  return frac === "" ? `${whole}%` : `${whole}.${frac}%`;
}

const channels: Readonly<Record<string, string>> = {
  counter: "Counter",
  qr_table: "QR at the table",
  web: "Web",
  kiosk: "Kiosk",
  aggregator: "Delivery apps",
};

/** A channel's name for a person. */
export function channelName(key: string): string {
  return channels[key] ?? key;
}

const methods: Readonly<Record<PaymentMethod, string>> = { cash: "Cash", card: "Card", upi: "UPI", other: "Other" };

export function methodName(m: PaymentMethod): string {
  return methods[m];
}

/** Who an ID is, from the names the report was issued with. */
export function staffName(names: readonly ZNameResponse[], id: number): string {
  return names.find((n) => n.id === id)?.name ?? `Staff ${id}`;
}

/** How the drawer came out: a word and a tone, never colour alone. */
export function differenceWords(variance: Money): { text: string; tone: "success" | "danger" | "warning" } {
  const by = formatMoney({ ...variance, minor: Math.abs(variance.minor) });
  if (variance.minor < 0) return { text: `Short ${by}`, tone: "danger" };
  if (variance.minor > 0) return { text: `Over ${by}`, tone: "warning" };
  return { text: "Balanced", tone: "success" };
}

/** "18:02" in the device's zone: the till and the back office are at the venue. */
export function clockText(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false });
}

/** A slip's date line: the trading day and when the shift closed, "2026-10-05 · 23:38". */
export function stampText(tradingDay: string, closedAt: string): string {
  const at = clockText(closedAt);
  return at ? `${tradingDay} · ${at}` : tradingDay;
}

const services: Readonly<Record<string, string>> = { dine_in: "Dine-in", takeaway: "Takeaway", delivery: "Delivery" };

/** How an order was served, for a person. */
export function serviceName(key: string): string {
  return services[key] ?? key;
}

/**
 * What the Tax card says under its rows when credit notes were issued in the
 * shift: its rows are the tax invoiced less the tax credited, so they come to
 * less than the Sales card's Tax, which is the tax invoiced. Null with none.
 */
export function taxNote(creditNotes: number): string | null {
  if (creditNotes <= 0) return null;
  return `After ${creditNotes} ${creditNotes === 1 ? "credit note" : "credit notes"}: the tax invoiced, less the tax credited.`;
}

/**
 * The slip's two lines under its Total when credit notes were issued in the
 * shift: what they take off it, and the net sales left. The slip's Total is
 * before credit notes and its tax rows are after them, and with nothing
 * between the two the slip did not add up to the eye (ADR-342). The printout
 * has the same two lines. Null with none.
 */
export function creditLines(r: { sales: { net_of_credits: Money }; credit_notes: { count: number; amount: Money } }): {
  label: string;
  less: Money;
  net: Money;
} | null {
  if (r.credit_notes.count <= 0) return null;
  return {
    label: `Less credit notes (${r.credit_notes.count})`,
    less: { ...r.credit_notes.amount, minor: -Math.abs(r.credit_notes.amount.minor) },
    net: r.sales.net_of_credits,
  };
}

/** The error a report screen shows, in words. */
export function reportProblem(status: number | undefined, number?: number): string {
  if (status === 403) return "You don't have permission to see reports. Ask a manager.";
  if (status === 404 && number !== undefined) return `There is no Z-report ${number}.`;
  return "Can't reach the mez server to load the report.";
}

/**
 * The orders still open when the shift closed, which went on into the next
 * one, and what was still to pay on them (ADR-384): "3 · ₹650.00 to pay",
 * "None", or null on a report issued before they were counted.
 */
export function carriedText(carried: Tally | undefined): string | null {
  if (!carried) return null;
  if (carried.count === 0) return "None";
  const due = carried.amount.minor > 0 ? `${formatMoney(carried.amount)} to pay` : "nothing to pay";
  return `${carried.count} · ${due}`;
}
