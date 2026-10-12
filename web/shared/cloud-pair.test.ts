import { describe, expect, it } from "vitest";
import { ApiError } from "./api";
import type { CloudPairResponse } from "./api-types";
import { backOfficeAddress, cleanCode, codeRefusal, pairRefusal, pairedLines, possessive } from "./cloud-pair";

describe("cleanCode", () => {
  it.each([
    ["ABCD2345", "ABCD2345"],
    ["abcd2345", "ABCD2345"],
    ["abcd-2345", "ABCD2345"],
    [" AB CD 23 45 ", "ABCD2345"],
    ["zzzz9999", "ZZZZ9999"],
  ])("%s is sent as %s", (typed, want) => {
    expect(cleanCode(typed)).toBe(want);
  });

  it.each(["", "ABCD234", "ABCD23456", "ABCD234O", "ABCD2340", "ABCD234I", "ABCD2341", "ABCD_345", "12345678"])("%s could not be a code", (typed) => {
    expect(cleanCode(typed)).toBeNull();
  });
});

describe("codeRefusal", () => {
  it("asks for a code when none is typed", () => {
    expect(codeRefusal("   ")).toBe("Type the code from your mez Cloud account.");
  });

  it("says what a code is when what was typed could not be one", () => {
    expect(codeRefusal("ABCD-234")).toMatch(/8 letters and digits, with no 0, O, 1 or I/);
  });

  it("lets a code through however it was typed", () => {
    expect(codeRefusal("abcd 2345")).toBeNull();
  });
});

describe("possessive", () => {
  it("gives a name that ends in s the apostrophe alone", () => {
    expect(possessive("Priya Menon")).toBe("Priya Menon's");
    expect(possessive("Chris")).toBe("Chris'");
  });
});

describe("backOfficeAddress", () => {
  it("is the venue's hostname with the back office's path", () => {
    expect(backOfficeAddress("corner-cafe.mezpos.com")).toBe("https://corner-cafe.mezpos.com/backoffice/");
  });
});

describe("pairedLines", () => {
  const res: CloudPairResponse = {
    owner: "Priya Menon",
    hostname: "corner-cafe.mezpos.com",
    paired_at: "2026-10-12T09:00:00Z",
    backup: "set",
    restart_needed: false,
  };

  it("says who, and that a backup is now kept away from the computer", () => {
    expect(pairedLines(res)).toEqual([
      "This venue is now paired to Priya Menon's mez Cloud account.",
      "A backup of the venue is now kept with mez Cloud, away from this computer.",
    ]);
  });

  it("says a backup of the owner's own was left alone, and how to change that", () => {
    const lines = pairedLines({ ...res, backup: "kept" });
    expect(lines[1]).toMatch(/^Your backup is left where it was\./);
    expect(lines[1]).toMatch(/turn the backup off under Settings and pair again/);
  });

  it("says the venue is paired all the same when the backup could not be started", () => {
    expect(pairedLines({ ...res, backup: "failed" })[1]).toMatch(/could not be started\. The venue is paired all the same/);
  });

  it("says nothing of a backup on a server with none", () => {
    expect(pairedLines({ ...res, backup: "none" })).toHaveLength(1);
  });

  it("says to restart last, when part of it waits for one", () => {
    const lines = pairedLines({ ...res, restart_needed: true });
    expect(lines).toHaveLength(3);
    expect(lines[2]).toMatch(/^Restart mez to finish/);
  });
});

describe("pairRefusal", () => {
  it("passes on the server's words, which say whether the code is still good", () => {
    expect(pairRefusal(new ApiError("pairing_code_refused", "mez Cloud does not accept that code.", 409, "code"))).toBe("mez Cloud does not accept that code.");
  });

  it("says the code is still good when mez itself was not reached", () => {
    expect(pairRefusal(new ApiError("network", "failed to fetch"))).toMatch(/Can't reach the mez server.*the code is still good/);
  });

  it("says who may pair a venue to someone who may not", () => {
    expect(pairRefusal(new ApiError("forbidden", "that staff member cannot do this", 403))).toMatch(/Only someone who can change the venue's settings/);
  });

  it("has words for what is not the server's at all", () => {
    expect(pairRefusal(new Error("boom"))).toMatch(/^Something went wrong/);
  });
});
