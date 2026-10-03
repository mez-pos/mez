import { describe, expect, it } from "vitest";
import { currentCursor, firstPage, nextPage, nextSort, pageNumber, previousPage } from "./table-state";

describe("nextSort", () => {
  const tests = [
    { name: "first tap sorts ascending", current: null, column: "total", want: { column: "total", direction: "asc" } },
    {
      name: "same column flips to descending",
      current: { column: "total", direction: "asc" as const },
      column: "total",
      want: { column: "total", direction: "desc" },
    },
    { name: "and back", current: { column: "total", direction: "desc" as const }, column: "total", want: { column: "total", direction: "asc" } },
    {
      name: "another column starts ascending",
      current: { column: "total", direction: "desc" as const },
      column: "opened",
      want: { column: "opened", direction: "asc" },
    },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(nextSort(tt.current, tt.column)).toEqual(tt.want));
  }
});

describe("cursor pages", () => {
  it("starts on page one with no cursor", () => {
    expect(currentCursor(firstPage)).toBeNull();
    expect(pageNumber(firstPage)).toBe(1);
  });

  it("goes forward by cursor and back by memory", () => {
    const two = nextPage(firstPage, "c2");
    const three = nextPage(two, "c3");
    expect([currentCursor(three), pageNumber(three)]).toEqual(["c3", 3]);

    const back = previousPage(three);
    expect([currentCursor(back), pageNumber(back)]).toEqual(["c2", 2]);
    expect(currentCursor(previousPage(back))).toBeNull();
  });

  it("stays on page one when asked to go back from it", () => {
    expect(previousPage(firstPage)).toBe(firstPage);
  });

  it("never changes the state it was given", () => {
    const two = nextPage(firstPage, "c2");
    previousPage(two);
    expect(two.stack).toEqual([null, "c2"]);
    expect(firstPage.stack).toEqual([null]);
  });
});
