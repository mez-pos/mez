import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { TableCodeResponse } from "../../../shared/api-types";
import { actionsFor, canPrint, codeText, codesProblem, codesSummary, confirmWords, missingCount, noAddressWords } from "./codes-logic";

const table = (id: number, code: string): TableCodeResponse => ({
  table_id: id,
  table_name: `T${id}`,
  code,
  url: code === "" ? "" : `https://cafe.example/order/t/${code}`,
});

describe("the summary", () => {
  it.each([
    [[], "No tables on the floor yet."],
    [[table(1, "aaaabbbbcccc")], "The table has a code."],
    [[table(1, "")], "The table has no code."],
    [[table(1, "aaaabbbbcccc"), table(2, "ddddeeeeffff")], "All 2 tables have a code."],
    [[table(1, ""), table(2, "")], "None of the 2 tables has a code."],
    [[table(1, ""), table(2, "ddddeeeeffff"), table(3, "gggghhhhjjjj")], "1 of 3 tables has no code."],
    [[table(1, ""), table(2, ""), table(3, "gggghhhhjjjj")], "2 of 3 tables have no code."],
  ])("counts the tables guests cannot order from", (tables, want) => {
    expect(codesSummary({ tables })).toBe(want);
  });
  it("counts what is missing", () => {
    expect(missingCount({ tables: [table(1, ""), table(2, "x"), table(3, "")] })).toBe(2);
  });
});

describe("a code on the screen", () => {
  it("is shown in groups of four, and is still the same twelve characters", () => {
    expect(codeText("dpyyd3kcn5ev")).toBe("dpyy d3kc n5ev");
    expect(codeText("dpyyd3kcn5ev").replaceAll(" ", "")).toBe("dpyyd3kcn5ev");
    expect(codeText("")).toBe("");
  });
});

describe("what a row can do", () => {
  it("can only give a code to a table with none", () => {
    expect(actionsFor(table(1, ""), true)).toEqual(["give"]);
    expect(actionsFor(table(1, ""), false)).toEqual(["give"]);
  });
  it("can show, renew and turn off a table's code", () => {
    expect(actionsFor(table(1, "aaaabbbbcccc"), true)).toEqual(["show", "renew", "revoke"]);
  });
  it("has no QR to show where the venue has no public address", () => {
    expect(canPrint({ address: "" })).toBe(false);
    expect(canPrint({ address: "https://cafe.example/order/t/" })).toBe(true);
    expect(actionsFor(table(1, "aaaabbbbcccc"), false)).toEqual(["renew", "revoke"]);
  });
});

describe("the confirm", () => {
  it("says a renewed code stops the sticker on the table, and what to do next", () => {
    const w = confirmWords("renew", "T4");
    expect(w.title).toContain("T4");
    expect(w.body).toContain("stops working at once");
    expect(w.body).toContain("Print the new code");
    expect(w.confirm).toBe("Renew the code");
  });
  it("says a code turned off takes no orders until it is given one again", () => {
    const w = confirmWords("revoke", "T4");
    expect(w.body).toContain("until you give it a code again");
    expect(w.confirm).toBe("Turn the code off");
    expect(w.done).toContain("T4");
  });
});

describe("codesProblem", () => {
  it("says what to set when the venue has no public address", () => {
    expect(codesProblem(new ApiError("no_public_url", "x", 409))).toBe(noAddressWords);
    expect(noAddressWords).toContain("public_url");
  });
  it("says a table has gone, a role cannot, and the server was not reached", () => {
    expect(codesProblem(new ApiError("table_not_found", "x", 404))).toContain("no longer on the floor");
    expect(codesProblem(new ApiError("forbidden", "x", 403))).toContain("role");
    expect(codesProblem(new ApiError("network", "offline"))).toContain("Can't reach");
  });
  it("shows nothing of the server's insides", () => {
    expect(codesProblem(new ApiError("internal", "pq: seq 4", 500))).toBe("That did not work. Try again.");
    expect(codesProblem(new Error("boom"))).toBe("That did not work. Try again.");
  });
});
