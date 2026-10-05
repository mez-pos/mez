// Every status a StatusChip can show, as data: the tone it takes and the word
// it says. A chip takes one of these keys, never free text, so a screen cannot
// invent "Almost ready" in a colour nobody agreed on.
//
// The server's vocabularies are Records over the generated unions, so a status
// added in Go fails the TypeScript build here until it has a tone and a word.
// Tones follow docs/design/tokens.md: terracotta (accent) is for "Late" alone.

import type { DayStatus, LineStatus, OrderStatus, PaymentStatus, PrintJobState, TableStatus } from "../api-types";

export type ChipTone = "neutral" | "success" | "warning" | "danger" | "accent" | "info";

export interface ChipSpec {
  readonly tone: ChipTone;
  readonly word: string;
  /** Voided things stay visible forever, struck through (docs/02-domain.md). */
  readonly struck?: true;
}

/** The kitchen's states, from tokens.md. Not a server enum yet (phase 08). */
export type KitchenStatus = "queued" | "firing" | "ready" | "late" | "void";

export const kitchenStatuses: Readonly<Record<KitchenStatus, ChipSpec>> = {
  queued: { tone: "neutral", word: "Queued" },
  firing: { tone: "warning", word: "Firing" },
  ready: { tone: "success", word: "Ready" },
  late: { tone: "accent", word: "Late" },
  void: { tone: "neutral", word: "Void", struck: true },
};

export const orderStatuses: Readonly<Record<OrderStatus, ChipSpec>> = {
  open: { tone: "neutral", word: "Open" },
  held: { tone: "info", word: "Held" },
  settling: { tone: "warning", word: "Settling" },
  closed: { tone: "success", word: "Closed" },
  reopened: { tone: "warning", word: "Reopened" },
};

export const paymentStatuses: Readonly<Record<PaymentStatus, ChipSpec>> = {
  pending: { tone: "warning", word: "Pending" },
  captured: { tone: "success", word: "Paid" },
  voided: { tone: "neutral", word: "Voided", struck: true },
  refunded: { tone: "danger", word: "Refunded" },
};

export const lineStatuses: Readonly<Record<LineStatus, ChipSpec>> = {
  active: { tone: "neutral", word: "Active" },
  voided: { tone: "neutral", word: "Void", struck: true },
  comped: { tone: "info", word: "Comped" },
};

// A print job's words are about the paper, not the queue: staff need to
// know whether the guest has their receipt (ADR-081).
export const printStatuses: Readonly<Record<PrintJobState, ChipSpec>> = {
  queued: { tone: "neutral", word: "Waiting" },
  printing: { tone: "info", word: "Printing" },
  done: { tone: "success", word: "Printed" },
  failed: { tone: "warning", word: "Retrying" },
  dead: { tone: "danger", word: "Not printed" },
};

// A table's turn (terminal-floor.md, ADR-086). The words are the floor's:
// staff say a table is paying or needs clearing, not that it is settling
// or dirty. Never terracotta: a table is not "late". The tones are the
// design's floor, one per state: free is green, seated is blue, a table
// with an order on it is the occupied rose (the danger chip's fill, which
// is no alarm here), paying is mustard and one to clear is the cleaning grey.
export const tableStatuses: Readonly<Record<TableStatus, ChipSpec>> = {
  free: { tone: "success", word: "Free" },
  seated: { tone: "info", word: "Seated" },
  ordered: { tone: "danger", word: "Ordered" },
  settling: { tone: "warning", word: "Paying" },
  dirty: { tone: "neutral", word: "To clear" },
};

// An order on the day's list (D09), in the list's four words. Open is the
// pending mustard: it still wants its bill.
export const dayStatuses: Readonly<Record<DayStatus, ChipSpec>> = {
  open: { tone: "warning", word: "Open" },
  paid: { tone: "success", word: "Paid" },
  refunded: { tone: "danger", word: "Refunded" },
  voided: { tone: "neutral", word: "Voided", struck: true },
};

// Where a stock item stands (D12, D19). Low, at or under its level, is the
// pending mustard; none left is the alarm.
export type StockLevel = "ok" | "low" | "out";

export const stockLevels: Readonly<Record<StockLevel, ChipSpec>> = {
  ok: { tone: "success", word: "Healthy" },
  low: { tone: "warning", word: "Low" },
  out: { tone: "danger", word: "Out" },
};

// A menu item in the back office's list (D18). Sold out is for the rest of
// the trading day (ADR-109); retired is off every till until brought back.
export type MenuItemState = "active" | "sold_out" | "retired";

export const menuItemStates: Readonly<Record<MenuItemState, ChipSpec>> = {
  active: { tone: "success", word: "Active" },
  sold_out: { tone: "warning", word: "Sold out" },
  retired: { tone: "neutral", word: "Retired" },
};

// A staff member on the back office's list (D20). Deactivated is off the
// sign-in screen until brought back; their name stays on what they did.
export type StaffState = "active" | "off";

export const staffStates: Readonly<Record<StaffState, ChipSpec>> = {
  active: { tone: "success", word: "Active" },
  off: { tone: "neutral", word: "Deactivated" },
};

/** What a StatusChip is given: which vocabulary, and a value from it. */
export type ChipStatus =
  | { readonly kind: "kitchen"; readonly status: KitchenStatus }
  | { readonly kind: "order"; readonly status: OrderStatus }
  | { readonly kind: "payment"; readonly status: PaymentStatus }
  | { readonly kind: "line"; readonly status: LineStatus }
  | { readonly kind: "print"; readonly status: PrintJobState }
  | { readonly kind: "table"; readonly status: TableStatus }
  | { readonly kind: "day"; readonly status: DayStatus }
  | { readonly kind: "stock"; readonly status: StockLevel }
  | { readonly kind: "menu"; readonly status: MenuItemState }
  | { readonly kind: "staff"; readonly status: StaffState };

export function chipSpec(chip: ChipStatus): ChipSpec {
  switch (chip.kind) {
    case "kitchen":
      return kitchenStatuses[chip.status];
    case "order":
      return orderStatuses[chip.status];
    case "payment":
      return paymentStatuses[chip.status];
    case "line":
      return lineStatuses[chip.status];
    case "print":
      return printStatuses[chip.status];
    case "table":
      return tableStatuses[chip.status];
    case "day":
      return dayStatuses[chip.status];
    case "stock":
      return stockLevels[chip.status];
    case "menu":
      return menuItemStates[chip.status];
    case "staff":
      return staffStates[chip.status];
  }
}
