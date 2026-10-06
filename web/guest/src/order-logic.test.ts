import { describe, expect, it } from "vitest";
import { ApiError } from "../../shared/api";
import type { GuestOrderRequest, GuestRoundProgress, GuestRoundResponse } from "../../shared/api-types";
import {
  ASK_AGAIN_MS,
  REMEMBER_MS,
  TRACK_MS,
  TRACK_READY_MS,
  askAfterMs,
  drawKey,
  dueOf,
  keyFor,
  parseKey,
  kitchenSteps,
  orderGone,
  orderWords,
  parsePlaced,
  pickupCodeOf,
  progressWords,
  refusalOf,
  sentNote,
  statusWords,
  stepIndex,
  toRequest,
  withRound,
  type Placed,
} from "./order-logic";

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
  const placed: Placed = { key: "k", request, round, at: now - 60_000, paidFirst: false, tracked: false };
  const stored = (change: object = {}) => JSON.stringify({ ...placed, ...change });

  it("reads back an order placed this sitting", () => {
    expect(parsePlaced(stored(), now)).toEqual(placed);
  });
  it("reads an order written before paying first, or tracking, was known as one with neither", () => {
    const { paidFirst: _dropped, tracked: _also, ...older } = placed;
    expect(parsePlaced(JSON.stringify(older), now)).toEqual(placed);
  });
  it("keeps how far the kitchen had come, and that it was asked", () => {
    const sent = { ...round, status: "sent" as const, progress: "preparing" as const };
    const back = parsePlaced(stored({ round: sent, tracked: true }), now);
    expect(back?.round.progress).toBe("preparing");
    expect(back?.tracked).toBe(true);
    // An order that is not sent was never tracked, whatever storage says.
    expect(parsePlaced(stored({ tracked: true }), now)?.tracked).toBe(false);
  });
  it("keeps that an order was held for its payment, and what it asks", () => {
    const held = { ...round, status: "unpaid" as const, due: inr(58800) };
    const back = parsePlaced(stored({ round: held }), now);
    expect(back?.paidFirst).toBe(true);
    expect(back && dueOf(back.round)).toEqual(inr(58800));
    // Sent since, it is still an order that was paid first.
    expect(parsePlaced(stored({ round, paidFirst: true }), now)?.paidFirst).toBe(true);
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
    ["an amount to pay that is not money", stored({ round: { ...round, status: "unpaid", due: "588.00" } })],
    ["a step the app does not know", stored({ round: { ...round, status: "sent", progress: "eaten" } })],
  ])("is nothing for %s", (_name, text) => {
    expect(parsePlaced(text, now)).toBeNull();
  });
});

describe("askAfterMs", () => {
  const of = (status: GuestRoundResponse["status"], progress?: GuestRoundProgress, tracked = false): Placed => ({
    key: "k",
    request,
    round: progress === undefined ? { ...round, status } : { ...round, status, progress },
    at: 0,
    paidFirst: false,
    tracked,
  });
  it("asks while an order waits for staff or for its payment", () => {
    expect(askAfterMs(of("waiting"))).toBe(ASK_AGAIN_MS);
    expect(askAfterMs(of("unpaid"))).toBe(ASK_AGAIN_MS);
  });
  it("asks nothing of an order that was declined, or of none", () => {
    expect(askAfterMs(of("declined"))).toBeNull();
    expect(askAfterMs(null)).toBeNull();
  });
  it("asks soon of an order just sent, to learn where it is", () => {
    expect(askAfterMs(of("sent"))).toBe(ASK_AGAIN_MS);
  });
  it("follows the kitchen while it has more to do, and slows once it is ready", () => {
    expect(askAfterMs(of("sent", "accepted", true))).toBe(TRACK_MS);
    expect(askAfterMs(of("sent", "sent", true))).toBe(TRACK_MS);
    expect(askAfterMs(of("sent", "preparing", true))).toBe(TRACK_MS);
    // A recall takes ready back, so it is still asked, and seldom.
    expect(askAfterMs(of("sent", "ready", true))).toBe(TRACK_READY_MS);
    expect(TRACK_READY_MS).toBeGreaterThan(TRACK_MS);
  });
  it("stops when nothing more is coming", () => {
    expect(askAfterMs(of("sent", "cancelled", true))).toBeNull();
    // Asked, and the venue says nothing of the kitchen: sent before orders were tracked.
    expect(askAfterMs(of("sent", undefined, true))).toBeNull();
  });
});

describe("withRound", () => {
  const before = { key: "k", request, at: 0 };
  it("marks an order tracked only by an answer from its own address, once it is sent", () => {
    const sent = { ...round, status: "sent" as const };
    expect(withRound(before, sent).tracked).toBe(false);
    expect(withRound(before, sent, true).tracked).toBe(true);
    expect(withRound(before, round, true).tracked).toBe(false);
    expect(withRound({ ...before, tracked: true }, sent).tracked).toBe(true);
  });
});

describe("the kitchen's steps", () => {
  const all: GuestRoundProgress[] = ["accepted", "sent", "preparing", "ready", "cancelled"];
  it("has words for every step the server can answer", () => {
    for (const p of all) {
      const w = progressWords(p);
      expect(w.title).not.toBe("");
      expect(w.line).not.toBe("");
      expect(w.brief).not.toBe("");
    }
  });
  it("never promises a time", () => {
    for (const p of all) expect(`${progressWords(p).title} ${progressWords(p).line}`).not.toMatch(/minute|soon|shortly|\d/);
  });
  it("places an order on the row, and off it when it is on none of the steps", () => {
    expect(kitchenSteps.map((s) => s.step)).toEqual(["sent", "preparing", "ready"]);
    expect(stepIndex("sent")).toBe(0);
    expect(stepIndex("ready")).toBe(2);
    expect(stepIndex("accepted")).toBe(-1);
    expect(stepIndex("cancelled")).toBe(-1);
    expect(stepIndex(undefined)).toBe(-1);
  });
  it("says the kitchen's step for a sent order, and the order's status for any other", () => {
    const of = (status: GuestRoundResponse["status"], progress?: GuestRoundProgress, paidFirst = false) => ({
      round: progress === undefined ? { ...round, status } : { ...round, status, progress },
      paidFirst,
    });
    expect(orderWords(of("sent", "preparing"), "Demo Cafe").title).toBe("Your order is being made");
    expect(orderWords(of("sent", "ready"), "Demo Cafe").brief).toBe("Ready");
    // Sent with nothing known of the kitchen: the title, and no line to repeat the note under it.
    expect(orderWords(of("sent"), "Demo Cafe")).toEqual({ ...statusWords("sent", "Demo Cafe"), line: "" });
    expect(orderWords(of("waiting", "ready"), "Demo Cafe")).toEqual(statusWords("waiting", "Demo Cafe"));
    expect(orderWords(of("sent", "sent", true), "Demo Cafe").title).toBe("Paid, and with the kitchen");
  });
  it("tells an order that paid first nothing of paying on leaving", () => {
    expect(sentNote(false)).toContain("You pay when you are ready to leave");
    expect(sentNote(true)).not.toContain("pay");
  });
});

describe("statusWords", () => {
  it("names the venue while waiting, and never says an unaccepted order is being made", () => {
    const waiting = statusWords("waiting", "Demo Cafe");
    expect(waiting.title).toBe("Waiting for Demo Cafe to accept your order");
    expect(waiting.line).toContain("Nothing is being made yet");
    expect(statusWords("declined", "Demo Cafe").line).toContain("ask a member of staff");
    expect(statusWords("sent", "Demo Cafe").brief).toBe("Sent to the kitchen");
    expect(statusWords("unpaid", "Demo Cafe").line).toContain("until this order is paid for");
  });
  it("does not tell a guest who paid first that they pay on leaving", () => {
    expect(statusWords("sent", "Demo Cafe").line).toContain("You pay when you are ready to leave");
    const paid = statusWords("sent", "Demo Cafe", true);
    expect(paid.title).toBe("Paid, and with the kitchen");
    expect(paid.line).not.toContain("pay");
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

describe("withRound and dueOf", () => {
  const base = { key: "k", request, at: 5 };
  it("marks an order held for its payment, and keeps the mark once it is sent", () => {
    const held = withRound(base, { ...round, status: "unpaid", due: inr(58800) });
    expect(held.paidFirst).toBe(true);
    const sent = withRound(held, { ...round, status: "sent" });
    expect(sent.paidFirst).toBe(true);
    expect(sent.round.status).toBe("sent");
    expect(sent.key).toBe("k");
  });
  it("leaves a request for staff unmarked", () => {
    expect(withRound(base, { ...round, status: "waiting" }).paidFirst).toBe(false);
    expect(withRound(withRound(base, { ...round, status: "waiting" }), { ...round, status: "sent" }).paidFirst).toBe(false);
  });
  it("asks for the server's figure, and only of an unpaid order", () => {
    expect(dueOf({ ...round, status: "unpaid", due: inr(58800) })).toEqual(inr(58800));
    expect(dueOf({ ...round, status: "unpaid" })).toBeNull();
    expect(dueOf({ ...round, status: "unpaid", due: inr(0) })).toBeNull();
    expect(dueOf({ ...round, status: "sent", due: inr(58800) })).toBeNull();
  });
});

describe("an order to collect", () => {
  const all: GuestRoundProgress[] = ["accepted", "sent", "preparing", "ready", "cancelled"];
  const now = 1_800_000_000_000;
  const sent: GuestRoundResponse = { ...round, status: "sent", progress: "ready", pickup_code: "B7K" };
  const stored = (r: object) => JSON.stringify({ key: "k", request, round: r, at: now - 60_000, paidFirst: true, tracked: true });

  it("says nothing of a table, a bill, or of anything being brought", () => {
    for (const p of all) {
      const w = progressWords(p, true);
      expect(`${w.title} ${w.line} ${w.brief}`).not.toMatch(/table|bill|bring|member of staff/i);
      // The step is the kitchen's, whoever is waiting for it.
      expect(w.title).toBe(progressWords(p).title);
      expect(w.brief).toBe(progressWords(p).brief);
    }
    expect(sentNote(true, true)).not.toMatch(/table|bill|order more/i);
    expect(sentNote(true, true)).toContain("code");
  });

  it("sends a ready order's guest to the counter", () => {
    expect(progressWords("ready", true).line).toContain("counter");
    expect(orderWords({ round: sent, paidFirst: true }, "Demo Cafe", true).line).toContain("counter");
    // A table's is still brought to it.
    expect(orderWords({ round: sent, paidFirst: true }, "Demo Cafe").line).toContain("your table");
  });

  it("is paid, and with the kitchen, until a cook starts it", () => {
    expect(orderWords({ round: { ...sent, progress: "sent" }, paidFirst: true }, "Demo Cafe", true).title).toBe("Paid, and with the kitchen");
  });

  it.each(["B7K", "2222", "M4Q9"])("reads the pickup code %s", (code) => {
    expect(pickupCodeOf({ ...sent, pickup_code: code })).toBe(code);
  });

  it.each([undefined, "", "b7k", "B7", "B7K2X", "B 7", "<b>"])("reads no pickup code in %s", (code) => {
    expect(pickupCodeOf({ ...round, ...(code === undefined ? {} : { pickup_code: code }) })).toBeNull();
  });

  it("keeps the code with the order it remembers", () => {
    expect(parsePlaced(stored(sent), now)?.round.pickup_code).toBe("B7K");
  });

  it.each([7, "<b>x</b>", "B7K2X"])("does not trust a stored code of %s", (code) => {
    expect(parsePlaced(stored({ ...sent, pickup_code: code }), now)).toBeNull();
  });

  it("stops asking about an order the venue no longer finds, and not for a failure that may pass", () => {
    expect(orderGone(new ApiError("request_not_found", "That order is not one placed to collect here.", 404))).toBe(true);
    expect(orderGone(new ApiError("pickup_code_unknown", "This code does not lead anywhere.", 404))).toBe(true);
    expect(orderGone(new ApiError("network", "offline"))).toBe(false);
    expect(orderGone(new ApiError("rate_limited", "wait", 429))).toBe(false);
    expect(orderGone(new ApiError("internal", "internal error", 500))).toBe(false);
    expect(orderGone(new Error("anything"))).toBe(false);
  });

  it("uses the server's words when the venue stops taking orders to collect", () => {
    const off = "This venue is not taking orders to collect online. Order at the counter.";
    expect(refusalOf(new ApiError("pickup_off", off, 409))).toEqual({ text: off, readMenu: false });
    expect(refusalOf(new ApiError("pickup_code_unknown", "This code does not lead anywhere.", 404)).text).toBe("This code does not lead anywhere.");
  });
});
