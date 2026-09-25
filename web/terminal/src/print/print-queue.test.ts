import { describe, expect, it } from "vitest";
import type { PrintJobResponse } from "../../../shared/api-types";
import { actionsFor, detailOf, trouble } from "./print-queue";

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
      "No printer is set up for this; set one up and retry.",
    ],
    ["cancelled", { state: "dead", last_error: "cancelled by Asha" }, "Cancelled by Asha."],
  ];
  it.each(cases)("%s", (_name, extra, want) => {
    expect(detailOf(job(extra), now)).toBe(want);
  });
});
