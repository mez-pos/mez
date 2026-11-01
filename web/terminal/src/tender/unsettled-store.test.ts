import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearUnsettled, loadUnsettled, saveUnsettled, unsettledHold, type Kept } from "./unsettled-store";

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

  describe("a screen's hold", () => {
    type Payment = { amount: number };
    const isKept = (v: unknown): v is Kept<Payment, string> => typeof v === "object" && v !== null && "unsettled" in v;

    it("shows the payment and keeps it with its attempt", () => {
      const shown: (Payment | null)[] = [];
      const { hold } = unsettledHold<Payment, string>("o1", (u) => shown.push(u));
      hold({ amount: 500 }, "key-1");
      expect(shown).toEqual([{ amount: 500 }]);
      expect(loadUnsettled("o1", isKept)).toEqual({ attempt: "key-1", unsettled: { amount: 500 } });
    });

    // TBS-20: the waiter's own copy of release called itself, so every
    // recorded payment ended on "Maximum call stack size exceeded".
    it("lets go once: shows nothing and forgets what was kept", () => {
      const shown: (Payment | null)[] = [];
      const { hold, release } = unsettledHold<Payment, string>("o1", (u) => shown.push(u));
      hold({ amount: 500 }, null);
      release();
      expect(shown).toEqual([{ amount: 500 }, null]);
      expect(loadUnsettled("o1", isKept)).toBeNull();
    });

    it("lets go when nothing was held, as a recorded payment does", () => {
      const shown: (Payment | null)[] = [];
      const { release } = unsettledHold<Payment, string>("o1", (u) => shown.push(u));
      expect(() => release()).not.toThrow();
      expect(shown).toEqual([null]);
    });

    // Each screen once kept its own copy of the hold, and one broke (TBS-20).
    it("is the only hold the screens have", () => {
      const screens = import.meta.glob(["./PayScreen.tsx", "../waiter/WaiterBill.tsx"], { query: "?raw", import: "default", eager: true }) as Record<
        string,
        string
      >;
      expect(Object.keys(screens)).toHaveLength(2);
      for (const [file, source] of Object.entries(screens)) {
        expect(source, file).toContain("unsettledHold<");
        expect(source, file).not.toMatch(/\b(saveUnsettled|clearUnsettled)\(/);
      }
    });

    it("holds each order apart", () => {
      unsettledHold<Payment, string>("o1", () => undefined).hold({ amount: 1 }, "k1");
      unsettledHold<Payment, string>("o2", () => undefined).hold({ amount: 2 }, "k2");
      unsettledHold<Payment, string>("o1", () => undefined).release();
      expect(loadUnsettled("o1", isKept)).toBeNull();
      expect(loadUnsettled("o2", isKept)).toEqual({ attempt: "k2", unsettled: { amount: 2 } });
    });
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
