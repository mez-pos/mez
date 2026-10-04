import { describe, expect, it } from "vitest";
import { clockText, connectionText, versionText } from "./status-logic";

describe("clockText", () => {
  const at = new Date("2026-09-19T04:54:00Z");

  it("reads the time in the venue's zone", () => {
    expect(clockText(at, "Asia/Kolkata")).toBe("Sat, 19 Sep 2026 · 10:24 AM");
  });

  it("is the same instant on another zone's clock, and its date", () => {
    expect(clockText(at, "America/New_York")).toBe("Sat, 19 Sep 2026 · 12:54 AM");
    expect(clockText(new Date("2026-09-19T20:30:00Z"), "Asia/Kolkata")).toBe("Sun, 20 Sep 2026 · 2:00 AM");
  });

  it("says PM in capitals, with the hour unpadded", () => {
    expect(clockText(new Date("2026-09-19T13:05:00Z"), "UTC")).toBe("Sat, 19 Sep 2026 · 1:05 PM");
  });

  it("falls back to the device's clock for no zone or one it does not know", () => {
    expect(clockText(at, null)).toMatch(/^\w{3}, \d{1,2} \w{3} 2026 · \d{1,2}:\d{2} (AM|PM)$/);
    expect(clockText(at, "Mars/Olympus")).toMatch(/^\w{3}, \d{1,2} \w{3} 2026 · \d{1,2}:\d{2} (AM|PM)$/);
  });
});

describe("versionText", () => {
  it("puts a v before a release number and leaves a build name alone", () => {
    expect(versionText("1.0.0")).toBe("mez v1.0.0");
    expect(versionText("dev-dirty")).toBe("mez dev-dirty");
    expect(versionText("f886835-dirty")).toBe("mez f886835-dirty");
  });
  it("says only the name until the version is known", () => {
    expect(versionText(null)).toBe("mez");
    expect(versionText("")).toBe("mez");
  });
});

describe("connectionText", () => {
  it("says Online while the stream is live, in the success tone", () => {
    expect(connectionText({ state: "live", since: 0 }, 0)).toMatchObject({ word: "Online", tone: "success", retryable: false });
  });
  it("says what the connection indicator says when it is not", () => {
    expect(connectionText({ state: "reconnecting", reason: "unreachable", retryAt: 5_000, attempt: 2 }, 0)).toMatchObject({
      word: "Server unreachable",
      tone: "danger",
      retryable: true,
    });
    expect(connectionText({ state: "syncing", target: 10 }, 0).word).toBe("Syncing");
  });
});
