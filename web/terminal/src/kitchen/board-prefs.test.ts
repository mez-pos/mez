import { describe, expect, it } from "vitest";
import type { KitchenTicketResponse } from "../../../shared/api-types";
import {
  arrivals,
  defaultPrefs,
  groupByTable,
  newlyVoided,
  noArrivals,
  noteArrivals,
  prefsKey,
  readPrefs,
  surfaceOf,
  ticketId,
  voidedKeys,
  writePrefs,
} from "./board-prefs";

const ticket = (order: string, seq: number): KitchenTicketResponse => ({
  order_id: order,
  ticket_seq: seq,
  round: 1,
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
  it("starts on night, with the chime on, ungrouped (ADR-391)", () => {
    expect(readPrefs(holding(null))).toEqual(defaultPrefs);
    expect(readPrefs(null)).toEqual(defaultPrefs);
    expect(defaultPrefs).toEqual({ night: true, sound: true, groupByTable: false, skipPreparing: false });
  });
  it("turns the chime on for a display that kept its choices before it was the default, and keeps the rest", () => {
    expect(readPrefs(holding('{"night":false,"sound":false,"groupByTable":true}'))).toEqual({
      night: false,
      sound: true,
      groupByTable: true,
      skipPreparing: false,
    });
  });
  it("keeps sound off where it was turned off since", () => {
    expect(readPrefs(holding('{"night":true,"sound":false,"groupByTable":false,"v":2}')).sound).toBe(false);
  });
  it("is night unless paper was chosen on this device", () => {
    const cases: readonly { kept: string; night: boolean }[] = [
      { kept: "{}", night: true },
      { kept: '{"sound":true}', night: true },
      { kept: '{"night":"dark"}', night: true },
      { kept: '{"night":true}', night: true },
      { kept: '{"night":false}', night: false },
      { kept: '{"night":false,"groupByTable":true}', night: false },
    ];
    for (const c of cases) expect(readPrefs(holding(c.kept)).night, c.kept).toBe(c.night);
  });
  it("reads what was kept", () => {
    expect(readPrefs(holding('{"night":true,"sound":true,"groupByTable":true,"skipPreparing":true,"v":2}'))).toEqual({
      night: true,
      sound: true,
      groupByTable: true,
      skipPreparing: true,
    });
  });
  it("takes the default for anything missing or not a yes or no", () => {
    expect(readPrefs(holding('{"night":false,"sound":"loud","v":2}'))).toEqual({ night: false, sound: true, groupByTable: false, skipPreparing: false });
    expect(readPrefs(holding('{"groupByTable":true}'))).toEqual({ night: true, sound: true, groupByTable: true, skipPreparing: false });
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
    writePrefs({ setItem: (k, v) => void kept.set(k, v) }, { night: true, sound: false, groupByTable: true, skipPreparing: true });
    expect(readPrefs({ getItem: (k) => kept.get(k) ?? null })).toEqual({ night: true, sound: false, groupByTable: true, skipPreparing: true });
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
    expect(surfaceOf({ ...defaultPrefs, night: false })).toBe("paper");
  });
  it("is ink on a display that has chosen nothing", () => {
    expect(surfaceOf(readPrefs(null))).toBe("ink");
    expect(surfaceOf(defaultPrefs)).toBe("ink");
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

describe("noteArrivals", () => {
  const t = (seq: number) => ({ order_id: "ord_A", ticket_seq: seq }) as Parameters<typeof ticketId>[0];
  it("notes none at the first read, then each new ticket at the time it was seen, and drops those gone (KDS-1)", () => {
    const first = noteArrivals(noArrivals, [t(1), t(2)], 1_000);
    expect(first.at.size).toBe(0);
    const second = noteArrivals(first, [t(1), t(2), t(3)], 5_000);
    expect([...second.at]).toEqual([[ticketId(t(3)), 5_000]]);
    const third = noteArrivals(second, [t(1), t(3), t(4)], 9_000);
    expect(third.at.get(ticketId(t(3)))).toBe(5_000);
    expect(third.at.get(ticketId(t(4)))).toBe(9_000);
    const fourth = noteArrivals(third, [t(1)], 12_000);
    expect(fourth.at.size).toBe(0);
    expect(second.at.size).toBe(1);
  });
});

describe("newlyVoided (KDS-13)", () => {
  const line = (id: number, status: "active" | "voided") => ({ id, name: "Dosa", qty: 1, course: 1, modifiers: [], notes: [], status });
  const ticket = (order: string, lines: ReturnType<typeof line>[]) =>
    ({ order_id: order, ticket_seq: 3, lines }) as unknown as Parameters<typeof voidedKeys>[0][number];
  it("hears nothing at the first read, then each line voided since, by order and line", () => {
    const before = [ticket("ord_A", [line(2, "voided"), line(3, "active")]), ticket("ord_B", [line(2, "active")])];
    expect(newlyVoided(null, before)).toEqual([]);
    const after = [ticket("ord_A", [line(2, "voided"), line(3, "active")]), ticket("ord_B", [line(2, "voided")])];
    expect(newlyVoided(voidedKeys(before), after)).toEqual(["ord_B:2"]);
    expect(newlyVoided(voidedKeys(after), after)).toEqual([]);
  });
});
