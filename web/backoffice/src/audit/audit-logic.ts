// What the Audit screen says about the trail (backoffice-audit.md). The trail
// and its figures are the server's (ADR-117); this words them and builds the
// query the server reads.

import type { AuditEntry, AuditKind } from "../../../shared/api-types";
import { lastDays, type VenueClock } from "../../../shared/venue-clock";

/** Each kind in words, one and many. A Record, so a new kind fails the typecheck until it has words. */
export const kindWords: Readonly<Record<AuditKind, { one: string; many: string }>> = {
  void: { one: "Void", many: "Voids" },
  comp: { one: "Comp", many: "Comps" },
  discount: { one: "Discount", many: "Discounts" },
  refund: { one: "Refund", many: "Refunds" },
  payment_void: { one: "Payment void", many: "Payment voids" },
  reopen: { one: "Reopen", many: "Reopens" },
  // A tip over the venue's ceiling that a manager let through (ADR-261); its amount is the tip.
  tip_approved: { one: "Tip approved", many: "Tips approved" },
};

export const kinds = Object.keys(kindWords) as AuditKind[];

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

const dayPattern = /^\d{4}-\d{2}-\d{2}$/;

/** The path that opens the trail on a filter (backoffice-audit.md). */
export function auditLink(f: Pick<AuditFilter, "from" | "to"> & Partial<Pick<AuditFilter, "staff" | "kind">>): string {
  const p = new URLSearchParams({ from: f.from, to: f.to });
  if (f.staff) p.set("staff", f.staff);
  if (f.kind) p.set("kind", f.kind);
  return `/audit?${p.toString()}`;
}

/**
 * The filter the screen opens on: what its address asks for, where that can
 * be read, and the last seven days otherwise. An address is typed by anyone,
 * so a day that is not one and a kind we do not have are dropped here, not
 * sent to the server.
 */
export function filterFromSearch(search: string, now: Date, clock: VenueClock): AuditFilter {
  const p = new URLSearchParams(search);
  const from = p.get("from") ?? "";
  const to = p.get("to") ?? "";
  const kind = p.get("kind");
  const range = dayPattern.test(from) && dayPattern.test(to) && from <= to ? { from, to } : lastDays(now, 7, clock);
  return { ...range, staff: p.get("staff") ?? "", kind: kinds.find((k) => k === kind) ?? null };
}

/** What a set of corrections were, in words: "2 voids · 1 comp". Kinds with none are left out. */
export function kindsText(tallies: readonly { kind: AuditKind; count: number }[]): string {
  return tallies
    .filter((t) => t.count > 0)
    .map((t) => `${t.count} ${(t.count === 1 ? kindWords[t.kind].one : kindWords[t.kind].many).toLowerCase()}`)
    .join(" · ");
}
