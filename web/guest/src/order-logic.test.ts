import { describe, expect, it } from "vitest";
import { ApiError } from "../../shared/api";
import type { GuestOrderRequest, GuestRoundResponse } from "../../shared/api-types";
import { REMEMBER_MS, awaitsAnswer, drawKey, keyFor, parseKey, parsePlaced, refusalOf, statusWords, toRequest, type Placed } from "./order-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const request: GuestOrderRequest = {
  lines: [
    { item_id: 5, modifier_ids: [51, 60], qty: 2 },
    { item_id: 9, modifier_ids: [], qty: 1 },
  ],
};

const round: GuestRoundResponse = {
  id: "rnd_7K3M9QX2",
  status: "waiting",
  lines: [{ item_id: 5, name: "Flat white", qty: 2, choices: ["Oat"], total: inr(44000) }],
  subtotal: inr(44000),
};

describe("toRequest", () => {
  it("sends ids and counts, and no name or price", () => {
    const line = (id: number, mods: number[], qty: number) => ({
      item: { id, category_id: 1, name: "x", price: inr(100), modifier_groups: [] },
      modifiers: mods.map((m) => ({ modifier_id: m, name: "y", price: inr(10) })),
      qty,
    });
    const got = toRequest([line(5, [51, 60], 2), line(9, [], 1)]);
    expect(got).toEqual(request);
    expect(JSON.stringify(got)).not.toMatch(/name|price|minor/);
  });
});

describe("keyFor", () => {
  const draw = () => "fresh";
  it("draws a key for a cart never sent", () => {
    expect(keyFor(null, request, draw)).toEqual({ key: "fresh", body: JSON.stringify(request) });
  });
  it("keeps the key for the same cart sent again", () => {
    const first = keyFor(null, request, () => "first");
    expect(keyFor(first, request, draw)).toBe(first);
  });
  it("draws another when the cart has changed", () => {
    const first = keyFor(null, request, () => "first");
    const more: GuestOrderRequest = { lines: [...request.lines.slice(0, 1), { item_id: 9, modifier_ids: [], qty: 2 }] };
    expect(keyFor(first, more, draw).key).toBe("fresh");
    expect(keyFor(first, { lines: request.lines.slice(0, 1) }, draw).key).toBe("fresh");
  });
});

describe("parseKey", () => {
  it("reads back what was kept", () => {
    const kept = keyFor(null, request, () => "k");
    expect(parseKey(JSON.stringify(kept))).toEqual(kept);
  });
  it.each([
    ["nothing", null],
    ["not JSON", "{"],
    ["a list", "[]"],
    ["no key", '{"body":"x"}'],
    ["an empty key", '{"key":"","body":"x"}'],
    ["no body", '{"key":"k"}'],
  ])("is nothing for %s", (_name, text) => {
    expect(parseKey(text)).toBeNull();
  });
});

describe("drawKey", () => {
  it("is 32 hex characters, and not the same twice", () => {
    expect(drawKey()).toMatch(/^[0-9a-f]{32}$/);
    expect(drawKey()).not.toBe(drawKey());
  });
});

describe("parsePlaced", () => {
  const now = 1_800_000_000_000;
  const placed: Placed = { key: "k", request, round, at: now - 60_000 };
  const stored = (change: object = {}) => JSON.stringify({ ...placed, ...change });

  it("reads back an order placed this sitting", () => {
    expect(parsePlaced(stored(), now)).toEqual(placed);
  });
  it("forgets one from another day, or from a clock far ahead", () => {
    expect(parsePlaced(stored({ at: now - REMEMBER_MS - 1 }), now)).toBeNull();
    expect(parsePlaced(stored({ at: now + REMEMBER_MS + 1 }), now)).toBeNull();
    expect(parsePlaced(stored({ at: now - REMEMBER_MS }), now)).not.toBeNull();
  });
  it.each([
    ["nothing", null],
    ["not JSON", "placed"],
    ["no key", stored({ key: 7 })],
    ["no time", stored({ at: "now" })],
    ["no request", stored({ request: null })],
    ["a status the app does not know", stored({ round: { ...round, status: "eaten" } })],
    ["a line with no name", stored({ round: { ...round, lines: [{ qty: 1, choices: [], total: inr(1) }] } })],
    ["a choice that is not words", stored({ round: { ...round, lines: [{ ...round.lines[0], choices: [3] }] } })],
    ["a subtotal that is not whole", stored({ round: { ...round, subtotal: { minor: 1.5, currency: "INR" } } })],
  ])("is nothing for %s", (_name, text) => {
    expect(parsePlaced(text, now)).toBeNull();
  });
});

describe("awaitsAnswer", () => {
  it("is true only while waiting", () => {
    const of = (status: GuestRoundResponse["status"]): Placed => ({ key: "k", request, round: { ...round, status }, at: 0 });
    expect(awaitsAnswer(of("waiting"))).toBe(true);
    expect(awaitsAnswer(of("sent"))).toBe(false);
    expect(awaitsAnswer(of("declined"))).toBe(false);
    expect(awaitsAnswer(null)).toBe(false);
  });
});

describe("statusWords", () => {
  it("names the venue while waiting, and never says an unaccepted order is being made", () => {
    const waiting = statusWords("waiting", "Demo Cafe");
    expect(waiting.title).toBe("Waiting for Demo Cafe to accept your order");
    expect(waiting.line).toContain("Nothing is being made yet");
    expect(statusWords("declined", "Demo Cafe").line).toContain("ask a member of staff");
    expect(statusWords("sent", "Demo Cafe").brief).toBe("Sent to the kitchen");
  });
});

describe("refusalOf", () => {
  it("uses the server's words where it wrote them for a guest, and reads the menu again when a dish moved", () => {
    expect(refusalOf(new ApiError("dish_unavailable", "Brownie is no longer available.", 409))).toEqual({
      text: "Brownie is no longer available.",
      readMenu: true,
    });
    expect(refusalOf(new ApiError("choices_invalid", "The choices for Latte have changed.", 409)).readMenu).toBe(true);
    expect(refusalOf(new ApiError("no_open_shift", "Ordering is closed right now. Ask a member of staff.", 409))).toEqual({
      text: "Ordering is closed right now. Ask a member of staff.",
      readMenu: false,
    });
  });
  it("says so when the kitchen cannot be reached, by the phone or by the relay", () => {
    expect(refusalOf(new ApiError("network", "offline")).text).toContain("Can't reach the kitchen");
    expect(refusalOf(new ApiError("bad_response", "bad gateway", 502)).text).toContain("Can't reach the kitchen");
    expect(refusalOf(new ApiError("rate_limited", "slow down", 429)).text).toContain("Wait a moment");
  });
  it("never shows a guest the server's insides", () => {
    for (const err of [new ApiError("internal", "pq: deadlock at seq 4", 500), new Error("boom"), "boom"]) {
      const { text } = refusalOf(err);
      expect(text).toContain("order at the counter");
      expect(text).not.toMatch(/seq|boom/);
    }
  });
});
