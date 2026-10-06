import { describe, expect, it } from "vitest";
import type { GuestBillResponse, Money } from "../../shared/api-types";
import { askingOf, canSplit, restLabel, newPhoneName, peopleRange, phoneNameOf, restOf, startingChoice, stepPeople } from "./split-logic";

const inr = (minor: number): Money => ({ minor, currency: "INR" });
const phone = "p_0123456789abcdef";

type SplitBill = Pick<GuestBillResponse, "split" | "balance_due" | "online">;

function bill(over: Partial<NonNullable<GuestBillResponse["split"]>> = {}): SplitBill {
  return {
    online: true,
    balance_due: inr(44000),
    split: {
      rest: inr(44000),
      unclaimed: inr(44000),
      parts: [
        { people: 2, amount: inr(22000) },
        { people: 3, amount: inr(14667) },
        { people: 4, amount: inr(11000) },
      ],
      ...over,
    },
  };
}

describe("the phone's name", () => {
  it("is one the server takes", () => {
    const name = newPhoneName(new Uint8Array([0, 1, 254, 255, 16, 32, 48, 64]));
    expect(name).toBe("p_0001feff10203040");
    expect(phoneNameOf(name)).toBe(name);
  });
  it("is not trusted from storage", () => {
    for (const bad of [null, "", "short", "has a space in it", "<script>alert(1)</script>", "x".repeat(65)]) {
      expect(phoneNameOf(bad)).toBeNull();
    }
  });
});

describe("askingOf", () => {
  it("asks for the rest with no body, and says the server's figure", () => {
    expect(askingOf(bill({ rest: inr(4000) }), startingChoice, phone)).toEqual({ ok: true, amount: inr(4000), request: null });
  });
  it("falls back to what is owed on a bill that says nothing of splitting", () => {
    const old: SplitBill = { online: true, balance_due: inr(44000) };
    expect(restOf(old)).toEqual(inr(44000));
    expect(askingOf(old, { way: "even", people: 3, typed: "" }, phone)).toEqual({ ok: true, amount: inr(44000), request: null });
  });
  it("offers no rest when others are paying all of it", () => {
    const got = askingOf(bill({ rest: inr(0) }), startingChoice, phone);
    expect(got.ok).toBe(false);
  });
  it("asks for an equal part by how many people, with the server's figure on the button", () => {
    expect(askingOf(bill(), { way: "even", people: 3, typed: "" }, phone)).toEqual({
      ok: true,
      amount: inr(14667),
      request: { phone, people: 3 },
    });
  });
  it("offers no part the server did not", () => {
    expect(askingOf(bill(), { way: "even", people: 9, typed: "" }, phone).ok).toBe(false);
    expect(askingOf(bill({ parts: [{ people: 2, amount: inr(0) }] }), { way: "even", people: 2, typed: "" }, phone).ok).toBe(false);
  });
  it("asks for a typed figure, in the bill's currency", () => {
    expect(askingOf(bill(), { way: "amount", people: 2, typed: "250.50" }, phone)).toEqual({
      ok: true,
      amount: inr(25050),
      request: { phone, amount: inr(25050) },
    });
  });
  it("refuses a figure that is nothing, not a figure, or more than is left, and says why", () => {
    const refused = (typed: string, b = bill()) => {
      const got = askingOf(b, { way: "amount", people: 2, typed }, phone);
      return got.ok ? "" : got.why;
    };
    expect(refused("")).toContain("Type the amount");
    expect(refused("0")).toContain("not an amount");
    expect(refused("-5")).toContain("not an amount");
    expect(refused("lots")).toContain("not an amount");
    expect(refused("440.01")).toContain("more than");
    expect(refused("50", bill({ unclaimed: inr(4000) }))).toContain("₹40.00");
    expect(refused("440")).toBe("");
  });
  it("is the rest for a phone with no name", () => {
    expect(askingOf(bill(), { way: "even", people: 3, typed: "" }, null)).toEqual({ ok: true, amount: inr(44000), request: null });
  });
});

describe("splitting", () => {
  it("is offered where the bill can be paid online, the phone has a name and something is left for it", () => {
    expect(canSplit(bill(), phone)).toBe(true);
    expect(canSplit(bill(), null)).toBe(false);
    expect(canSplit(bill({ unclaimed: inr(0) }), phone)).toBe(false);
    expect(canSplit({ ...bill(), online: false }, phone)).toBe(false);
    expect(canSplit({ online: true }, phone)).toBe(false);
  });
  it("calls the first choice the rest once some of the bill is paid or being paid", () => {
    expect(restLabel({ ...bill(), total: inr(44000) })).toBe("All of it");
    expect(restLabel({ ...bill({ rest: inr(4000) }), total: inr(44000) })).toBe("The rest");
    expect(restLabel({ balance_due: inr(20000), total: inr(44000) })).toBe("The rest");
  });
  it("steps people within what the server offers", () => {
    expect(peopleRange(bill())).toEqual({ min: 2, max: 4 });
    expect(stepPeople(bill(), 2, -1)).toBe(2);
    expect(stepPeople(bill(), 2, 1)).toBe(3);
    expect(stepPeople(bill(), 4, 1)).toBe(4);
    expect(stepPeople({}, 3, 1)).toBe(3);
  });
});
