import { describe, expect, it } from "vitest";
import {
  addDays,
  dayShort,
  dayStartHour,
  dayStartMinutes,
  dayRunsText,
  dayText,
  lastDays,
  tradingDay,
  venueClock,
  venueDate,
  venueDateTime,
  venueTime,
  type VenueClock,
} from "./venue-clock";

const kolkata = (dayStart = 240): VenueClock => ({ timezone: "Asia/Kolkata", dayStart });
const at = (iso: string) => new Date(iso);

describe("dayStartMinutes", () => {
  it("reads the venue's day start", () => {
    expect(dayStartMinutes("04:00")).toBe(240);
    expect(dayStartMinutes("05:30")).toBe(330);
    expect(dayStartMinutes("00:00")).toBe(0);
    expect(dayStartMinutes("23:59")).toBe(1439);
  });
  it("takes the server's default, 04:00, for none or one that is not a time", () => {
    expect(dayStartMinutes(undefined)).toBe(240);
    expect(dayStartMinutes("")).toBe(240);
    expect(dayStartMinutes("25:00")).toBe(240);
    expect(dayStartMinutes("soon")).toBe(240);
  });
});

describe("venueClock", () => {
  it("is the venue's zone and day start, or the device's zone and the default before the venue is read", () => {
    expect(venueClock({ timezone: "Asia/Kolkata", day_start: "05:30" })).toEqual({ timezone: "Asia/Kolkata", dayStart: 330 });
    expect(venueClock(undefined)).toEqual({ timezone: null, dayStart: 240 });
  });
});

describe("tradingDay", () => {
  it("is the venue's date, or the date before when it is earlier than the day start (BO-1)", () => {
    // 02:30 on 1 Nov in Kolkata: still the 31st's trading day.
    expect(tradingDay(at("2026-10-31T21:00:00Z"), kolkata())).toBe("2026-10-31");
    // 04:00 sharp: the new day.
    expect(tradingDay(at("2026-10-31T22:30:00Z"), kolkata())).toBe("2026-11-01");
    // A venue whose day starts at 02:00 has turned over by 02:30.
    expect(tradingDay(at("2026-10-31T21:00:00Z"), kolkata(120))).toBe("2026-11-01");
  });
  it("reads the venue's zone, not the device's", () => {
    // 03:30 in London on 31 Oct (after the clocks went back): the 30th's day.
    expect(tradingDay(at("2026-10-31T03:30:00Z"), { timezone: "Europe/London", dayStart: 240 })).toBe("2026-10-30");
    // The same moment is 09:00 in Kolkata: the 31st.
    expect(tradingDay(at("2026-10-31T03:30:00Z"), kolkata())).toBe("2026-10-31");
  });
});

describe("addDays and lastDays", () => {
  it("counts calendar days across months and years", () => {
    expect(addDays("2026-03-01", -1)).toBe("2026-02-28");
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
  });
  it("is the last n trading days, today included", () => {
    expect(lastDays(at("2026-10-31T21:00:00Z"), 7, kolkata())).toEqual({ from: "2026-10-25", to: "2026-10-31" });
    expect(lastDays(at("2026-11-02T06:00:00Z"), 3, kolkata())).toEqual({ from: "2026-10-31", to: "2026-11-02" });
  });
});

describe("dayStartHour", () => {
  it("is the clock hour the trading day starts in", () => {
    expect(dayStartHour(kolkata())).toBe(4);
    expect(dayStartHour(kolkata(330))).toBe(5);
  });
});

describe("dayRunsText", () => {
  it("says when the venue's trading day runs, where a screen used to say 04:00 for every venue", () => {
    expect(dayRunsText(kolkata())).toBe("A trading day runs 4:00 AM to 4:00 AM.");
    expect(dayRunsText(kolkata(330))).toBe("A trading day runs 5:30 AM to 5:30 AM.");
    expect(dayRunsText(kolkata(0))).toBe("A trading day runs midnight to midnight.");
  });
});

describe("dayText and dayShort", () => {
  it("writes a trading day one way (BO-4)", () => {
    expect(dayText("2026-09-24")).toBe("24 Sep 2026");
    expect(dayShort("2026-09-24")).toBe("24 Sep");
    expect(dayText("2027-01-05")).toBe("5 Jan 2027");
  });
  it("leaves what is not a day as it came", () => {
    expect(dayText("someday")).toBe("someday");
  });
});

describe("venueDate, venueTime and venueDateTime", () => {
  it("read an instant in the venue's zone: someone added at 01:30 there joined that day (BO-4)", () => {
    expect(venueDate("2026-10-01T20:00:00Z", "Asia/Kolkata")).toBe("2 Oct 2026");
    expect(venueDate("2026-10-01T20:00:00Z", "UTC")).toBe("1 Oct 2026");
  });
  it("write the time on the twelve-hour clock, as the status bar does (TBS-17)", () => {
    expect(venueTime("2026-10-31T05:28:00Z", "Asia/Kolkata")).toBe("10:58 AM");
    expect(venueTime("2026-10-31T18:30:00Z", "Asia/Kolkata")).toBe("12:00 AM");
    expect(venueTime("2026-10-31T06:30:00Z", "Asia/Kolkata")).toBe("12:00 PM");
    expect(venueTime(at("2026-10-31T15:10:00Z"), "Asia/Kolkata")).toBe("8:40 PM");
  });
  it("put the two together", () => {
    expect(venueDateTime("2026-10-31T05:28:00Z", "Asia/Kolkata")).toBe("31 Oct 2026, 10:58 AM");
  });
  it("say nothing for what is not a time, and fall back to the device's zone for one this browser does not know", () => {
    expect(venueDate("not a time", "Asia/Kolkata")).toBe("");
    expect(venueDateTime("", "Asia/Kolkata")).toBe("");
    expect(venueDate("2026-10-01T12:00:00Z", "Mars/Olympus")).toMatch(/^\d{1,2} (Sep|Oct) 2026$/);
  });
});
