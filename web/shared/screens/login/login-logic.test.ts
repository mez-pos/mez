import { describe, expect, it } from "vitest";
import { defaultPerPage, pageOf, perPage, slotsAcross, tilePage } from "./login-logic";

const people = (n: number) => Array.from({ length: n }, (_, i) => ({ id: `stf_${i + 1}` }));

describe("tilePage", () => {
  it("shows everyone, with no More, when they fit on a page", () => {
    const p = tilePage(people(3), 0);
    expect(p.shown.map((x) => x.id)).toEqual(["stf_1", "stf_2", "stf_3"]);
    expect(p).toMatchObject({ hasMore: false, next: 0, page: 0, pages: 1 });
    expect(tilePage(people(4), 0).hasMore).toBe(false);
  });

  it("offers More past a page, and on the second of two only Back, which goes where More would", () => {
    expect(tilePage(people(5), 0)).toMatchObject({ hasMore: true, next: 1, prev: null, pages: 2 });
    const second = tilePage(people(5), 1);
    expect(second.shown.map((x) => x.id)).toEqual(["stf_5"]);
    expect(second).toMatchObject({ hasMore: false, prev: 0 });
  });

  it("goes round to the first page after the last, when there are more than two", () => {
    const last = tilePage(people(10), 2);
    expect(last).toMatchObject({ hasMore: true, next: 0, prev: 1 });
  });

  it("pages a long list a page at a time", () => {
    const list = people(10);
    expect(tilePage(list, 2).shown.map((x) => x.id)).toEqual(["stf_9", "stf_10"]);
    expect(tilePage(list, 1).next).toBe(2);
    expect(tilePage(list, 1, 6).shown.map((x) => x.id)).toEqual(["stf_7", "stf_8", "stf_9", "stf_10"]);
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
  it("finds it at another page size", () => {
    expect(pageOf(people(9), "stf_5", 6)).toBe(0);
    expect(pageOf(people(9), "stf_7", 6)).toBe(1);
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

describe("slotsAcross", () => {
  it.each([
    // The tile is 104px with 16px between: a 1024 tablet, the brand panel hidden.
    { name: "a 1024 tablet's full width", width: 944, slot: 104, gap: 16, want: 8 },
    // 1366 wide, the brand panel taking 46%.
    { name: "beside the brand panel at 1366", width: 658, slot: 104, gap: 16, want: 5 },
    // 1920, at 2xl: 124px tiles with 24px between.
    { name: "beside the brand panel at 1920", width: 893, slot: 124, gap: 24, want: 6 },
    { name: "exactly five, with no gap after the last", width: 5 * 104 + 4 * 16, slot: 104, gap: 16, want: 5 },
    { name: "a pixel short of five", width: 5 * 104 + 4 * 16 - 1, slot: 104, gap: 16, want: 4 },
    { name: "no gap", width: 300, slot: 100, gap: 0, want: 3 },
    { name: "a gap that is not one", width: 300, slot: 100, gap: Number.NaN, want: 3 },
    { name: "narrower than one", width: 80, slot: 104, gap: 16, want: 0 },
    { name: "not measured yet", width: 0, slot: 0, gap: 16, want: 0 },
    { name: "a width that is not one", width: Number.NaN, slot: 104, gap: 16, want: 0 },
  ])("$name: $want", ({ width, slot, gap, want }) => {
    expect(slotsAcross(width, slot, gap)).toBe(want);
  });
});

describe("perPage", () => {
  it.each([
    { name: "everyone fits", slots: 8, count: 5, want: 8 },
    { name: "everyone fits exactly", slots: 5, count: 5, want: 5 },
    { name: "two pages: one slot kept for More or Back", slots: 5, count: 6, want: 4 },
    { name: "two pages at the edge", slots: 5, count: 8, want: 4 },
    { name: "three pages: two slots kept, for Back and More", slots: 5, count: 9, want: 3 },
    { name: "a long list on a wide row", slots: 8, count: 30, want: 6 },
    { name: "a row of three, more than two pages", slots: 3, count: 7, want: 1 },
    { name: "too narrow for a profile, Back and More", slots: 2, count: 9, want: defaultPerPage },
    { name: "not measured yet", slots: 0, count: 9, want: defaultPerPage },
  ])("$name: $want", ({ slots, count, want }) => {
    expect(perPage(slots, count)).toBe(want);
  });

  it("never leaves a page wider than its row", () => {
    for (let slots = 3; slots <= 12; slots++) {
      for (let count = 1; count <= 40; count++) {
        const size = perPage(slots, count);
        const view = tilePage(people(count), 0, size);
        for (let page = 0; page < view.pages; page++) {
          const p = tilePage(people(count), page, size);
          const used = p.shown.length + (p.prev !== null ? 1 : 0) + (p.hasMore ? 1 : 0);
          expect(used, `${count} people, ${slots} slots, page ${page}`).toBeLessThanOrEqual(slots);
        }
      }
    }
  });
});
