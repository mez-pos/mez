import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { StockItem, StockMovement } from "../../../shared/api-types";
import {
  costMeasures,
  countDelta,
  emptyItemForm,
  emptyMovementForm,
  itemFormFrom,
  movementText,
  newRecipeRow,
  problemOf,
  recipeRowsFrom,
  recipeText,
  toItemInput,
  toMovementInput,
  toParts,
} from "./stock-logic";

const beans: StockItem = {
  id: 1,
  name: "Beans",
  unit: "g",
  low_level: 2_000_000,
  active: true,
  on_hand: 4_750_000,
  low: false,
  count_at_close: true,
  cost: { minor: 125000, currency: "INR" },
};
const milk: StockItem = {
  id: 2,
  name: "Milk",
  unit: "ml",
  low_level: 0,
  active: true,
  on_hand: 0,
  low: false,
  count_at_close: false,
  cost: { minor: 0, currency: "INR" },
};

describe("toMovementInput", () => {
  it("sends a delivery in thousandths, with no reason", () => {
    expect(toMovementInput("received", { qty: "5000", reason: "spilled", note: " " })).toEqual({ input: { kind: "received", qty: 5_000_000 } });
  });
  it("sends waste with its reason and note", () => {
    expect(toMovementInput("wasted", { qty: "1.5", reason: "other", note: " Dropped the tray " })).toEqual({
      input: { kind: "wasted", qty: 1_500, reason: "other", note: "Dropped the tray" },
    });
  });
  it("lets a correction be negative, and nothing else", () => {
    expect(toMovementInput("adjusted", { qty: "-500", reason: "", note: "Miscount" })).toMatchObject({ input: { qty: -500_000 } });
    expect(toMovementInput("received", { ...emptyMovementForm(), qty: "-5" })).toMatchObject({ problem: { field: "qty" } });
    expect(toMovementInput("adjusted", emptyMovementForm())).toMatchObject({ problem: { message: /-500/ } });
  });
  it("refuses an amount it cannot read before sending", () => {
    expect(toMovementInput("counted", { ...emptyMovementForm(), qty: "lots" })).toMatchObject({ problem: { field: "qty" } });
  });
});

describe("countDelta", () => {
  it("is what was counted less what the ledger says", () => {
    expect(countDelta("4700", 4_750_000)).toBe(-50_000);
    expect(countDelta("4750", 4_750_000)).toBe(0);
    expect(countDelta("", 4_750_000)).toBeNull();
  });
});

describe("movementText", () => {
  const m = (over: Partial<StockMovement>): StockMovement => ({ id: 1, kind: "received", delta: 0, staff_id: "s", staff_name: "Asha", at: "x", ...over });
  it("words each kind", () => {
    expect(movementText(m({ kind: "received" }), "g")).toBe("Received");
    expect(movementText(m({ kind: "wasted", reason: "spilled" }), "g")).toBe("Wasted: spilled");
    expect(movementText(m({ kind: "counted", counted: 4_700_000 }), "g")).toBe("Counted 4,700 g");
    expect(movementText(m({ kind: "adjusted" }), "g")).toBe("Corrected");
    expect(movementText(m({ kind: "sold" }), "g")).toBe("Sold");
  });
});

describe("the item form", () => {
  it("reads an item in and back out", () => {
    const form = itemFormFrom(beans);
    expect(form).toEqual({ name: "Beans", unit: "g", low: "2000", countAtClose: true, cost: "1250.00" });
    expect(toItemInput(form, "INR")).toEqual({
      input: { name: "Beans", unit: "g", low_level: 2_000_000, count_at_close: true, cost: { minor: 125000, currency: "INR" } },
    });
  });
  it("treats an empty low level as none, and trims the name", () => {
    expect(itemFormFrom(milk).low).toBe("");
    expect(toItemInput({ ...emptyItemForm(), name: " Ice " }, "INR")).toEqual({
      input: { name: "Ice", unit: "g", low_level: 0, count_at_close: false, cost: { minor: 0, currency: "INR" } },
    });
  });
  it("refuses a low level it cannot read", () => {
    expect(toItemInput({ ...emptyItemForm(), low: "some" }, "INR")).toMatchObject({ problem: { field: "low_level" } });
  });
  it("treats an empty cost as not known, and reads one in the venue currency", () => {
    expect(itemFormFrom(milk).cost).toBe("");
    expect(toItemInput({ ...emptyItemForm(), name: "Milk", unit: "ml", cost: " 60 " }, "INR")).toMatchObject({
      input: { cost: { minor: 6000, currency: "INR" } },
    });
    expect(toItemInput({ ...emptyItemForm(), name: "Yen", cost: "60" }, "JPY")).toMatchObject({ input: { cost: { minor: 60, currency: "JPY" } } });
  });
  it("refuses a cost it cannot read", () => {
    for (const cost of ["cheap", "-5", "12.345"]) {
      expect(toItemInput({ ...emptyItemForm(), cost }, "INR")).toMatchObject({ problem: { field: "cost" } });
    }
  });
  it("names what a cost is for", () => {
    expect(costMeasures.g).toEqual({ full: "a kilogram", short: "kg" });
    expect(costMeasures.ml.short).toBe("l");
    expect(costMeasures.each.full).toBe("one");
  });
});

describe("recipes", () => {
  const stock = new Map([
    [1, beans],
    [2, milk],
  ]);
  it("words what a recipe takes", () => {
    expect(
      recipeText(
        [
          { stock_item_id: 1, qty: 18_000 },
          { stock_item_id: 2, qty: 200_000 },
        ],
        stock,
      ),
    ).toBe("Beans 18 g, Milk 200 ml");
    expect(recipeText([], stock)).toBe("");
  });
  it("reads a recipe into rows and back", () => {
    const rows = recipeRowsFrom([{ stock_item_id: 1, qty: 18_500 }]);
    expect(rows[0]).toMatchObject({ stockItemId: 1, qty: "18.5" });
    expect(toParts(rows)).toEqual({ parts: [{ stock_item_id: 1, qty: 18_500 }] });
  });
  it("names the row whose amount cannot be read", () => {
    const rows = [...recipeRowsFrom([{ stock_item_id: 1, qty: 18_000 }]), { ...newRecipeRow(2), qty: "a bit" }];
    expect(toParts(rows)).toMatchObject({ problem: { row: { index: 1, field: "qty" } } });
  });
  it("gives new rows their own keys", () => {
    expect(newRecipeRow().key).not.toBe(newRecipeRow().key);
  });
});

describe("problemOf", () => {
  const refusal = (code: string, field: string | null) => new ApiError(code as ConstructorParameters<typeof ApiError>[0], "The server's words.", 422, field);
  it("puts a field's refusal on it, and a recipe row's on its row", () => {
    expect(problemOf(refusal("invalid_stock", "unit"))).toEqual({ field: "unit", message: "The server's words." });
    expect(problemOf(refusal("invalid_stock", "parts.2.stock_item_id")).row).toEqual({ index: 2, field: "stock_item_id" });
  });
  it("words a refusal of permission, a lost server and anything else", () => {
    expect(problemOf(refusal("forbidden", "stock.manage")).message).toMatch(/manage stock/);
    expect(problemOf(new ApiError("network", "x")).message).toMatch(/Can't reach/);
    expect(problemOf(new Error("boom")).message).toBe("Something went wrong. Try again.");
  });
});
