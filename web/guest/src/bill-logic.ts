// The table's bill in words (docs/design/screens/guest-bill.md, ADR-214).
// Every figure is the server's: this decides which rows to draw and what to
// call them, and adds nothing up.

import type { GuestBillResponse, Money } from "../../shared/api-types";
import { rateText } from "../../shared/zreport/zreport-logic";

/** How often an open bill is read again while it is on screen. */
export const BILL_REFRESH_MS = 15_000;

export interface BillRow {
  readonly label: string;
  readonly amount: Money;
  /** A row that explains the total without adding to it: tax already in the prices. */
  readonly within?: boolean;
  /** Money coming off. */
  readonly minus?: boolean;
}

/** "Items (3)" by dishes charged and given, counting each one of a line. */
export function itemsLabel(bill: Pick<GuestBillResponse, "lines">): string {
  const count = bill.lines.reduce((sum, l) => sum + l.qty, 0);
  return `Items (${count})`;
}

/**
 * The rows between the lines and the total: what the dishes come to, what
 * was taken off, and each tax by its name and rate. With tax in the prices
 * the tax rows say "Includes", so nobody adds them to a total that already
 * holds them.
 */
export function summaryRows(bill: GuestBillResponse): BillRow[] {
  const rows: BillRow[] = [{ label: itemsLabel(bill), amount: bill.items }];
  if (bill.discounts.minor !== 0) rows.push({ label: "Discount", amount: bill.discounts, minus: true });
  for (const t of bill.taxes) {
    const name = `${t.name} (${rateText(t.rate)})`;
    rows.push(bill.tax_included ? { label: `Includes ${name}`, amount: t.amount, within: true } : { label: name, amount: t.amount });
  }
  return rows;
}

export type BillState = "none" | "due" | "part_paid" | "paid";

/** Where the bill stands: nothing open, all of it owed, some of it paid, or settled. */
export function billState(bill: GuestBillResponse): BillState {
  if (!bill.open) return "none";
  if (bill.balance_due.minor <= 0 && bill.total.minor > 0) return "paid";
  return bill.paid.minor > 0 ? "part_paid" : "due";
}

/** What to do about it, until a guest can pay from the phone (4e). */
export function payLine(state: BillState): string {
  switch (state) {
    case "none":
      return "Nothing is on this table's bill yet. What you order appears here once it is accepted.";
    case "paid":
      return "This bill is paid. Thank you.";
    case "due":
    case "part_paid":
      return "To pay, ask a member of staff or pay at the counter.";
  }
}
