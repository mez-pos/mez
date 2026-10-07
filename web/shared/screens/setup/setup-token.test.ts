import { describe, expect, it } from "vitest";
import type { KeyValue } from "../../auth/storage";
import { forgetSetupToken, setupHeaders, takeSetupToken, tokenIn, type TokenPage } from "./setup-token";

const token = "0123456789abcdef0123456789abcdef";
const other = "ffffffffffffffffffffffffffffffff";

class MapStorage implements KeyValue {
  readonly map = new Map<string, string>();
  getItem = (k: string) => this.map.get(k) ?? null;
  setItem = (k: string, v: string) => void this.map.set(k, v);
  removeItem = (k: string) => void this.map.delete(k);
}

const blocked: KeyValue = {
  getItem: () => {
    throw new Error("blocked");
  },
  setItem: () => {
    throw new Error("blocked");
  },
  removeItem: () => {
    throw new Error("blocked");
  },
};

function page(hash: string, storage: KeyValue | null = new MapStorage()) {
  let cleared = 0;
  const p: TokenPage = { hash, clearHash: () => void cleared++, storage };
  return { p, cleared: () => cleared };
}

describe("tokenIn", () => {
  it.each<[string, string | null]>([
    [`#token=${token}`, token],
    [`token=${token}`, token],
    [`#a=1&token=${token}`, token],
    ["", null],
    ["#", null],
    ["#menu", null],
    ["#token=", null],
    ["#token=abc", null],
    [`#token=${token.toUpperCase()}`, null],
    [`#token=${token}0`, null],
    ["#token=<script>alert(1)</script>", null],
  ])("%s", (hash, want) => {
    expect(tokenIn(hash)).toBe(want);
  });
});

describe("takeSetupToken", () => {
  it("takes the token out of the address, keeps it, and holds it after a reload", () => {
    const storage = new MapStorage();
    const first = page(`#token=${token}`, storage);
    expect(takeSetupToken(first.p)).toBe(token);
    expect(first.cleared()).toBe(1);

    const reloaded = page("", storage);
    expect(takeSetupToken(reloaded.p)).toBe(token);
    expect(reloaded.cleared()).toBe(0);
  });

  it("holds none where none arrived and none was kept", () => {
    expect(takeSetupToken(page("").p)).toBeNull();
    expect(takeSetupToken(page("#menu").p)).toBeNull();
  });

  it("takes a newer token over the one it kept", () => {
    const storage = new MapStorage();
    takeSetupToken(page(`#token=${token}`, storage).p);
    expect(takeSetupToken(page(`#token=${other}`, storage).p)).toBe(other);
    expect(takeSetupToken(page("", storage).p)).toBe(other);
  });

  it("leaves alone a fragment that is not a token, and something kept that is not one", () => {
    const storage = new MapStorage();
    storage.setItem("mez.setup.token", "not a token");
    const p = page("#token=nope", storage);
    expect(takeSetupToken(p.p)).toBeNull();
    expect(p.cleared()).toBe(0);
  });

  it("still holds the token for this page where storage is blocked or missing", () => {
    expect(takeSetupToken(page(`#token=${token}`, blocked).p)).toBe(token);
    expect(takeSetupToken(page(`#token=${token}`, null).p)).toBe(token);
    expect(takeSetupToken(page("", blocked).p)).toBeNull();
  });
});

describe("forgetSetupToken", () => {
  it("forgets it, and does not mind having nowhere to look", () => {
    const storage = new MapStorage();
    takeSetupToken(page(`#token=${token}`, storage).p);
    forgetSetupToken(storage);
    expect(takeSetupToken(page("", storage).p)).toBeNull();
    expect(() => forgetSetupToken(blocked)).not.toThrow();
    expect(() => forgetSetupToken(null)).not.toThrow();
  });
});

describe("setupHeaders", () => {
  it("carries the token when there is one", () => {
    expect(setupHeaders(token)).toEqual({ "X-Setup-Token": token });
    expect(setupHeaders(null)).toEqual({});
  });
});
