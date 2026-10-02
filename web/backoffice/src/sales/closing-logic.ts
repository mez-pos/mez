// What the closing stock section works out for itself (backoffice-reports.md).
// Every figure is the server's (ADR-125); this builds the paths, lays the
// shifts out as rows and words a variance.

import type { ClosingStockLine, ClosingStockReport, Money } from "../../../shared/api-types";
import { formatMoney } from "../../../shared/money";
import type { SalesRange } from "./sales-logic";

const query = (range: SalesRange): string => new URLSearchParams({ from: range.from, to: range.to }).toString();

/** The path the range's closing stock is read from. */
export function closingPath(range: SalesRange): string {
  return `/api/v1/reports/closing-stock?${query(range)}`;
}

/** The path the range's closing stock is downloaded from as CSV. */
export function closingCsvPath(range: SalesRange): string {
  return `/api/v1/reports/closing-stock/csv?${query(range)}`;
}

export interface ClosingRow {
  /** Unique over the report: a shift counts an item once. */
  key: string;
  tradingDay: string;
  line: ClosingStockLine;
}

/** Every count as a row, in the order the server sends: by day, shift, then item. */
export function closingRows(report: ClosingStockReport): ClosingRow[] {
  return report.shifts.flatMap((sh) => sh.lines.map((line) => ({ key: `${sh.shift_id} ${line.stock_item_id}`, tradingDay: sh.trading_day, line })));
}

export interface VarianceWords {
  text: string;
  tone: "danger" | "warning" | "success" | "muted";
}

/**
 * The range's variance at cost in words and a tone, never colour alone:
 * "Short by ₹23.60", "Over by ₹6.40", "Counted as expected". With no count
 * costed there is no figure to give, and it says so.
 */
export function varianceWords(value: Money, counts: number, uncosted: number): VarianceWords {
  if (counts > 0 && uncosted >= counts) return { text: "No cost on what was counted", tone: "muted" };
  const by = formatMoney({ ...value, minor: Math.abs(value.minor) });
  if (value.minor < 0) return { text: `Short by ${by}`, tone: "danger" };
  if (value.minor > 0) return { text: `Over by ${by}`, tone: "warning" };
  return { text: "Counted as expected", tone: "success" };
}

/** How many counts a total leaves out, in words; empty when it leaves none out. */
export function uncostedText(uncosted: number): string {
  if (uncosted <= 0) return "";
  return uncosted === 1 ? "1 count had no cost and is not in this." : `${uncosted} counts had no cost and are not in this.`;
}

/** How many counts the report holds. */
export function countsIn(report: ClosingStockReport): number {
  return report.shifts.reduce((n, sh) => n + sh.lines.length, 0);
}
