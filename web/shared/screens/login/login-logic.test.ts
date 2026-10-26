import { describe, expect, it } from "vitest";
import { pageOf, tilePage } from "./login-logic";

const people = (n: number) => Array.from({ length: n }, (_, i) => ({ id: `stf_${i + 1}` }));

describe("tilePage", () => {
  it("shows everyone, with no More, when they fit on a page", () => {
    const p = tilePage(people(3), 0);
    expect(p.shown.map((x) => x.id)).toEqual(["stf_1", "stf_2", "stf_3"]);
    expect(p).toMatchObject({ hasMore: false, next: 0, page: 0, pages: 1 });
    expect(tilePage(people(4), 0).hasMore).toBe(false);
  });

  it("offers More past four, and goes round to the first page after the last", () => {
    expect(tilePage(people(5), 0)).toMatchObject({ hasMore: true, next: 1, pages: 2 });
    const second = tilePage(people(5), 1);
    expect(second.shown.map((x) => x.id)).toEqual(["stf_5"]);
    expect(second.next).toBe(0);
  });

  it("pages a long list four at a time", () => {
    const list = people(10);
    expect(tilePage(list, 2).shown.map((x) => x.id)).toEqual(["stf_9", "stf_10"]);
    expect(tilePage(list, 1).next).toBe(2);
  });

  it("keeps the page in range when the list shrank under it, or when it is nonsense", () => {
    expect(tilePage(people(3), 4).page).toBe(0);
    expect(tilePage(people(9), 7).page).toBe(2);
    expect(tilePage(people(9), -1).page).toBe(0);
    expect(tilePage(people(9), Number.NaN).page).toBe(0);
  });

  it("has an empty page for nobody", () => {
    expect(tilePage([], 0)).toEqual({ shown: [], hasMore: false, next: 0, prev: null, page: 0, pages: 1 });
  });
});

describe("pageOf", () => {
  it("finds the page a person is on", () => {
    expect(pageOf(people(9), "stf_1")).toBe(0);
    expect(pageOf(people(9), "stf_4")).toBe(0);
    expect(pageOf(people(9), "stf_5")).toBe(1);
    expect(pageOf(people(9), "stf_9")).toBe(2);
  });
  it("says the first page for someone who is not there", () => {
    expect(pageOf(people(9), "stf_nobody")).toBe(0);
  });
});

describe("tilePage's Back", () => {
  it("offers the page before, and none on the first", () => {
    const people = ["a", "b", "c", "d", "e", "f", "g", "h", "i"];
    expect(tilePage(people, 0).prev).toBeNull();
    expect(tilePage(people, 1).prev).toBe(0);
    expect(tilePage(people, 2).prev).toBe(1);
  });
});
