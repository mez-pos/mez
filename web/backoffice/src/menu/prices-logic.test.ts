import { describe, expect, it } from "vitest";
import type { MenuPriceChange, MenuPrices } from "../../../shared/api-types";
import { applyLabel, appliedText, byLargestChange, differenceOf, freeCount, priceProblemsText, priceSummary, priceVerdict, unknownText } from "./prices-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const change = (name: string, old: number, next: number, row = 2): MenuPriceChange => ({
  row,
  item_id: row,
  category: "Coffee",
  name,
  old: inr(old),
  new: inr(next),
});

const plan = (patch: Partial<MenuPrices> = {}): MenuPrices => ({ version: 7, changes: [], unchanged: 0, unknown: [], problems: [], applied: false, ...patch });

describe("priceVerdict", () => {
  it("is ready with a price to change and nothing wrong", () => {
    expect(priceVerdict(plan({ changes: [change("Latte", 18000, 19000)] }))).toBe("ready");
  });

  it("is problems as soon as there is one", () => {
    expect(priceVerdict(plan({ changes: [change("Latte", 18000, 19000)], problems: [{ row: 3, column: "Price", message: "x" }] }))).toBe("problems");
  });

  it("is nothing when no price differs, even with rows that are not on the menu", () => {
    expect(priceVerdict(plan({ unchanged: 4, unknown: [{ row: 5, category: "Coffee", name: "Cortado" }] }))).toBe("nothing");
  });
});

describe("differenceOf", () => {
  it("is how far the price moves and which way", () => {
    expect(differenceOf(change("Latte", 18000, 19000))).toEqual({ direction: "up", amount: inr(1000) });
    expect(differenceOf(change("Latte", 18000, 17550))).toEqual({ direction: "down", amount: inr(450) });
  });
});

describe("byLargestChange", () => {
  it("puts the largest move first, whichever way it goes, and leaves the plan as it was", () => {
    const changes = [
      change("Small rise", 10000, 10500, 2),
      change("Slipped digit", 12000, 120000, 3),
      change("Big cut", 50000, 20000, 4),
      change("Free", 9000, 0, 5),
    ];
    expect(byLargestChange(changes).map((c) => c.name)).toEqual(["Slipped digit", "Big cut", "Free", "Small rise"]);
    expect(changes.map((c) => c.name)).toEqual(["Small rise", "Slipped digit", "Big cut", "Free"]);
  });

  it("keeps the file's order between changes of one size", () => {
    const changes = [change("B", 100, 200, 2), change("A", 300, 200, 3)];
    expect(byLargestChange(changes).map((c) => c.name)).toEqual(["B", "A"]);
  });
});

describe("priceSummary", () => {
  it("counts the prices, and leaves out what does not apply", () => {
    expect(priceSummary(plan({ changes: [change("Latte", 18000, 19000)] }))).toEqual(["1 price to change."]);
  });

  it("counts the rows that stay as they are", () => {
    expect(priceSummary(plan({ changes: [change("Latte", 1, 2), change("Mocha", 1, 2, 3)], unchanged: 1 }))).toEqual([
      "2 prices to change.",
      "1 row keeps the price it has.",
    ]);
    expect(priceSummary(plan({ changes: [change("Latte", 1, 2)], unchanged: 30 }))).toEqual(["1 price to change.", "30 rows keep the price they have."]);
  });
});

describe("unknownText", () => {
  const row = (name: string, n: number) => ({ row: n, category: "Coffee", name });

  it("is nothing when every row is an item", () => {
    expect(unknownText(plan())).toBeNull();
  });

  it("names the rows that are not on the menu and where to add them", () => {
    expect(unknownText(plan({ unknown: [row("Cortado", 3)] }))).toBe("1 row is not on the menu and is left out: Cortado. To add it, use Import new items.");
    expect(unknownText(plan({ unknown: [row("Cortado", 3), row("Idli", 4)] }))).toBe(
      "2 rows are not on the menu and are left out: Cortado, Idli. To add them, use Import new items.",
    );
  });

  it("names five and counts the rest", () => {
    const unknown = ["A", "B", "C", "D", "E", "F", "G", "H"].map((n, i) => row(n, i + 2));
    expect(unknownText(plan({ unknown }))).toBe("8 rows are not on the menu and are left out: A, B, C, D, E and 3 more. To add them, use Import new items.");
  });
});

describe("freeCount", () => {
  it("counts the items a file would make free", () => {
    expect(freeCount(plan({ changes: [change("Latte", 18000, 0), change("Mocha", 18000, 19000, 3), change("Water", 2000, 0, 4)] }))).toBe(2);
  });
});

describe("the words on the button, the toast and the problems", () => {
  it("counts on the button", () => {
    expect(applyLabel(plan({ changes: [change("Latte", 1, 2)] }))).toBe("Change 1 price");
    expect(applyLabel(plan({ changes: [change("Latte", 1, 2), change("Mocha", 1, 2, 3)] }))).toBe("Change 2 prices");
  });

  it("says what changed, and what did not", () => {
    expect(appliedText(plan({ changes: [change("Latte", 1, 2)] }))).toBe("Changed 1 price. Orders already open keep theirs.");
    expect(appliedText(plan({ changes: [change("Latte", 1, 2), change("Mocha", 1, 2, 3)] }))).toBe("Changed 2 prices. Orders already open keep theirs.");
  });

  it("says how many problems, and that no price changed", () => {
    expect(priceProblemsText(plan({ problems: [{ row: 3, column: "Price", message: "x" }] }), "menu.csv")).toBe(
      "1 problem in menu.csv. No price has been changed.",
    );
  });
});
