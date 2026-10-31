import { describe, expect, it } from "vitest";
import { retireAreaWords, retireCategoryWords, retireGroupWords, retireTableWords } from "./retire-logic";

describe("retiring is asked with what goes with it (BO-6)", () => {
  it("says an area with tables in service cannot go yet, and asks about one without", () => {
    expect(retireAreaWords("Terrace", 4)).toEqual({
      title: "Terrace has 4 tables in service",
      body: "Retire them, or move them to another area, before you retire the area.",
      confirm: null,
    });
    expect(retireAreaWords("Terrace", 1).title).toBe("Terrace has 1 table in service");
    expect(retireAreaWords("Terrace", 0)).toEqual({
      title: "Retire Terrace?",
      body: "It leaves the floor plan and Tables on the till. You can bring it back here.",
      confirm: "Retire Terrace",
    });
  });

  it("says a table takes its QR code with it", () => {
    const words = retireTableWords("T4");
    expect(words.title).toBe("Retire T4?");
    expect(words.body).toMatch(/QR code leads nowhere until you bring it back/);
    expect(words.confirm).toBe("Retire T4");
  });

  it("counts the items a group of choices leaves", () => {
    expect(retireGroupWords("Milk", 12).body).toBe("12 items stop offering it. Orders already open keep their choices. Bring it back and they offer it again.");
    expect(retireGroupWords("Milk", 1).body).toMatch(/^1 item stops offering it\./);
    expect(retireGroupWords("Milk", 0).body).toBe("No item offers it. You can bring it back.");
    expect(retireGroupWords("Milk", 3).confirm).toBe("Retire Milk");
  });

  it("counts the items a category takes off the order screens", () => {
    expect(retireCategoryWords("Cakes", 3).body).toBe("Its 3 items come off every order screen. You can bring it back.");
    expect(retireCategoryWords("Cakes", 1).body).toBe("Its one item comes off every order screen. You can bring it back.");
    expect(retireCategoryWords("Cakes", 0).body).toBe("It has no items. You can bring it back.");
  });
});
