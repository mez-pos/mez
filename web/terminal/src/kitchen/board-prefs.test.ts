import { describe, expect, it } from "vitest";
import type { KitchenTicketResponse } from "../../../shared/api-types";
import { arrivals, defaultPrefs, groupByTable, prefsKey, readPrefs, surfaceOf, ticketId, writePrefs } from "./board-prefs";

const ticket = (order: string, seq: number): KitchenTicketResponse => ({
  order_id: order,
  ticket_seq: seq,
  station: "kitchen",
  status: "fired",
  sent_at: "2026-09-28T12:00:00Z",
  channel: "counter",
  service: "dine_in",
  covers: 2,
  notes: [],
  lines: [],
});

const holding = (value: string | null) => ({ getItem: (key: string) => (key === prefsKey ? value : null) });

describe("readPrefs", () => {
  it("starts on paper, silent and ungrouped", () => {
    expect(readPrefs(holding(null))).toEqual(defaultPrefs);
    expect(readPrefs(null)).toEqual(defaultPrefs);
    expect(defaultPrefs).toEqual({ night: false, sound: false, groupByTable: false });
  });
  it("reads what was kept", () => {
    expect(readPrefs(holding('{"night":true,"sound":true,"groupByTable":true}'))).toEqual({ night: true, sound: true, groupByTable: true });
  });
  it("takes the default for anything missing or not a yes or no", () => {
    expect(readPrefs(holding('{"night":true,"sound":"loud"}'))).toEqual({ night: true, sound: false, groupByTable: false });
  });
  it("takes the defaults for what it cannot read", () => {
    for (const bad of ["not json", "[]", "7", '"night"']) expect(readPrefs(holding(bad))).toEqual(defaultPrefs);
    expect(
      readPrefs({
        getItem: () => {
          throw new Error("blocked");
        },
      }),
    ).toEqual(defaultPrefs);
  });
});

describe("writePrefs", () => {
  it("keeps the choices where readPrefs finds them", () => {
    const kept = new Map<string, string>();
    writePrefs({ setItem: (k, v) => void kept.set(k, v) }, { night: true, sound: false, groupByTable: true });
    expect(readPrefs({ getItem: (k) => kept.get(k) ?? null })).toEqual({ night: true, sound: false, groupByTable: true });
  });
  it("does not fail a device that blocks storage", () => {
    expect(() =>
      writePrefs(
        {
          setItem: () => {
            throw new Error("blocked");
          },
        },
        defaultPrefs,
      ),
    ).not.toThrow();
  });
});

describe("surfaceOf", () => {
  it("is ink at night and paper otherwise", () => {
    expect(surfaceOf({ ...defaultPrefs, night: true })).toBe("ink");
    expect(surfaceOf(defaultPrefs)).toBe("paper");
  });
});

describe("groupByTable", () => {
  it("brings an order's later tickets up beside its first", () => {
    const column = [ticket("ord_A", 3), ticket("ord_B", 3), ticket("ord_A", 6), ticket("ord_C", 3), ticket("ord_B", 5)];
    expect(groupByTable(column).map(ticketId)).toEqual(["ord_A-3", "ord_A-6", "ord_B-3", "ord_B-5", "ord_C-3"]);
  });
  it("leaves a column of single tickets as it was, and the column it was given untouched", () => {
    const column = [ticket("ord_B", 3), ticket("ord_A", 3)];
    expect(groupByTable(column).map(ticketId)).toEqual(["ord_B-3", "ord_A-3"]);
    expect(column.map(ticketId)).toEqual(["ord_B-3", "ord_A-3"]);
    expect(groupByTable([])).toEqual([]);
  });
});

describe("arrivals", () => {
  it("is nothing at the first look, however full the board", () => {
    expect(arrivals(null, [ticket("ord_A", 3)])).toEqual([]);
  });
  it("is the new tickets not seen before", () => {
    const seen = new Set(["ord_A-3"]);
    expect(arrivals(seen, [ticket("ord_A", 3), ticket("ord_A", 6), ticket("ord_B", 3)])).toEqual(["ord_A-6", "ord_B-3"]);
    expect(arrivals(seen, [ticket("ord_A", 3)])).toEqual([]);
  });
});
