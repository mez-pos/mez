import { describe, expect, it } from "vitest";
import { ApiError } from "../../shared/api";
import { needsSession, recoveryOf, sessionOf, sessionStorageKey, tableCodeOfPath } from "./session-logic";

const good = "gst_" + "A".repeat(42) + "_";

describe("sessionOf", () => {
  it.each<[string, unknown, string | null]>([
    ["a session", good, good],
    ["with - and _ in it", "gst_" + "-_".repeat(21) + "z", "gst_" + "-_".repeat(21) + "z"],
    ["nothing", null, null],
    ["undefined", undefined, null],
    ["a number", 42, null],
    ["an empty string", "", null],
    ["one character short", good.slice(0, -1), null],
    ["one character long", good + "A", null],
    ["another prefix", "nfd_" + "A".repeat(43), null],
    ["a character base64url has not", "gst_" + "A".repeat(42) + "+", null],
    ["JSON around it", JSON.stringify(good), null],
  ])("reads %s", (_, value, want) => {
    expect(sessionOf(value)).toBe(want);
  });
});

describe("needsSession", () => {
  it("is the venue's refusal of the session", () => {
    expect(needsSession(new ApiError("guest_session_required", "Scan the code on your table again to continue.", 401))).toBe(true);
  });
  it.each([
    ["another refusal", new ApiError("table_code_unknown", "no", 404)],
    ["a lost connection", new ApiError("network", "offline")],
    ["something else thrown", new Error("guest_session_required")],
    ["nothing", null],
  ])("is not %s", (_, error) => {
    expect(needsSession(error)).toBe(false);
  });
});

describe("tableCodeOfPath", () => {
  it.each<[string, string | null]>([
    ["/order/api/t/abcdefghjkmn", "abcdefghjkmn"],
    ["/order/api/t/abcdefghjkmn/bill?phone=x", "abcdefghjkmn"],
    ["/order/api/t/abcdefghjkmn?x=1", "abcdefghjkmn"],
    ["/order/api/t/abcdefghjkmn/orders/rnd_1/charge", "abcdefghjkmn"],
    ["/order/api/p/abcdefghjkmn/orders", null],
    ["/order/api/photos/x.jpg", null],
    ["/order/api/t/", null],
    ["/api/v1/orders", null],
    ["/order/api/t/%E0%A4%A", null],
  ])("finds the table in %s", (path, want) => {
    expect(tableCodeOfPath(path)).toBe(want);
  });
});

describe("recoveryOf", () => {
  it.each<[string, string | null, boolean, string]>([
    ["a new session, reading the bill", good, false, "again"],
    ["a new session, ordering", good, true, "again"],
    ["none, ordering: it asks to join", null, true, "again"],
    ["none, reading the bill: ask staff", null, false, "ask_staff"],
  ])("with %s", (_, fresh, ordering, want) => {
    expect(recoveryOf(fresh, ordering)).toBe(want);
  });
});

describe("sessionStorageKey", () => {
  it("is kept by table code", () => {
    expect(sessionStorageKey("abcdefghjkmn")).toBe("mez.session.abcdefghjkmn");
  });
});
