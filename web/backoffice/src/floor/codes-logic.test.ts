import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { TableCodeResponse } from "../../../shared/api-types";
import {
  CARDS_PER_PAGE,
  actionsFor,
  canPrint,
  codeText,
  codesProblem,
  codesSummary,
  confirmWords,
  hostOf,
  missingCount,
  noAddressWords,
  pickupConfirmWords,
  pickupOffWords,
  pickupState,
  printableCount,
  sheetSummary,
} from "./codes-logic";

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

describe("the sheet", () => {
  it("counts the tables that have a code to print", () => {
    expect(printableCount({ tables: [table(1, ""), table(2, "ddddeeeeffff"), table(3, "gggghhhhjjjj")] })).toBe(2);
    expect(printableCount({ tables: [] })).toBe(0);
  });
  it.each([
    [0, "No table has a code to print."],
    [1, "1 card on 1 page of A4."],
    [CARDS_PER_PAGE, `${CARDS_PER_PAGE} cards on 1 page of A4.`],
    [CARDS_PER_PAGE + 1, `${CARDS_PER_PAGE + 1} cards on 2 pages of A4.`],
    [30, "30 cards on 5 pages of A4."],
  ])("says how much paper %i cards take", (count, want) => {
    expect(sheetSummary(count)).toBe(want);
  });
  it("says whose code it is by the venue's host, and nothing of the code", () => {
    expect(hostOf("https://democafe.example/order/t/sptdem4x7abc")).toBe("democafe.example");
    expect(hostOf("https://cafe.example:8443/order/t/x")).toBe("cafe.example:8443");
    expect(hostOf("not an address")).toBe("");
    expect(hostOf("")).toBe("");
  });
});

describe("the venue's pickup code", () => {
  const pickup = (code: string, ordering: boolean) => ({ code, url: code === "" ? "" : `https://cafe.example/order/p/${code}`, ordering });

  it("says how ordering to collect stands", () => {
    expect(pickupState(pickup("", true))).toBe("none");
    expect(pickupState(pickup("", false))).toBe("none");
    expect(pickupState(pickup("sptdem4x7abc", true))).toBe("on");
    // A code where guests cannot pay online leads to a page that takes no orders.
    expect(pickupState(pickup("sptdem4x7abc", false))).toBe("no_orders");
  });

  it("can make a code where there is none, and show, renew and turn off the one there is", () => {
    expect(actionsFor(pickup("", true), true)).toEqual(["give"]);
    expect(actionsFor(pickup("sptdem4x7abc", true), true)).toEqual(["show", "renew", "revoke"]);
    expect(actionsFor(pickup("sptdem4x7abc", true), false)).toEqual(["renew", "revoke"]);
  });

  it("says a renewed code stops the one on the door, and what becomes of guests who have paid", () => {
    const words = pickupConfirmWords("renew");
    expect(words.body).toContain("stops working at once");
    expect(words.body).toContain("keep the code they are called by");
    expect(words.body).toContain("door");
    expect(words.done).toContain("Print it");
    expect(`${words.title} ${words.body} ${words.done}`).not.toMatch(/table/i);
  });

  it("says a code turned off takes no orders until one is made again", () => {
    const words = pickupConfirmWords("revoke");
    expect(words.body).toContain("until you make a code again");
    expect(words.body).toContain("keep the code they are called by");
    expect(`${words.title} ${words.body} ${words.done}`).not.toMatch(/table/i);
  });

  it("says what to set where guests cannot pay online", () => {
    expect(pickupOffWords).toContain("payment gateway");
    expect(pickupOffWords).toContain("not taking orders");
  });

  it("counts the door's card on the sheet, and says it is there", () => {
    expect(sheetSummary(1, true)).toBe("1 card on 1 page of A4: the one for the door.");
    expect(sheetSummary(CARDS_PER_PAGE + 1, true)).toBe(`${CARDS_PER_PAGE + 1} cards on 2 pages of A4, one of them for the door.`);
    expect(sheetSummary(0, false)).toBe("No table has a code to print.");
  });
});
