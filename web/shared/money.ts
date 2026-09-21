// Turning an amount into text. The only place that happens: <Money> calls
// this, and nothing else may render money (docs/design/components.md).
//
// Money arrives as minor units, an integer (docs/02-domain.md). It is never
// divided into a float. The major part goes to Intl as a BigInt, which is
// exact at any size, and the minor digits are cut from the integer's own
// decimal string. A float would show ₹90,07,19,92,54,74,09.91 as .92.

import { currencyExponents, defaultCurrencyExponent, type Money } from "./api-types";

// U+2212, not a hyphen: the same width as a digit in JetBrains Mono, so a
// refund lines up with the charges above it.
const minus = "−";

// Until the venue's own locale arrives with its settings (phase 10), the
// currency picks the grouping its customers expect: a lakh is 1,00,000.
const localeByCurrency: Readonly<Record<string, string>> = {
  INR: "en-IN",
  GBP: "en-GB",
  USD: "en-US",
  EUR: "en-IE",
};

export function exponentOf(currency: string): number {
  return currencyExponents[currency] ?? defaultCurrencyExponent;
}

/**
 * The display text for an amount: "₹1,20,000.00", "¥1,234", "−£0.99".
 * Throws RangeError for an amount that is not a safe integer: that is a bug
 * upstream, not something to round quietly.
 */
export function formatMoney(amount: Money, locale?: string): string {
  const { minor, currency } = amount;
  if (!Number.isSafeInteger(minor)) {
    throw new RangeError(`money must be a safe integer of minor units, got ${minor} ${currency}`);
  }
  const exponent = exponentOf(currency);
  const { major, fraction } = split(Math.abs(minor), exponent);
  const sign = minor < 0 ? minus : "";

  const format = formatterFor(locale ?? localeByCurrency[currency] ?? "en", currency, exponent);
  if (!format) {
    // Not a code Intl accepts. Say what it is rather than fail a till.
    return `${sign}${major}${exponent > 0 ? `.${fraction}` : ""} ${currency}`;
  }
  return sign + format.formatToParts(BigInt(major)).map((part) => (part.type === "fraction" ? fraction : part.value)).join("");
}

/** 12345 at exponent 2 is "123" and "45"; 5 is "0" and "05". */
function split(absMinor: number, exponent: number): { major: string; fraction: string } {
  const digits = String(absMinor).padStart(exponent + 1, "0");
  const cut = digits.length - exponent;
  return { major: digits.slice(0, cut), fraction: digits.slice(cut) };
}

const formatters = new Map<string, Intl.NumberFormat | null>();

function formatterFor(locale: string, currency: string, exponent: number): Intl.NumberFormat | null {
  const key = `${locale}|${currency}|${exponent}`;
  let format = formatters.get(key);
  if (format === undefined) {
    try {
      format = new Intl.NumberFormat(locale, {
        style: "currency",
        currency,
        minimumFractionDigits: exponent,
        maximumFractionDigits: exponent,
      });
    } catch {
      format = null;
    }
    formatters.set(key, format);
  }
  return format;
}
