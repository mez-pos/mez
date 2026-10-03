import { describe, expect, it } from "vitest";
import type { HistoryResponse } from "../../../shared/api-types";
import { historyKey, withNames } from "./HistorySheet";
import { orderQueryKey } from "./use-ticket";

const history = (timeZone: string): HistoryResponse => ({
  events: [],
  staff: [
    { id: 1, name: "Asha" },
    { id: 2, name: "Ravi" },
  ],
  devices: [{ id: 3, name: "Counter 1" }],
  time_zone: timeZone,
});

describe("withNames", () => {
  it("turns the name lists into lookups", () => {
    const named = withNames(history("Asia/Kolkata"));
    expect(named.staff.get(2)).toBe("Ravi");
    expect(named.devices.get(3)).toBe("Counter 1");
    expect(named.staff.get(9)).toBeUndefined();
    expect(named.timeZone).toBe("Asia/Kolkata");
  });

  it("falls back to the device's zone for one the browser does not know", () => {
    expect(withNames(history("Mars/Olympus")).timeZone).toBeUndefined();
  });
});

describe("historyKey", () => {
  it("sits under the order's key, so the order's invalidation refreshes it", () => {
    const order = orderQueryKey("ord_1");
    expect(historyKey("ord_1").slice(0, order.length)).toEqual([...order]);
  });
});
