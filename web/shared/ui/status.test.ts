import { describe, expect, it } from "vitest";
import {
  chipSpec,
  dayStatuses,
  deviceStates,
  kitchenStatuses,
  lineStatuses,
  orderStatuses,
  paymentStatuses,
  printStatuses,
  menuItemStates,
  remoteStates,
  staffStates,
  stockLevels,
  tableStatuses,
  tunnelStates,
  type ChipSpec,
} from "./status";

const all: Record<string, Readonly<Record<string, ChipSpec>>> = {
  kitchen: kitchenStatuses,
  order: orderStatuses,
  payment: paymentStatuses,
  line: lineStatuses,
  print: printStatuses,
  table: tableStatuses,
  day: dayStatuses,
  stock: stockLevels,
  menu: menuItemStates,
  staff: staffStates,
  device: deviceStates,
  tunnel: tunnelStates,
  remote: remoteStates,
};

describe("status vocabularies", () => {
  for (const [kind, table] of Object.entries(all)) {
    for (const [status, spec] of Object.entries(table)) {
      it(`${kind} ${status} has a word, not colour alone`, () => {
        expect(spec.word.trim()).not.toBe("");
      });
    }
  }

  it("keeps terracotta for Late alone (tokens.md)", () => {
    const accented = Object.entries(all).flatMap(([kind, table]) =>
      Object.entries(table)
        .filter(([, spec]) => spec.tone === "accent")
        .map(([status]) => `${kind}.${status}`),
    );
    expect(accented).toEqual(["kitchen.late"]);
  });

  it("strikes through every voided state, and only those", () => {
    const struck = Object.entries(all).flatMap(([kind, table]) =>
      Object.entries(table)
        .filter(([, spec]) => spec.struck)
        .map(([status]) => `${kind}.${status}`),
    );
    expect(struck).toEqual(["kitchen.void", "payment.voided", "line.voided", "day.voided"]);
  });

  it("gives every table state its own tone, so the floor reads at a glance", () => {
    const tones = Object.values(tableStatuses).map((s) => s.tone);
    expect(new Set(tones).size).toBe(tones.length);
    expect(chipSpec({ kind: "table", status: "dirty" })).toEqual({ tone: "neutral", word: "To clear" });
  });

  it("looks up by kind", () => {
    expect(chipSpec({ kind: "payment", status: "refunded" })).toEqual({ tone: "danger", word: "Refunded" });
    expect(chipSpec({ kind: "kitchen", status: "ready" })).toEqual({ tone: "success", word: "Ready" });
    expect(chipSpec({ kind: "print", status: "dead" })).toEqual({ tone: "danger", word: "Not printed" });
  });
});
