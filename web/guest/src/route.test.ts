import { describe, expect, it } from "vitest";
import { hashFor, tableCodeFrom, viewFrom } from "./route";

const code = "dpyyd3kcn5ev";

describe("tableCodeFrom", () => {
  it.each([
    ["on the venue's wifi", `/order/t/${code}`],
    ["through the relay", `/t/${code}`],
    ["with a trailing slash", `/order/t/${code}/`],
    ["under some other prefix", `/v/cafe/order/t/${code}`],
  ])("reads the code %s", (_, path) => {
    expect(tableCodeFrom(path)).toBe(code);
  });

  it.each([
    ["the app's root", "/order/"],
    ["nothing at all", "/"],
    ["a code with no /t/ before it", `/order/${code}`],
    ["something after the code", `/order/t/${code}/menu`],
    ["a code a character short", `/t/${code.slice(1)}`],
    ["a code a character over", `/t/${code}2`],
    ["capitals", `/t/${code.toUpperCase()}`],
    ["a letter the alphabet leaves out", `/t/${code.slice(0, 11)}o`],
    ["a digit the alphabet leaves out", `/t/${code.slice(0, 11)}1`],
    ["an encoded character", `/t/${code.slice(0, 9)}%2F`],
  ])("finds no code in %s", (_, path) => {
    expect(tableCodeFrom(path)).toBeNull();
  });
});

describe("the four screens", () => {
  it("is the menu or the cart for their hashes and the landing for anything else", () => {
    expect(viewFrom("#menu")).toBe("menu");
    expect(viewFrom("#cart")).toBe("cart");
    expect(viewFrom("")).toBe("landing");
    expect(viewFrom("#")).toBe("landing");
    expect(viewFrom("#checkout")).toBe("landing");
    expect(viewFrom("#MENU")).toBe("landing");
  });

  it.each(["landing", "menu", "cart", "order"] as const)("round-trips %s", (view) => {
    expect(viewFrom(hashFor(view))).toBe(view);
  });

  it("gives the landing no hash, so Back from the menu returns to it", () => {
    expect(hashFor("landing")).toBe("");
  });
});
