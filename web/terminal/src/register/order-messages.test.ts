import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import { approvalFailure, failureOf } from "./order-messages";

const wrapped = (e: ApiError) => new Error(failureOf(e), { cause: e });

describe("approvalFailure", () => {
  const cases: [string, ApiError, string][] = [
    ["a wrong PIN", new ApiError("invalid_pin", "that staff member or PIN is not right", 401), "That PIN isn't right. Try again."],
    ["someone without the permission", new ApiError("forbidden", "that staff member cannot authorise this", 403), "Asha can't approve this. Choose a manager."],
    ["a lockout", new ApiError("locked_out", "too many failed attempts", 429), "Too many wrong PINs. Try again in a few minutes."],
    ["an order refusal", new ApiError("line_not_active", "db: line is voided", 409), "That line has been voided or comped."],
    ["no server", new ApiError("network", "Failed to fetch"), "Not sent: the server can't be reached."],
  ];
  it.each(cases)("words %s, never the server's text", (_, e, want) => {
    expect(approvalFailure(wrapped(e), "Asha")).toBe(want);
    expect(approvalFailure(e, "Asha")).toBe(want);
  });

  it("passes the queue's own words through", () => {
    expect(approvalFailure(new Error("There is no ticket to change."), "Asha")).toBe("There is no ticket to change.");
  });
});
