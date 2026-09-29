import { describe, expect, it } from "vitest";
import { sizeText } from "./ExportsPanel";

describe("sizeText", () => {
  it("says bytes, then kilobytes to one place", () => {
    expect(sizeText(512)).toBe("512 bytes");
    expect(sizeText(1024)).toBe("1 KB");
    expect(sizeText(12_700)).toBe("12.4 KB");
  });
});
