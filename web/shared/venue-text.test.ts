import { describe, expect, it } from "vitest";
import { barClock, clockText, versionText } from "./venue-text";

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

describe("barClock", () => {
  const kolkata = { timezone: "Asia/Kolkata", dayStart: 240 };

  it("is the date and time through the day, and the trading day short for a phone", () => {
    expect(barClock(new Date("2026-10-31T05:28:00Z"), kolkata)).toEqual({
      text: "Sat, 31 Oct 2026 · 10:58 AM",
      short: "31 Oct",
      label: "Sat, 31 Oct 2026 · 10:58 AM",
    });
  });

  it("says whose trading day it still is between midnight and the day start (BO-3)", () => {
    // 1:05 AM on 1 Nov in Kolkata: reports still count it as 31 Oct.
    expect(barClock(new Date("2026-10-31T19:35:00Z"), kolkata)).toEqual({
      text: "Trading day 31 Oct · 1:05 AM",
      short: "31 Oct",
      label: "Sun, 1 Nov 2026 · 1:05 AM, still the trading day of 31 Oct 2026",
    });
  });

  it("turns over at the venue's own day start", () => {
    expect(barClock(new Date("2026-10-31T19:35:00Z"), { ...kolkata, dayStart: 60 }).text).toBe("Sun, 1 Nov 2026 · 1:05 AM");
  });
});
