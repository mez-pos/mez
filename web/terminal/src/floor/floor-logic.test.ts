import { describe, expect, it } from "vitest";
import type { LiveFloorResponse, LiveTableResponse } from "../../../shared/api-types";
import {
  areaToShow,
  areasShown,
  availability,
  beginMode,
  countsText,
  idle,
  listRows,
  modeText,
  packed,
  panelFor,
  panelTitle,
  partyTable,
  startingCovers,
  stepMode,
  tableInSearch,
  tableNames,
  tileText,
  tilesOf,
} from "./floor-logic";

const tile = (extra: Partial<LiveTableResponse> = {}): LiveTableResponse => ({
  id: 1,
  area_id: 10,
  name: "T1",
  seats: 4,
  shape: "square",
  x: 0,
  y: 0,
  status: "free",
  covers: 0,
  orders: [],
  ...extra,
});

const floor = (tables: LiveTableResponse[]): LiveFloorResponse => ({
  grid: { columns: 12, rows: 8 },
  areas: [
    { id: 10, name: "Inside", active: true },
    { id: 20, name: "Terrace", active: true },
  ],
  tables,
});

describe("tilesOf", () => {
  it("keeps the layout but drops the empty edges", () => {
    const rows = tilesOf(floor([tile({ id: 1, x: 2, y: 1 }), tile({ id: 2, x: 4, y: 3 }), tile({ id: 3, x: 0, y: 0, area_id: 20 })]), 10);
    expect(rows.map((r) => r.map((t) => t?.id ?? 0))).toEqual([
      [1, 0, 0],
      [0, 0, 0],
      [0, 0, 2],
    ]);
  });
  it("is empty for an area with no tables", () => {
    expect(tilesOf(floor([tile({ area_id: 20 })]), 10)).toEqual([]);
  });
  it("packs out the rows and columns nobody sits in", () => {
    const rows = tilesOf(floor([tile({ id: 1, x: 0, y: 0 }), tile({ id: 2, x: 4, y: 0 }), tile({ id: 3, x: 2, y: 2 })]), 10);
    expect(packed(rows).map((r) => r.map((t) => t?.id ?? 0))).toEqual([
      [1, 0, 2],
      [0, 3, 0],
    ]);
    expect(packed([])).toEqual([]);
  });
});

describe("areaToShow", () => {
  it("keeps the chosen area while it is there", () => {
    expect(areaToShow(floor([]), 20)).toBe(20);
    expect(areaToShow(floor([]), 99)).toBe(10);
    expect(areaToShow({ ...floor([]), areas: [] }, null)).toBeNull();
  });
});

describe("countsText", () => {
  it("counts in turn order, in the floor's words", () => {
    const tables = [tile({ status: "dirty" }), tile({ status: "free" }), tile({ status: "settling" }), tile({ status: "free" }), tile({ status: "seated" })];
    expect(countsText(tables)).toBe("2 free · 1 seated · 1 paying · 1 to clear");
    expect(countsText([])).toBe("");
  });

  it("calls a settling table with nothing owed paid, not paying (ADR-383)", () => {
    const owed = { minor: 33000, currency: "INR" };
    const none = { minor: 0, currency: "INR" };
    const tables = [tile({ status: "settling", balance: none }), tile({ status: "settling", balance: owed }), tile({ status: "dirty" })];
    expect(countsText(tables)).toBe("1 paying · 1 paid · 1 to clear");
  });
});

describe("areasShown", () => {
  it("is the one chosen, the first when none is, or every one", () => {
    expect(areasShown(floor([]), 20).map((a) => a.id)).toEqual([20]);
    expect(areasShown(floor([]), null).map((a) => a.id)).toEqual([10]);
    expect(areasShown(floor([]), 99).map((a) => a.id)).toEqual([10]);
    expect(areasShown(floor([]), "all").map((a) => a.id)).toEqual([10, 20]);
  });
});

describe("partyTable", () => {
  it("picks the party's own table for a joined one", () => {
    const host = tile({ id: 4, status: "ordered", orders: ["ord_1"] });
    const joined = tile({ id: 7, status: "ordered", joined_to: 4 });
    expect(partyTable(joined, [host, joined])).toBe(host);
    expect(partyTable(host, [host, joined])).toBe(host);
    // A floor read mid-change may not have the host: the table stands for itself.
    expect(partyTable(joined, [joined])).toBe(joined);
  });
  it("heads the panel with every table the party is at", () => {
    const host = tile({ id: 4, name: "T4", status: "ordered", orders: ["ord_1"] });
    const joined = tile({ id: 7, name: "T7", status: "ordered", joined_to: 4 });
    expect(panelTitle(host, [host, joined, tile({ id: 9, name: "T9" })])).toBe("Table T4 + T7");
    expect(panelTitle(host, [host])).toBe("Table T4");
  });
});

describe("panelFor", () => {
  const cases: [Partial<LiveTableResponse>, string | null, ReturnType<typeof panelFor>][] = [
    [{ status: "free" }, null, { kind: "seat" }],
    [{ status: "dirty" }, null, { kind: "clear" }],
    [{ status: "seated", orders: ["ord_1"] }, null, { kind: "order", orderId: "ord_1" }],
    [{ status: "settling", orders: ["ord_1"] }, null, { kind: "order", orderId: "ord_1" }],
    [{ status: "ordered", orders: ["ord_1", "ord_2"] }, null, { kind: "choose", orders: ["ord_1", "ord_2"] }],
    [{ status: "ordered", orders: ["ord_1", "ord_2"] }, "ord_2", { kind: "order", orderId: "ord_2" }],
    // An order picked and since closed is no longer the table's.
    [{ status: "ordered", orders: ["ord_1", "ord_2"] }, "ord_9", { kind: "choose", orders: ["ord_1", "ord_2"] }],
    [{ status: "ordered", orders: [] }, null, { kind: "nothing" }],
    [{ status: "free", orders: [] }, "ord_1", { kind: "seat" }],
  ];
  it.each(cases)("%o with %o picked shows %o", (extra, picked, want) => {
    expect(panelFor(tile(extra), picked)).toEqual(want);
  });
});

describe("availability", () => {
  it("counts every state, in turn order, with its share of the floor", () => {
    const tables = [tile(), tile(), tile({ status: "ordered" }), tile({ status: "dirty" })];
    expect(availability(tables)).toEqual([
      { status: "free", count: 2, share: 50 },
      { status: "seated", count: 0, share: 0 },
      { status: "ordered", count: 1, share: 25 },
      { status: "settling", count: 0, share: 0 },
      { status: "settling", paid: true, count: 0, share: 0 },
      { status: "dirty", count: 1, share: 25 },
    ]);
  });
  it("counts a table paid in full as Paid, apart from Paying, as the line over the bars does (TBS-14)", () => {
    const tables = [
      tile({ status: "settling", balance: { minor: 0, currency: "INR" } }),
      tile({ status: "settling", balance: { minor: 500, currency: "INR" } }),
    ];
    const bars = availability(tables);
    expect(bars.find((b) => b.status === "settling" && !b.paid)?.count).toBe(1);
    expect(bars.find((b) => b.paid)?.count).toBe(1);
    expect(countsText(tables)).toBe("1 paying · 1 paid");
  });
  it("is all zero for an empty floor", () => {
    expect(availability([]).every((a) => a.count === 0 && a.share === 0)).toBe(true);
  });
});

describe("listRows", () => {
  it("goes area by area, and by name the way people count", () => {
    const data = floor([tile({ id: 1, name: "T10" }), tile({ id: 2, name: "P1", area_id: 20 }), tile({ id: 3, name: "T2" }), tile({ id: 4, name: "T1" })]);
    expect(listRows(data, data.areas).map((t) => t.name)).toEqual(["T1", "T2", "T10", "P1"]);
    expect(listRows(data, data.areas.slice(1)).map((t) => t.name)).toEqual(["P1"]);
    // The floor's own list is left as it was.
    expect(data.tables.map((t) => t.name)).toEqual(["T10", "P1", "T2", "T1"]);
  });
});

describe("tileText", () => {
  const now = Date.parse("2026-09-27T12:42:00Z");
  it("words each state", () => {
    expect(tileText(tile({ seats: 1 }), now)).toEqual({ people: "1 seat", time: null });
    expect(tileText(tile({ status: "ordered", covers: 3, since: "2026-09-27T12:00:00Z" }), now)).toEqual({ people: "3 guests", time: "42m" });
    expect(tileText(tile({ status: "seated", covers: 1, since: "2026-09-27T12:40:00Z" }), now)).toEqual({ people: "1 guest", time: "2m" });
    expect(tileText(tile({ status: "seated", covers: 0, since: "2026-09-27T12:40:00Z" }), now).people).toBe("No guest count");
    expect(tileText(tile({ status: "dirty", since: "2026-09-27T12:36:00Z" }), now)).toEqual({ people: "Needs clearing", time: "6m" });
  });
});

describe("small helpers", () => {
  it("starts covers at the table's seats, within 1 to 12", () => {
    expect([0, 1, 4, 30].map(startingCovers)).toEqual([1, 1, 4, 12]);
  });
  it("names tables by id", () => {
    expect(tableNames(floor([tile({ id: 7, name: "Patio 2" })])).get(7)).toBe("Patio 2");
    expect(tableNames(undefined).size).toBe(0);
  });
});

describe("a joined table", () => {
  it("says whose party it is with", () => {
    const names = new Map([[4, "T4"]]);
    const t = tile({ status: "ordered", joined_to: 4, since: "2026-09-27T12:00:00Z" });
    expect(tileText(t, Date.parse("2026-09-27T12:05:00Z"), names)).toEqual({ people: "With T4", time: "5m" });
    expect(tileText(t, Date.parse("2026-09-27T12:05:00Z")).people).toBe("With table 4");
  });
});

describe("stepMode", () => {
  const names = new Map([[4, "T4"]]);
  const party = tile({ id: 4, name: "T4", status: "ordered", orders: ["ord_1"] });
  const free = tile({ id: 6, name: "T6" });
  const busy = tile({ id: 5, name: "T5", status: "seated", orders: ["ord_2"] });
  const joined = tile({ id: 7, name: "T7", status: "ordered", orders: ["ord_1"], joined_to: 4 });

  it("does nothing while idle", () => {
    expect(stepMode(idle, party, names)).toEqual({ mode: idle });
  });

  it("moves a party to a free table, then stops", () => {
    const first = stepMode({ kind: "move", from: null }, party, names);
    expect(first.mode).toEqual({ kind: "move", from: party });
    expect(stepMode(first.mode, free, names)).toEqual({ mode: idle, action: { kind: "move", orderId: "ord_1", from: 4, to: 6 } });
  });

  it("refuses to move from an empty, joined or two-ticket table, or onto a busy one", () => {
    const start = { kind: "move", from: null } as const;
    expect(stepMode(start, free, names).refused).toMatch(/nobody at it/);
    expect(stepMode(start, joined, names).refused).toBe("T7 is joined to T4. Tap that one.");
    expect(stepMode(start, { ...party, orders: ["a", "b"] }, names).refused).toMatch(/2 orders/);
    expect(stepMode({ kind: "move", from: party }, busy, names).refused).toBe("T5 isn't free. Tap a free table.");
  });

  it("lets go of the first table when it is tapped again", () => {
    expect(stepMode({ kind: "move", from: party }, party, names).mode).toEqual({ kind: "move", from: null });
  });

  it("joins free tables one tap at a time, and lets a joined one go", () => {
    const mode = { kind: "join", to: party } as const;
    expect(stepMode(mode, free, names)).toEqual({ mode, action: { kind: "join", orderId: "ord_1", table: 6 } });
    expect(stepMode(mode, joined, names)).toEqual({ mode, action: { kind: "part", orderId: "ord_1", table: 7 } });
    expect(stepMode(mode, busy, names).refused).toMatch(/isn't free/);
    expect(stepMode(mode, party, names)).toEqual({ mode });
  });

  it("parts joined tables and nothing else", () => {
    const mode = { kind: "part" } as const;
    expect(stepMode(mode, joined, names)).toEqual({ mode, action: { kind: "part", orderId: "ord_1", table: 7 } });
    expect(stepMode(mode, party, names).refused).toBe("T4 isn't joined to another table.");
  });

  it("begins a move or a join from the table picked in the panel", () => {
    expect(beginMode("move", party, names)).toEqual({ mode: { kind: "move", from: party } });
    expect(beginMode("join", party, names)).toEqual({ mode: { kind: "join", to: party } });
    expect(beginMode("move", free, names).refused).toMatch(/nobody at it/);
    expect(beginMode("move", free, names).mode).toEqual({ kind: "move", from: null });
  });

  it("says what to do next", () => {
    expect(modeText(idle)).toBeNull();
    expect(modeText({ kind: "move", from: null })).toBe("Move: tap the table the party is at");
    expect(modeText({ kind: "move", from: party })).toBe("Tap a free table for T4's party");
    expect(modeText({ kind: "join", to: null })).toBe("Join: tap the party's table");
    expect(modeText({ kind: "part" })).toMatch(/joined table/);
  });
});

describe("tableInSearch", () => {
  it("reads the table a link names", () => {
    expect(tableInSearch("table=7")).toBe(7);
    expect(tableInSearch("?table=12&x=1")).toBe(12);
  });
  it("picks nothing for no table, or one that is no table's id", () => {
    for (const search of ["", "table=", "table=0", "table=-3", "table=1.5", "table=abc", "table=99999999999", "other=4"]) {
      expect(tableInSearch(search)).toBeNull();
    }
  });
});
