import { describe, expect, it } from "vitest";
import type { LiveFloorResponse, LiveTableResponse } from "../../../shared/api-types";
import { moveActions, moveLabel, moveTiles, pickTile } from "./move-logic";

const table = (id: number, name: string, extra: Partial<LiveTableResponse> = {}): LiveTableResponse => ({
  id,
  area_id: 10,
  name,
  seats: 4,
  shape: "square",
  x: 0,
  y: 0,
  status: "free",
  covers: 0,
  orders: [],
  ...extra,
});

const party = table(4, "T4", { status: "ordered", covers: 6, orders: ["ord_A"] });
const floor: LiveFloorResponse = {
  grid: { columns: 6, rows: 4 },
  areas: [{ id: 10, name: "Main", active: true }],
  tables: [
    table(11, "T11", { status: "dirty" }),
    party,
    table(2, "T2"),
    table(5, "T5", { seats: 6 }),
    table(7, "T7", { status: "ordered", joined_to: 4, orders: ["ord_A"] }),
    table(9, "T9", { status: "settling", orders: ["ord_B"] }),
  ],
};
const tiles = moveTiles(floor, party);
const byName = (name: string) => tiles.find((t) => t.table.name === name)!;

describe("moveTiles", () => {
  it("lists every other table the way people count, with what each is", () => {
    expect(tiles.map((t) => [t.table.name, t.state, t.word])).toEqual([
      ["T2", "free", "Available · 4"],
      ["T5", "free", "Available · 6"],
      ["T7", "joined", "Joined"],
      ["T9", "taken", "Needs Bill"],
      ["T11", "taken", "To Clear"],
    ]);
  });
});

describe("pickTile", () => {
  it("takes one free table for a transfer, and the next tap replaces it", () => {
    const one = pickTile("transfer", [], byName("T2"));
    expect(one).toEqual([2]);
    expect(pickTile("transfer", one, byName("T5"))).toEqual([5]);
    expect(pickTile("transfer", one, byName("T2"))).toEqual([]);
  });
  it("takes any number for a merge: free ones to join, joined ones to let go", () => {
    const two = pickTile("merge", pickTile("merge", [], byName("T2")), byName("T7"));
    expect(two).toEqual([2, 7]);
    expect(pickTile("merge", two, byName("T2"))).toEqual([7]);
  });
  it("does not take a table with another party, or a joined one for a transfer", () => {
    expect(pickTile("merge", [], byName("T9"))).toEqual([]);
    expect(pickTile("transfer", [], byName("T11"))).toEqual([]);
    expect(pickTile("transfer", [], byName("T7"))).toEqual([]);
  });
});

describe("moveLabel", () => {
  it("names where the party goes", () => {
    expect(moveLabel("transfer", tiles, [5])).toBe("Move to T5");
  });
  it("names what is joined and what is let go", () => {
    expect(moveLabel("merge", tiles, [2])).toBe("Join T2");
    expect(moveLabel("merge", tiles, [2, 5])).toBe("Join T2, T5");
    expect(moveLabel("merge", tiles, [7])).toBe("Let go T7");
    expect(moveLabel("merge", tiles, [5, 7])).toBe("Join T5 and let go T7");
  });
  it("is nothing until a table is picked", () => {
    expect(moveLabel("transfer", tiles, [])).toBeNull();
    expect(moveLabel("merge", tiles, [])).toBeNull();
  });
});

describe("moveActions", () => {
  it("moves the party's order from its table to the one picked", () => {
    expect(moveActions("transfer", "ord_A", party, tiles, [5])).toEqual([{ kind: "move", orderId: "ord_A", from: 4, to: 5 }]);
  });
  it("joins each free table picked and parts each joined one, in turn", () => {
    expect(moveActions("merge", "ord_A", party, tiles, [2, 7])).toEqual([
      { kind: "join", orderId: "ord_A", table: 2 },
      { kind: "part", orderId: "ord_A", table: 7 },
    ]);
  });
  it("does nothing with nothing picked", () => {
    expect(moveActions("transfer", "ord_A", party, tiles, [])).toEqual([]);
  });
});
