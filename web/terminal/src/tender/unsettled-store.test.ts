import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearUnsettled, loadUnsettled, saveUnsettled } from "./unsettled-store";

interface Held {
  key: string;
}
const isHeld = (v: unknown): v is Held => typeof v === "object" && v !== null && typeof (v as Held).key === "string";

describe("the unsettled payment store", () => {
  const store = new Map<string, string>();
  beforeEach(() => {
    store.clear();
    vi.stubGlobal("sessionStorage", {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
      removeItem: (k: string) => void store.delete(k),
    });
  });
  afterEach(() => vi.unstubAllGlobals());

  it("keeps what is saved for an order, by order, until it is cleared", () => {
    saveUnsettled("o1", { key: "k1" });
    saveUnsettled("o2", { key: "k2" });
    expect(loadUnsettled("o1", isHeld)).toEqual({ key: "k1" });
    expect(loadUnsettled("o2", isHeld)).toEqual({ key: "k2" });
    clearUnsettled("o1");
    expect(loadUnsettled("o1", isHeld)).toBeNull();
    expect(loadUnsettled("o2", isHeld)).toEqual({ key: "k2" });
  });

  it("answers nothing for what does not parse or is not the shape", () => {
    store.set("mez.unsettled.o1", "{not json");
    expect(loadUnsettled("o1", isHeld)).toBeNull();
    store.set("mez.unsettled.o1", JSON.stringify({ other: 1 }));
    expect(loadUnsettled("o1", isHeld)).toBeNull();
  });

  it("tolerates storage that refuses", () => {
    const refuse = () => {
      throw new Error("refused");
    };
    vi.stubGlobal("sessionStorage", { getItem: refuse, setItem: refuse, removeItem: refuse });
    expect(() => saveUnsettled("o1", { key: "k" })).not.toThrow();
    expect(loadUnsettled("o1", isHeld)).toBeNull();
    expect(() => clearUnsettled("o1")).not.toThrow();
  });
});
