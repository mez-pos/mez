// Stock amounts on screen (ADR-111): the server counts in thousandths of a
// unit; people type and read whole units with up to three decimals. Read
// as digits and joined, never through a float, as money is.

import type { StockUnit } from "../../../shared/api-types";

/** Thousandths in one whole unit. */
export const perUnit = 1000;

const unitText: Readonly<Record<StockUnit, string>> = { g: "g", ml: "ml", each: "each" };

/**
 * Thousandths of a unit typed in whole units: "250" is 250000, "1.5" is
 * 1500, "0.125" is 125. Null for anything else: empty, letters, more than
 * three decimals, or a sign where none is allowed. With signed, "-500" is
 * allowed and gives -500000.
 */
export function parseQty(text: string, signed = false): number | null {
  let t = text.trim().replace(/[\s,]/g, "");
  let negative = false;
  if (t.startsWith("-") || t.startsWith("−")) {
    if (!signed) return null;
    negative = true;
    t = t.slice(1);
  }
  const match = /^(\d*)(?:\.(\d*))?$/.exec(t);
  if (!match) return null;
  const [, whole = "", fraction = ""] = match;
  if ((whole === "" && fraction === "") || fraction.length > 3) return null;
  const n = Number(`${whole || "0"}${fraction.padEnd(3, "0")}`);
  if (!Number.isSafeInteger(n)) return null;
  return negative ? -n : n;
}

/** The text a field starts with for an amount: "4750", "1.5", "-0.25". No grouping, so it reads back. */
export function qtyInputText(q: number): string {
  const sign = q < 0 ? "-" : "";
  const abs = Math.abs(q);
  const whole = Math.trunc(abs / perUnit);
  const fraction = String(abs % perUnit).padStart(3, "0").replace(/0+$/, "");
  return fraction ? `${sign}${whole}.${fraction}` : `${sign}${whole}`;
}

/** An amount with its unit: "4,750 g", "1.5 each", "−50 g". The minus is U+2212, a digit wide. */
export function formatQty(q: number, unit: StockUnit): string {
  const sign = q < 0 ? "−" : "";
  const abs = Math.abs(q);
  const whole = Math.trunc(abs / perUnit).toLocaleString("en-US");
  const fraction = String(abs % perUnit).padStart(3, "0").replace(/0+$/, "");
  return `${sign}${fraction ? `${whole}.${fraction}` : whole} ${unitText[unit]}`;
}

/** A change, signed either way: "+5,000 g", "−50 g", "0 g". */
export function formatDelta(q: number, unit: StockUnit): string {
  return q > 0 ? `+${formatQty(q, unit)}` : formatQty(q, unit);
}
