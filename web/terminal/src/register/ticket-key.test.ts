import { describe, expect, it } from "vitest";
import { newKey, ticketMemory } from "./ticket-key";

function memoryStorage() {
  const map = new Map<string, string>();
  return {
    getItem: (k: string) => map.get(k) ?? null,
    setItem: (k: string, v: string) => void map.set(k, v),
    removeItem: (k: string) => void map.delete(k),
    map,
  };
}

describe("newKey", () => {
  it("is 32 hex characters from 16 random bytes", () => {
    expect(newKey((b) => b.fill(0xab))).toBe("ab".repeat(16));
    expect(newKey()).toMatch(/^[0-9a-f]{32}$/);
    expect(newKey()).not.toBe(newKey());
  });
});

describe("ticketMemory", () => {
  it("remembers and forgets the order id", () => {
    const s = memoryStorage();
    const m = ticketMemory(s);
    expect(m.read()).toBeNull();
    m.write("ord_7K3M9QX2");
    expect(m.read()).toBe("ord_7K3M9QX2");
    m.write(null);
    expect(m.read()).toBeNull();
  });

  it("refuses a stored value that is not an order id", () => {
    const s = memoryStorage();
    s.map.set("nofiltr.terminal.ticket", "../../api/v1/staff");
    expect(ticketMemory(s).read()).toBeNull();
  });

  it("works with no storage, and with storage that throws", () => {
    expect(ticketMemory(null).read()).toBeNull();
    const broken = {
      getItem: () => {
        throw new Error("denied");
      },
      setItem: () => {
        throw new Error("denied");
      },
      removeItem: () => {
        throw new Error("denied");
      },
    };
    const m = ticketMemory(broken);
    expect(() => m.write("ord_1")).not.toThrow();
    expect(m.read()).toBeNull();
  });
});
