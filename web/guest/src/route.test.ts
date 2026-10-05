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

describe("the two screens", () => {
  it("is the menu for the menu's hash and the landing for anything else", () => {
    expect(viewFrom("#menu")).toBe("menu");
    expect(viewFrom("")).toBe("landing");
    expect(viewFrom("#")).toBe("landing");
    expect(viewFrom("#cart")).toBe("landing");
  });

  it("round-trips", () => {
    expect(viewFrom(hashFor("menu"))).toBe("menu");
    expect(viewFrom(hashFor("landing"))).toBe("landing");
    expect(hashFor("landing")).toBe("");
  });
});
