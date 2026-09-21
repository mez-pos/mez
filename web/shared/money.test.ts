import { describe, expect, it } from "vitest";
import { exponentOf, formatMoney } from "./money";

describe("formatMoney", () => {
  // Intl joins a currency code to its number with a no-break space (U+00A0),
  // so "KWD" never wraps away from its amount at the end of a line.
  const nbsp = String.fromCharCode(0xa0);
  const tests = [
    { name: "rupees and paise", minor: 12345, currency: "INR", want: "₹123.45" },
    { name: "zero", minor: 0, currency: "INR", want: "₹0.00" },
    { name: "paise only", minor: 5, currency: "INR", want: "₹0.05" },
    { name: "negative paise keep their sign", minor: -5, currency: "INR", want: "−₹0.05" },
    { name: "lakh grouping for rupees", minor: 12_000_000, currency: "INR", want: "₹1,20,000.00" },
    { name: "a refund", minor: -148_000, currency: "INR", want: "−₹1,480.00" },
    { name: "the largest safe amount, exactly", minor: Number.MAX_SAFE_INTEGER, currency: "INR", want: "₹9,00,71,99,25,47,409.91" },
    { name: "dollars group in thousands", minor: 123_456_789, currency: "USD", want: "$1,234,567.89" },
    { name: "pence", minor: 99, currency: "GBP", want: "£0.99" },
    { name: "euro", minor: 1234, currency: "EUR", want: "€12.34" },
    { name: "yen has no minor unit", minor: 1234, currency: "JPY", want: "¥1,234" },
    { name: "dinar has three places", minor: 1234, currency: "KWD", want: `KWD${nbsp}1.234` },
    { name: "an unlisted but well-formed code takes two places", minor: 150, currency: "XYZ", want: `XYZ${nbsp}1.50` },
    { name: "a malformed code still renders, without Intl", minor: -150, currency: "rupee", want: "−1.50 rupee" },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(formatMoney({ minor: tt.minor, currency: tt.currency })).toBe(tt.want));
  }

  it("takes an explicit locale over the currency's", () => {
    expect(formatMoney({ minor: 12_000_000, currency: "INR" }, "en-US")).toBe("₹120,000.00");
  });

  const refused = [
    { name: "a fraction of a paisa", minor: 12.5 },
    { name: "beyond safe integers", minor: 2 ** 53 },
    { name: "not a number", minor: Number.NaN },
  ];
  for (const tt of refused) {
    it(`refuses ${tt.name}`, () => {
      expect(() => formatMoney({ minor: tt.minor, currency: "INR" })).toThrow(RangeError);
    });
  }
});

describe("exponentOf", () => {
  const tests = [
    { currency: "INR", want: 2 },
    { currency: "JPY", want: 0 },
    { currency: "KWD", want: 3 },
    { currency: "CLF", want: 4 },
    { currency: "ZZZ", want: 2 },
  ];
  for (const tt of tests) {
    it(tt.currency, () => expect(exponentOf(tt.currency)).toBe(tt.want));
  }
});
