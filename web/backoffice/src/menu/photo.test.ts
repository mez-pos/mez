import { describe, expect, it } from "vitest";
import { fitWithin, minPhotoSide, photoSide, sizeProblem } from "./photo";

describe("fitWithin", () => {
  const tests: [number, number, { width: number; height: number }][] = [
    [4032, 3024, { width: 480, height: 360 }],
    [3024, 4032, { width: 360, height: 480 }],
    [1000, 1000, { width: 480, height: 480 }],
    [300, 200, { width: 300, height: 200 }],
    [480, 100, { width: 480, height: 100 }],
    [9600, 10, { width: 480, height: 1 }],
  ];
  for (const [w, h, want] of tests) {
    it(`fits ${w} x ${h} as ${want.width} x ${want.height}`, () => expect(fitWithin(w, h, photoSide)).toEqual(want));
  }
});

describe("sizeProblem", () => {
  it("refuses a side under the server's minimum, and accepts the minimum", () => {
    expect(sizeProblem(minPhotoSide - 1, 300)).toMatch(/too small/);
    expect(sizeProblem(300, minPhotoSide - 1)).toMatch(/too small/);
    expect(sizeProblem(minPhotoSide, minPhotoSide)).toBeNull();
  });
});
