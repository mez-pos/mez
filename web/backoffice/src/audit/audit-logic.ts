// What the Audit screen says about the trail (backoffice-audit.md). The trail
// and its figures are the server's (ADR-117); this words them and builds the
// query the server reads.

import type { AuditEntry, AuditKind } from "../../../shared/api-types";

/** Each kind in words, one and many. A Record, so a new kind fails the typecheck until it has words. */
export const kindWords: Readonly<Record<AuditKind, { one: string; many: string }>> = {
  void: { one: "Void", many: "Voids" },
  comp: { one: "Comp", many: "Comps" },
  discount: { one: "Discount", many: "Discounts" },
  refund: { one: "Refund", many: "Refunds" },
  payment_void: { one: "Payment void", many: "Payment voids" },
  reopen: { one: "Reopen", many: "Reopens" },
};

export const kinds = Object.keys(kindWords) as AuditKind[];

/** The hour a trading day starts, as the server counts it (shift.DefaultDayStart). */
const dayStartHour = 4;

const pad = (n: number) => String(n).padStart(2, "0");
const dayOf = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

/** Today's trading day on this machine's clock: before 04:00 it is still yesterday's. */
export function tradingDay(now: Date): string {
  const d = new Date(now);
  if (d.getHours() < dayStartHour) d.setDate(d.getDate() - 1);
  return dayOf(d);
}

/** The last n trading days, today included. */
export function lastDays(now: Date, n: number): { from: string; to: string } {
  const to = tradingDay(now);
  const [y, m, d] = to.split("-").map(Number) as [number, number, number];
  const from = new Date(y, m - 1, d - (n - 1));
  return { from: dayOf(from), to };
}

/** "24 Sept", for the empty state. */
export function dayText(day: string): string {
  const [y, m, d] = day.split("-").map(Number) as [number, number, number];
  return new Date(y, m - 1, d).toLocaleDateString("en-GB", { day: "numeric", month: "short" });
}

/** A reason code in words: "wrong_item" reads "Wrong item"; free text stays as typed. */
export function reasonWords(reason: string | undefined): string {
  if (!reason) return "";
  if (!/^[a-z0-9_]+$/.test(reason)) return reason;
  const words = reason.replace(/_/g, " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/** Who let it through, in words: their name, "Themselves", or "Not recorded" (before ADR-117). */
export function authorisedText(e: AuditEntry): { text: string; muted: boolean } {
  if (!e.authorised_by) return { text: "Not recorded", muted: true };
  if (e.by && e.authorised_by.id !== "" && e.authorised_by.id === e.by.id) return { text: "Themselves", muted: true };
  return { text: e.authorised_by.name, muted: false };
}

/** What it was about: "Chai × 1", "Whole bill" for a bill discount, or nothing. */
export function itemText(e: AuditEntry): string {
  if (e.item) return `${e.item} × ${e.qty ?? 1}`;
  return e.kind === "discount" ? "Whole bill" : "";
}

export interface AuditFilter {
  from: string;
  to: string;
  /** A public staff id, or empty for anyone. */
  staff: string;
  /** One kind, or null for every kind. */
  kind: AuditKind | null;
}

/** The query string for a page of the trail. */
export function auditQuery(f: AuditFilter, before: number, limit = 50): string {
  const p = new URLSearchParams({ from: f.from, to: f.to, limit: String(limit) });
  if (f.staff) p.set("staff", f.staff);
  if (f.kind) p.set("kind", f.kind);
  if (before) p.set("before", String(before));
  return `/api/v1/audit?${p.toString()}`;
}
