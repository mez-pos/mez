import { describe, expect, it } from "vitest";
import { readLinkedCode, withoutCode } from "./pairing-link";

describe("readLinkedCode", () => {
  it("reads a complete code out of the hash, however it was typed", () => {
    expect(readLinkedCode("#code=7K3M-9QX2")).toBe("7K3M-9QX2");
    expect(readLinkedCode("#code=7k3m9qx2")).toBe("7K3M-9QX2");
    expect(readLinkedCode("#code=7K3M%2D9QX2")).toBe("7K3M-9QX2");
    expect(readLinkedCode("#station=grill&code=7K3M-9QX2")).toBe("7K3M-9QX2");
  });

  it("answers null for no hash, no code, or a code that is not whole", () => {
    expect(readLinkedCode("")).toBeNull();
    expect(readLinkedCode("#station=grill")).toBeNull();
    expect(readLinkedCode("#code=")).toBeNull();
    expect(readLinkedCode("#code=7K3M")).toBeNull();
    expect(readLinkedCode("#code=%E0%A4%A")).toBeNull();
    expect(readLinkedCode("#mycode=7K3M9QX2")).toBeNull();
  });
});

describe("withoutCode", () => {
  it("leaves the rest of the hash and drops the code", () => {
    expect(withoutCode("#code=7K3M-9QX2")).toBe("");
    expect(withoutCode("#station=grill&code=7K3M-9QX2")).toBe("#station=grill");
    expect(withoutCode("#code=7K3M-9QX2&station=grill")).toBe("#station=grill");
    expect(withoutCode("")).toBe("");
  });
});
