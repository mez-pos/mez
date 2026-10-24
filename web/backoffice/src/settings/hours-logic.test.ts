import { describe, expect, it } from "vitest";
import { DEFAULT_CLOSE, DEFAULT_OPEN, WEEKDAYS, hoursFormFrom, hoursProblem, isOvernight, openDay, toHoursInput, type HoursForm } from "./hours-logic";

const closed = { closed: true, open: "", close: "" };

describe("the opening hours form", () => {
  it("lists the seven days, Monday first", () => {
    expect(WEEKDAYS.map((d) => d.key)).toEqual(["mon", "tue", "wed", "thu", "fri", "sat", "sun"]);
  });
  it("is closed on every day the server does not name, and not set reads the same", () => {
    const none = hoursFormFrom({});
    expect(Object.keys(none)).toHaveLength(7);
    expect(none.mon).toEqual(closed);
    expect(hoursFormFrom(undefined)).toEqual(none);
  });
  it("holds what the server has", () => {
    const f = hoursFormFrom({ mon: { open: "09:00", close: "23:00" }, sat: { open: "17:00", close: "02:00" } });
    expect(f.mon).toEqual({ closed: false, open: "09:00", close: "23:00" });
    expect(f.sat).toEqual({ closed: false, open: "17:00", close: "02:00" });
    expect(f.sun).toEqual(closed);
  });
  it("starts a newly ticked day at the defaults and keeps times it had", () => {
    expect(openDay(closed)).toEqual({ closed: false, open: DEFAULT_OPEN, close: DEFAULT_CLOSE });
    expect(openDay({ closed: true, open: "08:00", close: "20:00" })).toEqual({ closed: false, open: "08:00", close: "20:00" });
  });
  it("sends only the open days, trimmed, and nothing for a week with none", () => {
    const f: HoursForm = {
      ...hoursFormFrom({}),
      tue: { closed: false, open: " 10:00", close: "22:00 " },
      wed: { closed: true, open: "10:00", close: "22:00" },
    };
    expect(toHoursInput(f)).toEqual({ tue: { open: "10:00", close: "22:00" } });
    expect(toHoursInput(hoursFormFrom({}))).toEqual({});
  });
  it("knows a bar from a cafe", () => {
    expect(isOvernight({ closed: false, open: "17:00", close: "02:00" })).toBe(true);
    expect(isOvernight({ closed: false, open: "09:00", close: "23:00" })).toBe(false);
    expect(isOvernight({ closed: false, open: "9", close: "02:00" })).toBe(false);
  });
});

describe("what the hours refuse before asking the server", () => {
  const withMon = (open: string, close: string): HoursForm => ({ ...hoursFormFrom({}), mon: { closed: false, open, close } });
  it("lets a good week through, a closed week included", () => {
    expect(hoursProblem(hoursFormFrom({}))).toBeNull();
    expect(hoursProblem(withMon("09:00", "23:00"))).toBeNull();
    expect(hoursProblem(withMon("17:00", "02:00"))).toBeNull();
  });
  it("wants a time of day at each end, naming the day", () => {
    expect(hoursProblem(withMon("", "23:00"))).toBe("Give Monday an opening time, like 09:00.");
    expect(hoursProblem(withMon("09:00", "24:00"))).toBe("Give Monday a closing time, like 23:00.");
  });
  it("refuses a day open round the clock", () => {
    expect(hoursProblem(withMon("09:00", "09:00"))).toMatch(/Monday opens and closes at 09:00/);
  });
  it("ignores the times of a closed day", () => {
    expect(hoursProblem({ ...hoursFormFrom({}), fri: { closed: true, open: "x", close: "y" } })).toBeNull();
  });
  it("names the earliest day with a problem", () => {
    const f: HoursForm = { ...withMon("09:00", "23:00"), wed: { closed: false, open: "", close: "" }, thu: { closed: false, open: "", close: "" } };
    expect(hoursProblem(f)).toMatch(/Wednesday/);
  });
});
