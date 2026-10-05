import { describe, expect, it } from "vitest";
import type { GuestItemResponse, GuestMenuResponse } from "../../shared/api-types";
import {
  MAX_LINES,
  MAX_QTY,
  addLine,
  countOf,
  droppedText,
  emptyCart,
  groupsFor,
  itemsText,
  lineKey,
  parse,
  reconcile,
  serialize,
  setQty,
  storageKey,
  totalOf,
  type Cart,
} from "./cart-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const item = (id: number, name: string, minor: number, patch: Partial<GuestItemResponse> = {}): GuestItemResponse => ({
  id,
  category_id: 1,
  name,
  price: inr(minor),
  modifier_groups: [],
  ...patch,
});

// Milk is required, one of two. Extras are optional, up to two.
const menu: GuestMenuResponse = {
  version: 4,
  currency: "INR",
  categories: [{ id: 1, name: "Coffee" }],
  items: [item(1, "Flat white", 21000, { modifier_groups: [10, 11] }), item(2, "Filter coffee", 6000), item(3, "Brownie", 15000, { sold_out: true })],
  modifier_groups: [
    {
      id: 10,
      name: "Milk",
      min_select: 1,
      max_select: 1,
      modifiers: [
        { id: 100, name: "Dairy", price: inr(0), is_default: true },
        { id: 101, name: "Oat", price: inr(4000), is_default: false },
      ],
    },
    {
      id: 11,
      name: "Extras",
      min_select: 0,
      max_select: 2,
      modifiers: [
        { id: 110, name: "Extra shot", price: inr(5000), is_default: false },
        { id: 111, name: "Decaf", price: inr(-1000), is_default: false },
        { id: 112, name: "Syrup", price: inr(3000), is_default: false },
      ],
    },
  ],
};

const flatWhiteOat = { item_id: 1, modifier_ids: [101], qty: 1 };
const filter = { item_id: 2, modifier_ids: [], qty: 1 };

describe("addLine", () => {
  it("adds a line", () => {
    expect(addLine(emptyCart, flatWhiteOat)).toEqual([flatWhiteOat]);
  });

  it("adds onto the line with the same dish and the same choices", () => {
    const cart = addLine(addLine(emptyCart, flatWhiteOat), { ...flatWhiteOat, qty: 2 });
    expect(cart).toEqual([{ ...flatWhiteOat, qty: 3 }]);
  });

  it("keeps the same dish with other choices as its own line", () => {
    const cart = addLine(addLine(emptyCart, flatWhiteOat), { item_id: 1, modifier_ids: [100], qty: 1 });
    expect(cart).toHaveLength(2);
    expect(lineKey(cart[0]!)).not.toBe(lineKey(cart[1]!));
  });

  it("holds a line to the most a line can be", () => {
    const cart = addLine(addLine(emptyCart, { ...filter, qty: MAX_QTY - 1 }), { ...filter, qty: 5 });
    expect(cart[0]?.qty).toBe(MAX_QTY);
    expect(addLine(emptyCart, { ...filter, qty: 9999 })[0]?.qty).toBe(MAX_QTY);
  });

  it.each([0, -1, Number.NaN, Number.POSITIVE_INFINITY])("adds nothing for a quantity of %s", (qty) => {
    expect(addLine(emptyCart, { ...filter, qty })).toEqual(emptyCart);
  });

  it("drops the fraction of a quantity", () => {
    expect(addLine(emptyCart, { ...filter, qty: 2.9 })[0]?.qty).toBe(2);
  });

  it("holds the cart to the most lines it can have, and still adds onto one it has", () => {
    let cart: Cart = emptyCart;
    for (let id = 1; id <= MAX_LINES + 5; id++) cart = addLine(cart, { item_id: id, modifier_ids: [], qty: 1 });
    expect(cart).toHaveLength(MAX_LINES);
    expect(addLine(cart, { item_id: 1, modifier_ids: [], qty: 1 })[0]?.qty).toBe(2);
  });

  it("does not change the cart it was given", () => {
    const cart = addLine(emptyCart, flatWhiteOat);
    addLine(cart, flatWhiteOat);
    expect(cart).toEqual([flatWhiteOat]);
  });
});

describe("setQty", () => {
  const cart = addLine(addLine(emptyCart, flatWhiteOat), filter);

  it("sets how many of one line", () => {
    expect(setQty(cart, lineKey(filter), 4)).toEqual([flatWhiteOat, { ...filter, qty: 4 }]);
  });

  it("takes the line out at none", () => {
    expect(setQty(cart, lineKey(filter), 0)).toEqual([flatWhiteOat]);
    expect(setQty(cart, lineKey(filter), -3)).toEqual([flatWhiteOat]);
  });

  it("holds it to the most a line can be", () => {
    expect(setQty(cart, lineKey(filter), 500)[1]?.qty).toBe(MAX_QTY);
  });

  it("leaves the cart alone for a line it does not have", () => {
    expect(setQty(cart, "99:", 3)).toEqual(cart);
  });
});

describe("reconcile", () => {
  it("prices a line from the menu: the dish, plus each choice, times how many", () => {
    const { lines, dropped, cart } = reconcile([{ item_id: 1, modifier_ids: [101, 110], qty: 2 }], menu);
    expect(dropped).toEqual([]);
    expect(cart).toHaveLength(1);
    expect(lines[0]).toMatchObject({
      options: "Oat, Extra shot",
      qty: 2,
      unit: inr(21000 + 4000 + 5000),
      total: inr(60000),
    });
  });

  it("takes a choice that costs less off the price", () => {
    const { lines } = reconcile([{ item_id: 1, modifier_ids: [100, 111], qty: 1 }], menu);
    expect(lines[0]?.unit).toEqual(inr(20000));
  });

  it("has no options text for a dish with no choices", () => {
    expect(reconcile([filter], menu).lines[0]?.options).toBe("");
  });

  it("uses today's price, not the one the dish was added at", () => {
    const dearer = { ...menu, items: menu.items.map((it) => (it.id === 2 ? { ...it, price: inr(7500) } : it)) };
    expect(reconcile([{ ...filter, qty: 2 }], dearer).lines[0]?.total).toEqual(inr(15000));
  });

  it.each([
    ["a dish that has sold out", { item_id: 3, modifier_ids: [], qty: 1 }, "Brownie"],
    ["a dish the menu no longer has", { item_id: 99, modifier_ids: [], qty: 1 }, "A dish"],
    ["a choice the menu no longer offers", { item_id: 1, modifier_ids: [101, 999], qty: 1 }, "Flat white"],
    ["a choice from another dish's group", { item_id: 2, modifier_ids: [101], qty: 1 }, "Filter coffee"],
    ["a required choice not made", { item_id: 1, modifier_ids: [], qty: 1 }, "Flat white"],
    ["two of a choose-one", { item_id: 1, modifier_ids: [100, 101], qty: 1 }, "Flat white"],
    ["more extras than the group allows", { item_id: 1, modifier_ids: [100, 110, 111, 112], qty: 1 }, "Flat white"],
  ])("takes out %s, and says which", (_, line, name) => {
    const got = reconcile([filter, line], menu);
    expect(got.dropped).toEqual([name]);
    expect(got.cart).toEqual([filter]);
    expect(got.lines).toHaveLength(1);
  });

  it("hands back the very same cart when nothing was taken out", () => {
    const cart: Cart = [filter, flatWhiteOat];
    expect(reconcile(cart, menu).cart).toBe(cart);
  });

  it("is nothing for an empty cart", () => {
    expect(reconcile(emptyCart, menu)).toEqual({ cart: emptyCart, lines: [], dropped: [] });
  });
});

describe("the figures", () => {
  it("counts dishes, not lines", () => {
    expect(countOf([{ ...filter, qty: 2 }, flatWhiteOat])).toBe(3);
    expect(countOf(emptyCart)).toBe(0);
  });

  it("totals the lines, in whole minor units", () => {
    const { lines } = reconcile(
      [
        { ...filter, qty: 3 },
        { item_id: 1, modifier_ids: [101], qty: 1 },
      ],
      menu,
    );
    expect(totalOf(lines, "INR")).toEqual(inr(18000 + 25000));
    expect(totalOf([], "INR")).toEqual(inr(0));
  });

  it("says how many in words", () => {
    expect(itemsText(1)).toBe("1 item");
    expect(itemsText(3)).toBe("3 items");
  });

  it("names what was taken out", () => {
    expect(droppedText([])).toBe("");
    expect(droppedText(["Brownie"])).toBe("Brownie is no longer available and was taken out of your cart.");
    expect(droppedText(["Brownie", "Latte"])).toBe("Brownie and Latte are no longer available and were taken out of your cart.");
    expect(droppedText(["Brownie", "Latte", "Idli"])).toMatch(/^Brownie, Latte and Idli are /);
  });

  it("finds a dish's groups in the dish's order, skipping one the menu lost", () => {
    const flatWhite = { ...menu.items[0]!, modifier_groups: [11, 404, 10] };
    expect(groupsFor(menu, flatWhite).map((g) => g.name)).toEqual(["Extras", "Milk"]);
  });
});

describe("storage", () => {
  it("is kept by table code", () => {
    expect(storageKey("dpyyd3kcn5ev")).toBe("mez.cart.dpyyd3kcn5ev");
  });

  it("round-trips", () => {
    const cart: Cart = [{ item_id: 1, modifier_ids: [101, 110], qty: 2 }, filter];
    expect(parse(serialize(cart))).toEqual(cart);
  });

  it.each([
    ["nothing", null],
    ["an empty string", ""],
    ["not JSON", "{oops"],
    ["not a list", '{"i":1}'],
    ["a number", "7"],
    ["null", "null"],
  ])("is an empty cart for %s", (_, text) => {
    expect(parse(text)).toEqual(emptyCart);
  });

  it.each([
    ["no dish", '[{"m":[],"q":1}]'],
    ["a dish that is not an id", '[{"i":"1","m":[],"q":1}]'],
    ["a dish id of zero", '[{"i":0,"m":[],"q":1}]'],
    ["a fractional dish id", '[{"i":1.5,"m":[],"q":1}]'],
    ["choices that are not a list", '[{"i":1,"m":"101","q":1}]'],
    ["a choice that is not an id", '[{"i":1,"m":[101,"x"],"q":1}]'],
    ["a quantity that is not a number", '[{"i":1,"m":[],"q":"2"}]'],
    ["a quantity of none", '[{"i":1,"m":[],"q":0}]'],
    ["something that is not a line", "[7, null, []]"],
  ])("leaves out a line with %s and keeps the good one beside it", (_, text) => {
    const withGood = `${text.slice(0, -1)},{"i":2,"m":[],"q":1}]`;
    expect(parse(withGood)).toEqual([filter]);
  });

  it("holds what it reads to the cart's own limits", () => {
    expect(parse('[{"i":2,"m":[],"q":100000}]')[0]?.qty).toBe(MAX_QTY);
    const many = JSON.stringify(Array.from({ length: 500 }, (_, n) => ({ i: n + 1, m: [], q: 1 })));
    expect(parse(many)).toHaveLength(MAX_LINES);
  });

  it("joins lines that are the same dish and choices", () => {
    expect(parse('[{"i":2,"m":[],"q":1},{"i":2,"m":[],"q":2}]')).toEqual([{ ...filter, qty: 3 }]);
  });
});
