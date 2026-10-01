import { describe, expect, it } from "vitest";
import { formatDelta, formatQty, parseQty, qtyInputText } from "./qty";

describe("parseQty", () => {
  const tests: { text: string; signed?: boolean; want: number | null }[] = [
    { text: "250", want: 250_000 },
    { text: "1.5", want: 1_500 },
    { text: "0.125", want: 125 },
    { text: " 5,000 ", want: 5_000_000 },
    { text: ".5", want: 500 },
    { text: "0", want: 0 },
    { text: "0.1", want: 100 },
    { text: "", want: null },
    { text: ".", want: null },
    { text: "1.2345", want: null },
    { text: "abc", want: null },
    { text: "1e3", want: null },
    { text: "-5", want: null },
    { text: "-5", signed: true, want: -5_000 },
    { text: "−0.25", signed: true, want: -250 },
    { text: "--5", signed: true, want: null },
    { text: "99999999999999", want: null },
  ];
  for (const tt of tests) {
    it(`reads ${JSON.stringify(tt.text)}${tt.signed ? " signed" : ""} as ${tt.want}`, () => {
      expect(parseQty(tt.text, tt.signed)).toBe(tt.want);
    });
  }
});

describe("qtyInputText", () => {
  for (const [q, want] of [
    [4_750_000, "4750"],
    [1_500, "1.5"],
    [125, "0.125"],
    [0, "0"],
    [-250, "-0.25"],
  ] as const) {
    it(`writes ${q} as ${want}, which reads back`, () => {
      expect(qtyInputText(q)).toBe(want);
      expect(parseQty(want, true)).toBe(q);
    });
  }
});

describe("formatQty and formatDelta", () => {
  it("groups the whole part and keeps only the decimals there are", () => {
    expect(formatQty(4_750_000, "g")).toBe("4,750 g");
    expect(formatQty(1_500, "each")).toBe("1.5 each");
    expect(formatQty(200_000, "ml")).toBe("200 ml");
    expect(formatQty(-50_000, "g")).toBe("−50 g");
    expect(formatQty(0, "g")).toBe("0 g");
  });
  it("signs a change either way", () => {
    expect(formatDelta(5_000_000, "g")).toBe("+5,000 g");
    expect(formatDelta(-50_000, "g")).toBe("−50 g");
    expect(formatDelta(0, "ml")).toBe("0 ml");
  });
});
