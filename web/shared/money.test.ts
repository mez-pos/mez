import { describe, expect, it } from "vitest";
import { amountInputText, averageMoney, exponentOf, formatMoney, formatMoneyCompact, parseAmount } from "./money";

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

describe("parseAmount", () => {
  const tests: { text: string; currency: string; want: number | null }[] = [
    { text: "180", currency: "INR", want: 18000 },
    { text: "180.5", currency: "INR", want: 18050 },
    { text: "180.50", currency: "INR", want: 18050 },
    { text: " 180.50 ", currency: "INR", want: 18050 },
    { text: "1,80,000.00", currency: "INR", want: 18_000_000 },
    { text: "1 234", currency: "INR", want: 123_400 },
    { text: ".5", currency: "INR", want: 50 },
    { text: "5.", currency: "INR", want: 500 },
    { text: "0", currency: "INR", want: 0 },
    { text: "0.29", currency: "INR", want: 29 },
    { text: "0.07", currency: "USD", want: 7 },
    { text: "1234", currency: "JPY", want: 1234 },
    { text: "1.234", currency: "KWD", want: 1234 },
    { text: "", currency: "INR", want: null },
    { text: ".", currency: "INR", want: null },
    { text: "12.345", currency: "INR", want: null },
    { text: "12.0", currency: "JPY", want: null },
    { text: "-5", currency: "INR", want: null },
    { text: "abc", currency: "INR", want: null },
    { text: "1e5", currency: "INR", want: null },
    { text: "1.2.3", currency: "INR", want: null },
    { text: "₹180", currency: "INR", want: null },
    { text: "99999999999999999", currency: "INR", want: null },
  ];
  for (const tt of tests) {
    it(`reads ${JSON.stringify(tt.text)} ${tt.currency} as ${tt.want}`, () => {
      expect(parseAmount(tt.text, tt.currency)).toBe(tt.want);
    });
  }
});

describe("amountInputText", () => {
  const tests = [
    { minor: 18050, currency: "INR", want: "180.50" },
    { minor: 18000, currency: "INR", want: "180.00" },
    { minor: 5, currency: "INR", want: "0.05" },
    { minor: 0, currency: "USD", want: "0.00" },
    { minor: 1234, currency: "JPY", want: "1234" },
    { minor: 1234, currency: "KWD", want: "1.234" },
    { minor: -250, currency: "GBP", want: "-2.50" },
  ];
  for (const tt of tests) {
    it(`writes ${tt.minor} ${tt.currency} as ${tt.want}, which reads back`, () => {
      const text = amountInputText({ minor: tt.minor, currency: tt.currency });
      expect(text).toBe(tt.want);
      if (tt.minor >= 0) expect(parseAmount(text, tt.currency)).toBe(tt.minor);
    });
  }
});

describe("formatMoneyCompact", () => {
  const tests: { name: string; minor: number; currency: string; want: string }[] = [
    { name: "thousands of rupees", minor: 400_000, currency: "INR", want: "₹4K" },
    { name: "a lakh and a half", minor: 15_000_000, currency: "INR", want: "₹1.5L" },
    { name: "under a thousand stays whole", minor: 25_000, currency: "INR", want: "₹250" },
    { name: "nothing", minor: 0, currency: "INR", want: "₹0" },
    { name: "dollars", minor: 250_000, currency: "USD", want: "$2.5K" },
    { name: "a currency with no minor unit", minor: 4000, currency: "JPY", want: "¥4K" },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(formatMoneyCompact({ minor: tt.minor, currency: tt.currency })).toBe(tt.want));
  }

  it("refuses an amount that is not a safe integer", () => {
    expect(() => formatMoneyCompact({ minor: 1.5, currency: "INR" })).toThrow(RangeError);
  });
});

describe("averageMoney", () => {
  const inr = (minor: number) => ({ minor, currency: "INR" });
  it("is the total over the count, to the nearest minor unit", () => {
    expect(averageMoney(inr(260_000), 9)).toEqual(inr(28_889));
    expect(averageMoney(inr(80_000), 2)).toEqual(inr(40_000));
    expect(averageMoney(inr(100), 3)).toEqual(inr(33));
    expect(averageMoney(inr(200), 3)).toEqual(inr(67));
  });
  it("is nothing with nothing to divide by", () => {
    expect(averageMoney(inr(100), 0)).toBeNull();
    expect(averageMoney(inr(100), -1)).toBeNull();
  });
});
