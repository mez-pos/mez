import { describe, expect, it } from "vitest";
import { countedTotal, faceText, floatRequest, joinNames, setFaceCount, soldOutNames, soldOutNotice, typedFloat } from "./open-shift-logic";
import { maxPieces } from "./shift-logic";

describe("setFaceCount", () => {
  const tests = [
    { name: "a count typed", text: "12", want: 12 },
    { name: "an empty box is none", text: "", want: 0 },
    { name: "letters and signs are dropped", text: "1a2-", want: 12 },
    { name: "only letters is none", text: "abc", want: 0 },
    { name: "leading zeros", text: "007", want: 7 },
    { name: "spaces from a paste", text: " 4 5 ", want: 45 },
    { name: "a decimal point is dropped, not read as a fraction", text: "2.5", want: 25 },
    { name: "stops at the ceiling", text: "9999999999", want: maxPieces },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(setFaceCount({}, 50000, tt.text)[50000]).toBe(tt.want);
    });
  }

  it("leaves the other notes and does not change the tally it was given", () => {
    const before = { 50000: 2, 1000: 3 };
    const after = setFaceCount(before, 1000, "9");
    expect(after).toEqual({ 50000: 2, 1000: 9 });
    expect(before).toEqual({ 50000: 2, 1000: 3 });
  });
});

describe("soldOutNames", () => {
  const now = new Date("2026-10-04T10:00:00Z");
  const items = [
    { name: "Masala Chai" },
    { name: "French Fries", sold_out_until: "2026-10-05T00:00:00Z" },
    { name: "Cold Coffee", sold_out_until: "2026-10-04T09:00:00Z" },
    { name: "Brownie", sold_out_until: "2026-10-06T00:00:00Z" },
    { name: "Odd", sold_out_until: "not a time" },
  ];
  it("names what is sold out now, in order, and not what is back or unreadable", () => {
    expect(soldOutNames(items, now)).toEqual(["French Fries", "Brownie"]);
  });
  it("is empty for an empty menu", () => {
    expect(soldOutNames([], now)).toEqual([]);
  });
});

describe("soldOutNotice", () => {
  it("says nothing when nothing is sold out", () => {
    expect(soldOutNotice([])).toBeNull();
  });
  it("is singular for one", () => {
    expect(soldOutNotice(["French Fries"])).toEqual({
      title: "1 item is sold out",
      body: "French Fries is sold out and can't be ordered until back on sale.",
    });
  });
  it("names two and three", () => {
    expect(soldOutNotice(["A", "B"])?.body).toBe("A and B are sold out and can't be ordered until back on sale.");
    expect(soldOutNotice(["A", "B", "C"])?.body).toBe("A, B and C are sold out and can't be ordered until back on sale.");
  });
  it("names three and counts the rest", () => {
    expect(soldOutNotice(["A", "B", "C", "D", "E"])).toEqual({
      title: "5 items are sold out",
      body: "A, B, C and 2 more are sold out and can't be ordered until back on sale.",
    });
  });
});

describe("joinNames", () => {
  it("joins the way a person says it", () => {
    expect(joinNames([])).toBe("");
    expect(joinNames(["Arjun"])).toBe("Arjun");
    expect(joinNames(["Arjun", "Priya"])).toBe("Arjun and Priya");
    expect(joinNames(["Arjun", "Priya", "Meera"])).toBe("Arjun, Priya and Meera");
  });
});

describe("typedFloat", () => {
  const tests = [
    { name: "an amount", text: "1500", want: 150000 },
    { name: "an amount with paise", text: "1500.50", want: 150050 },
    { name: "an empty box is none", text: "  ", want: 0 },
    { name: "text that is not an amount is not zero", text: "15oo", want: null },
    { name: "too many decimals", text: "1.234", want: null },
    { name: "a sign", text: "-5", want: null },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(typedFloat(tt.text, "INR")).toBe(tt.want));
  }
});

describe("floatRequest", () => {
  it("is nothing to send while the typed amount is not one", () => {
    expect(floatRequest("XYZ", { tally: {}, typed: null })).toBeNull();
  });
  it("sends the typed amount for a currency with no notes", () => {
    expect(floatRequest("XYZ", { tally: {}, typed: 0 })).toEqual({ total: { minor: 0, currency: "XYZ" } });
    expect(floatRequest("XYZ", { tally: {}, typed: 4200 })).toEqual({ total: { minor: 4200, currency: "XYZ" } });
  });
  it("sends the notes counted, whatever was typed, for a currency with them", () => {
    const req = floatRequest("INR", { tally: { 50000: 2 }, typed: null });
    expect(req?.total).toEqual({ minor: 100000, currency: "INR" });
  });
});

describe("countedTotal", () => {
  const tests = [
    { name: "the notes counted", currency: "INR", counted: { tally: { 50000: 2, 2000: 10 }, typed: 0 }, want: 120000 },
    { name: "nothing counted", currency: "INR", counted: { tally: {}, typed: 0 }, want: 0 },
    { name: "notes, whatever was typed", currency: "INR", counted: { tally: { 100: 3 }, typed: 999 }, want: 300 },
    { name: "the amount typed, with no notes for the currency", currency: "XYZ", counted: { tally: {}, typed: 4200 }, want: 4200 },
    { name: "text that is not an amount counts as nothing", currency: "XYZ", counted: { tally: {}, typed: null }, want: 0 },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(countedTotal(tt.currency, tt.counted)).toBe(tt.want));
  }
});

describe("faceText", () => {
  const tests = [
    { name: "a note, without paise", face: 50000, currency: "INR", want: "₹500" },
    { name: "the smallest rupee", face: 100, currency: "INR", want: "₹1" },
    { name: "a dollar bill", face: 10000, currency: "USD", want: "$100" },
    { name: "a coin under the unit keeps its cents", face: 25, currency: "USD", want: "$0.25" },
    { name: "a euro coin", face: 50, currency: "EUR", want: "€0.50" },
    { name: "a pound coin", face: 200, currency: "GBP", want: "£2" },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(faceText(tt.face, tt.currency)).toBe(tt.want));
  }
  it("names every INR face without paise", () => {
    expect([50000, 20000, 10000, 5000, 2000, 1000, 500, 200, 100].map((face) => faceText(face, "INR"))).toEqual([
      "₹500",
      "₹200",
      "₹100",
      "₹50",
      "₹20",
      "₹10",
      "₹5",
      "₹2",
      "₹1",
    ]);
  });
});
