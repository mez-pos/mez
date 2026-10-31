import { describe, expect, it } from "vitest";
import type { PrintJobResponse } from "../../../shared/api-types";
import { actionsFor, detailOf, receiptState, trouble } from "./print-queue";

const job = (extra: Partial<PrintJobResponse>): PrintJobResponse => ({
  id: 1,
  kind: "receipt",
  label: "Receipt INV/26-27/000001",
  state: "queued",
  printer: "Counter",
  attempts: 0,
  max_attempts: 10,
  created_at: "2026-09-25T13:00:00Z",
  ...extra,
});

const now = new Date("2026-09-25T13:00:00Z");

describe("trouble", () => {
  it("counts the jobs that need a person", () => {
    const jobs = [job({ state: "done" }), job({ state: "failed" }), job({ state: "dead" }), job({ state: "queued" }), job({ state: "printing" })];
    expect(trouble(jobs)).toBe(2);
    expect(trouble([])).toBe(0);
  });
});

describe("actionsFor", () => {
  const cases: [PrintJobResponse["state"], boolean, boolean][] = [
    ["queued", false, true],
    ["printing", false, false],
    ["done", false, false],
    ["failed", true, true],
    ["dead", true, false],
  ];
  it.each(cases)("%s: retry %s, cancel %s", (state, retry, cancel) => {
    expect(actionsFor(job({ state }))).toEqual({ retry, cancel });
  });
});

describe("detailOf", () => {
  const cases: [string, Partial<PrintJobResponse>, string][] = [
    ["waiting", { state: "queued" }, "Waiting for Counter."],
    ["printing", { state: "printing" }, "Sending to Counter."],
    ["printed", { state: "done" }, "Printed on Counter."],
    [
      "failing, seconds away",
      { state: "failed", attempts: 3, last_error: "connection refused", next_attempt_at: "2026-09-25T13:00:08Z" },
      "Counter: connection refused. Try 3 of 10; next try in 8s.",
    ],
    [
      "failing, minutes away",
      { state: "failed", attempts: 9, last_error: "paper out", next_attempt_at: "2026-09-25T13:04:30Z" },
      "Counter: paper out. Try 9 of 10; next try in 5m.",
    ],
    ["failing, due", { state: "failed", attempts: 1, last_error: "x", next_attempt_at: "2026-09-25T12:59:00Z" }, "Counter: x. Try 1 of 10; trying again now."],
    [
      "no printer set up",
      { state: "dead", printer: "", last_error: "no printer is set up for this; set one up and retry" },
      // The server's words are for its log: a cashier is told who sets one up (TILL-19).
      "There is no printer for this. A manager adds one in the back office, under Printers; then Retry.",
    ],
    ["cancelled", { state: "dead", last_error: "cancelled by Asha" }, "Cancelled by Asha."],
  ];
  it.each(cases)("%s", (_name, extra, want) => {
    expect(detailOf(job(extra), now)).toBe(want);
  });
});

describe("receiptState (TILL-19)", () => {
  const no = "INV/26-27/000001";
  it("says nothing until the queue has the receipt", () => {
    expect(receiptState([], no)).toBeNull();
    expect(receiptState([job({ label: "Receipt INV/26-27/000009" })], no)).toBeNull();
  });
  it("says it printed, or is printing, on its printer", () => {
    expect(receiptState([job({ state: "done" })], no)).toEqual({ text: "The receipt printed on Counter.", failed: false });
    expect(receiptState([job({ state: "printing" })], no)?.text).toBe("The receipt is printing on Counter.");
  });
  it("says it is trying again while it fails, and that it did not print once it gives up", () => {
    expect(receiptState([job({ state: "failed" })], no)?.failed).toBe(false);
    expect(receiptState([job({ state: "dead", last_error: "paper out" })], no)).toEqual({
      text: "The receipt did not print. Retry it from the print queue.",
      failed: true,
    });
  });
  it("says there is no receipt printer, and who adds one", () => {
    const none = job({ state: "dead", printer: "", last_error: "no printer is set up for this; set one up and retry" });
    expect(receiptState([none], no)).toEqual({
      text: "No receipt printed: this venue has no receipt printer. A manager adds one in the back office.",
      failed: true,
    });
  });
  it("reads the newest job for the receipt, and not a duplicate's", () => {
    const jobs = [job({ id: 1, state: "dead", last_error: "x" }), job({ id: 2, state: "done" }), job({ id: 3, label: `Duplicate ${no}`, state: "dead" })];
    expect(receiptState(jobs, no)?.text).toBe("The receipt printed on Counter.");
  });
});
