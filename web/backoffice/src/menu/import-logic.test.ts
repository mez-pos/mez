import { describe, expect, it } from "vitest";
import type { MenuImport, MenuImportItem } from "../../../shared/api-types";
import { addLabel, addedText, examplesIn, fileProblem, maxImportBytes, problemsText, summaryLines, verdictOf, whereOf } from "./import-logic";

const item = (row: number, name: string, category = "Coffee"): MenuImportItem => ({ row, name, category, price: { minor: 18000, currency: "INR" } });

const plan = (patch: Partial<MenuImport> = {}): MenuImport => ({ to_add: [], new_categories: [], skipped: [], problems: [], imported: false, ...patch });

describe("fileProblem", () => {
  it.each([
    ["menu.csv", 2_000],
    ["MENU.CSV", 2_000],
    ["my.menu.final.csv", maxImportBytes],
  ])("takes %s", (name, size) => {
    expect(fileProblem({ name, size })).toBeNull();
  });

  it.each(["menu.xlsx", "menu.xls", "menu.numbers", "menu", "menu.csv.txt"])("sends %s back to the spreadsheet", (name) => {
    expect(fileProblem({ name, size: 2_000 })).toContain("CSV UTF-8");
  });

  it("refuses a file over the server's limit before sending it", () => {
    expect(fileProblem({ name: "menu.csv", size: maxImportBytes + 1 })).toContain("1 MB");
  });

  it("refuses an empty file", () => {
    expect(fileProblem({ name: "menu.csv", size: 0 })).toContain("empty");
  });
});

describe("verdictOf", () => {
  it("is ready with rows to add and nothing wrong", () => {
    expect(verdictOf(plan({ to_add: [item(2, "Latte")] }))).toBe("ready");
  });

  it("is problems as soon as there is one, whatever else there is", () => {
    expect(verdictOf(plan({ to_add: [item(2, "Latte")], problems: [{ row: 3, column: "Price", message: "x" }] }))).toBe("problems");
  });

  it("is nothing when every row is already there", () => {
    expect(verdictOf(plan({ skipped: [{ row: 2, category: "Coffee", name: "Latte" }] }))).toBe("nothing");
  });
});

describe("summaryLines", () => {
  it("says how many, and leaves out what does not apply", () => {
    expect(summaryLines(plan({ to_add: [item(2, "Latte")] }))).toEqual(["1 item to add."]);
  });

  it("names the new categories and counts what is left alone", () => {
    const p = plan({
      to_add: [item(2, "Latte"), item(3, "Tart", "Bakes")],
      new_categories: ["Bakes", "Tiffin"],
      skipped: [{ row: 4, category: "Coffee", name: "Espresso" }],
    });
    expect(summaryLines(p)).toEqual(["2 items to add.", "2 new categories: Bakes, Tiffin.", "1 is already on the menu and is left as it is."]);
  });

  it("agrees in number", () => {
    const p = plan({
      to_add: [item(2, "Latte")],
      new_categories: ["Bakes"],
      skipped: [
        { row: 4, category: "Coffee", name: "Espresso" },
        { row: 5, category: "Coffee", name: "Mocha" },
      ],
    });
    expect(summaryLines(p)).toEqual(["1 item to add.", "1 new category: Bakes.", "2 are already on the menu and are left as they are."]);
  });
});

describe("examplesIn", () => {
  it("counts the template's examples left in the file", () => {
    expect(examplesIn(plan({ to_add: [item(2, "Example: Masala dosa"), item(3, "example: filter coffee"), item(4, "Latte")] }))).toBe(2);
  });

  it("does not count an item that only mentions the word", () => {
    expect(examplesIn(plan({ to_add: [item(2, "Good example cake")] }))).toBe(0);
  });
});

describe("the words on the buttons and the toast", () => {
  it("counts on the button", () => {
    expect(addLabel(plan({ to_add: [item(2, "Latte")] }))).toBe("Add 1 item");
    expect(addLabel(plan({ to_add: [item(2, "Latte"), item(3, "Mocha")] }))).toBe("Add 2 items");
  });

  it("says what was added", () => {
    expect(addedText(plan({ to_add: [item(2, "Latte"), item(3, "Mocha")] }))).toBe("Added 2 items.");
    expect(addedText(plan({ to_add: [item(2, "Latte")], new_categories: ["Bakes"] }))).toBe("Added 1 item and 1 category.");
    expect(addedText(plan({ to_add: [item(2, "Latte"), item(3, "Mocha")], new_categories: ["Bakes", "Tiffin"] }))).toBe("Added 2 items and 2 categories.");
  });

  it("says how many problems, and that nothing was added", () => {
    expect(problemsText(plan({ problems: [{ row: 3, column: "Price", message: "x" }] }), "menu.csv")).toBe("1 problem in menu.csv. Nothing has been added.");
    expect(
      problemsText(
        plan({
          problems: [
            { row: 3, column: "Price", message: "x" },
            { row: 4, column: "Name", message: "y" },
          ],
        }),
        "menu.csv",
      ),
    ).toBe("2 problems in menu.csv. Nothing has been added.");
  });
});

describe("whereOf", () => {
  it("is the row as the spreadsheet numbers it", () => {
    expect(whereOf({ row: 5, column: "Price", message: "x" })).toBe("Row 5");
  });

  it("is the file for a problem with the whole of it", () => {
    expect(whereOf({ row: 0, column: "", message: "x" })).toBe("The file");
  });
});
