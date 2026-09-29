import { describe, expect, it } from "vitest";
import { channelName, differenceWords, methodName, rateText, reportProblem, staffName } from "./zreport-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

describe("rateText", () => {
  it("writes thousandths of a percent without trailing zeros", () => {
    expect(rateText(2500)).toBe("2.5%");
    expect(rateText(9000)).toBe("9%");
    expect(rateText(8875)).toBe("8.875%");
    expect(rateText(250)).toBe("0.25%");
    expect(rateText(0)).toBe("0%");
  });
});

describe("names", () => {
  it("names channels and methods for a person", () => {
    expect(channelName("qr_table")).toBe("QR at the table");
    expect(channelName("somewhere")).toBe("somewhere");
    expect(methodName("upi")).toBe("UPI");
  });
  it("names staff from the report's own list", () => {
    const names = [{ id: 2, name: "Priya" }];
    expect(staffName(names, 2)).toBe("Priya");
    expect(staffName(names, 9)).toBe("Staff 9");
  });
});

describe("differenceWords", () => {
  it("says short, over or balanced with a tone", () => {
    expect(differenceWords(inr(0))).toEqual({ text: "Balanced", tone: "success" });
    const short = differenceWords(inr(-950));
    expect(short.tone).toBe("danger");
    expect(short.text).toMatch(/^Short .*9\.50$/);
    expect(differenceWords(inr(500)).text).toMatch(/^Over .*5\.00$/);
  });
});

describe("reportProblem", () => {
  it("says why a report could not be read", () => {
    expect(reportProblem(403)).toMatch(/permission/);
    expect(reportProblem(404, 99)).toBe("There is no Z-report 99.");
    expect(reportProblem(undefined)).toMatch(/Can't reach/);
  });
});
