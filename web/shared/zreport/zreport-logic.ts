// Words for a Z-report on screen (docs/design/screens/zreport.md). Nothing
// here adds money up: every figure is the stored report's.

import type { Money, PaymentMethod, ZNameResponse } from "../api-types";
import { formatMoney } from "../money";

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

/** The error a report screen shows, in words. */
export function reportProblem(status: number | undefined, number?: number): string {
  if (status === 403) return "You don't have permission to see reports. Ask a manager.";
  if (status === 404 && number !== undefined) return `There is no Z-report ${number}.`;
  return "Can't reach the mez server to load the report.";
}
