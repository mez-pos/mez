import { describe, expect, it } from "vitest";
import type { Permission } from "../../../shared/api-types";
import { CHORD_WINDOW_MS, NAV, canOpen, chord, section, sectionForPath, visibleNav, type KeyInput } from "./nav-logic";

const owner: Permission[] = ["menu.edit", "staff.manage", "reports.view", "settings.edit", "device.manage", "stock.manage"];
const manager: Permission[] = ["staff.manage", "reports.view", "shift.manage"];
const cashier: Permission[] = ["order.open", "payment.take"];

const ids = (held: Permission[]) => visibleNav(held).flatMap((g) => g.sections.map((s) => s.id));
const labels = (held: Permission[]) => visibleNav(held).map((g) => g.label);

describe("visibleNav", () => {
  const tests: { name: string; held: Permission[]; ids: string[]; groups: (string | null)[] }[] = [
    {
      name: "the owner sees everything",
      held: owner,
      ids: ["dashboard", "menu", "stock", "people", "zreports", "sales", "floor", "printers", "settings", "devices"],
      groups: [null, "Run the venue", "Reports", "Settings"],
    },
    {
      name: "a manager sees people and reports, and no settings group",
      held: manager,
      ids: ["dashboard", "people", "zreports", "sales"],
      groups: [null, "Run the venue", "Reports"],
    },
    { name: "a cashier sees the dashboard alone", held: cashier, ids: ["dashboard"], groups: [null] },
    { name: "no permissions at all still reach the dashboard", held: [], ids: ["dashboard"], groups: [null] },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(ids(tt.held)).toEqual(tt.ids);
      expect(labels(tt.held)).toEqual(tt.groups);
    });
  }
});

describe("the section list", () => {
  const all = NAV.flatMap((g) => g.sections);
  it("gives every section its own key and path", () => {
    expect(new Set(all.map((s) => s.key)).size).toBe(all.length);
    expect(new Set(all.map((s) => s.path)).size).toBe(all.length);
  });
  it("does not use g, which starts every chord", () => {
    expect(all.map((s) => s.key)).not.toContain("g");
  });
});

describe("canOpen", () => {
  it("needs the section's permission, and nothing for the dashboard", () => {
    expect(canOpen(section("printers"), owner)).toBe(true);
    expect(canOpen(section("printers"), manager)).toBe(false);
    expect(canOpen(section("dashboard"), [])).toBe(true);
  });
});

describe("sectionForPath", () => {
  const tests: [string, string | null][] = [
    ["/", "dashboard"],
    ["", "dashboard"],
    ["/printers", "printers"],
    ["/reports", "zreports"],
    ["/reports/12", "zreports"],
    ["/shifts/4/tips", "zreports"],
    ["/floor", "floor"],
    ["/floorplan", null],
    ["/nowhere", null],
  ];
  for (const [path, want] of tests) {
    it(`puts ${JSON.stringify(path)} in ${want ?? "no section"}`, () => {
      expect(sectionForPath(path)?.id ?? null).toBe(want);
    });
  }
});

describe("chord", () => {
  const key = (k: string, now: number, extra: Partial<KeyInput> = {}): KeyInput => ({ key: k, now, modified: false, typing: false, ...extra });

  const run = (keys: KeyInput[], held: Permission[] = owner) => {
    let state: number | null = null;
    const went: string[] = [];
    for (const k of keys) {
      const r = chord(state, k, held);
      state = r.state;
      if (r.go) went.push(r.go);
    }
    return went;
  };

  const tests: { name: string; keys: KeyInput[]; held?: Permission[]; want: string[] }[] = [
    { name: "g then t goes to printers", keys: [key("g", 0), key("t", 100)], want: ["/printers"] },
    { name: "a capital G works too", keys: [key("G", 0), key("r", 100)], want: ["/reports"] },
    { name: "g then d goes to the dashboard", keys: [key("g", 0), key("d", 10)], want: ["/"] },
    { name: "a letter alone does nothing", keys: [key("t", 0)], want: [] },
    { name: "the second key at the window's edge still counts", keys: [key("g", 0), key("f", CHORD_WINDOW_MS)], want: ["/floor"] },
    { name: "the second key after the window is forgotten", keys: [key("g", 0), key("f", CHORD_WINDOW_MS + 1)], want: [] },
    { name: "typing in a field is never a chord", keys: [key("g", 0, { typing: true }), key("t", 10, { typing: true })], want: [] },
    { name: "a modifier is never a chord", keys: [key("g", 0), key("t", 10, { modified: true })], want: [] },
    { name: "g then m goes to the menu", keys: [key("g", 0), key("m", 10)], want: ["/menu"] },
    { name: "a section not built yet goes nowhere", keys: [key("g", 0), key("k", 10)], want: [] },
    { name: "a section the staff member cannot open goes nowhere", keys: [key("g", 0), key("t", 10)], held: manager, want: [] },
    { name: "an unknown letter ends the chord", keys: [key("g", 0), key("x", 10), key("t", 20)], want: [] },
    { name: "two chords in a row both go", keys: [key("g", 0), key("r", 10), key("g", 20), key("f", 30)], want: ["/reports", "/floor"] },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(run(tt.keys, tt.held)).toEqual(tt.want);
    });
  }
});
