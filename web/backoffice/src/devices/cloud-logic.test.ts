import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import { cloudProblemOf, codeProblem, lastReportLine, pairedLine } from "./cloud-logic";

describe("codeProblem", () => {
  it("asks for a code when none is typed", () => {
    expect(codeProblem("   ")).toEqual({ field: "code", message: "Type the code from your mez Cloud account." });
  });

  it("says what a code is when what was typed could not be one", () => {
    const p = codeProblem("ABCD-234");
    expect(p?.field).toBe("code");
    expect(p?.message).toMatch(/8 letters and digits, with no 0, O, 1 or I/);
  });

  it("lets a code through however it was typed", () => {
    expect(codeProblem("abcd 2345")).toBeNull();
  });
});

describe("cloudProblemOf", () => {
  it("puts a refusal of the code under the code, in the server's words", () => {
    const err = new ApiError("pairing_code_refused", "mez Cloud does not accept that code.", 409, "code");
    expect(cloudProblemOf(err)).toEqual({ field: "code", message: "mez Cloud does not accept that code." });
  });

  it("puts anything else at the foot", () => {
    const err = new ApiError("cloud_unreachable", "mez Cloud could not be reached.", 502);
    expect(cloudProblemOf(err)).toEqual({ field: null, message: "mez Cloud could not be reached." });
  });
});

describe("pairedLine", () => {
  const paired_at = new Date(2026, 9, 12, 14, 30).toISOString();

  it("says whose account, and since when", () => {
    expect(pairedLine({ owner: "Priya Menon", paired_at }, "Asia/Kolkata")).toBe("Paired to Priya Menon's mez Cloud account since 12 Oct 2026.");
  });

  it("gives a name that ends in s the apostrophe alone", () => {
    expect(pairedLine({ owner: "Chris", paired_at }, "Asia/Kolkata")).toBe("Paired to Chris' mez Cloud account since 12 Oct 2026.");
  });
});

describe("lastReportLine", () => {
  const now = new Date(2026, 9, 22, 10, 0);
  const at = (d: Date) => d.toISOString();
  it("says today, yesterday, or the date", () => {
    expect(lastReportLine({ last_report: at(new Date(2026, 9, 22, 0, 30)) }, now, "Asia/Kolkata")).toBe("Last report: today 12:30 AM.");
    expect(lastReportLine({ last_report: at(new Date(2026, 9, 21, 23, 4)) }, now, "Asia/Kolkata")).toBe("Last report: yesterday 11:04 PM.");
    expect(lastReportLine({ last_report: at(new Date(2026, 9, 12, 23, 4)) }, now, "Asia/Kolkata")).toBe("Last report: 12 Oct 2026, 11:04 PM.");
  });
  it("says none has gone when the cloud sends messages and none went yet", () => {
    expect(lastReportLine({ messages_url: "https://app.mezpos.com/v/cafe" }, now, "Asia/Kolkata")).toBe("No close-of-day report has gone yet.");
  });
  it("says nothing when the cloud said nothing of reports", () => {
    expect(lastReportLine({}, now, "Asia/Kolkata")).toBeNull();
    expect(lastReportLine(undefined, now, "Asia/Kolkata")).toBeNull();
  });
});
