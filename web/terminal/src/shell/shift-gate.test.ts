import { describe, expect, it } from "vitest";
import type { ShiftResponse } from "../../../shared/api-types";
import { shiftGate } from "./shift-gate";

const shift = (extra: Partial<ShiftResponse> = {}): ShiftResponse => ({
  id: 4,
  public_id: "shf_ABCD1234",
  status: "open",
  trading_day: "2026-10-04",
  opened_by: 1,
  opened_at: "2026-10-04T04:00:00Z",
  float: { minor: 200000, currency: "INR" },
  events: [],
  counted: false,
  ...extra,
});

describe("shiftGate", () => {
  it("waits until the server has said whether a shift is open", () => {
    expect(shiftGate(undefined, "/pos")).toEqual({ kind: "wait" });
  });

  describe("with the till closed", () => {
    it("sends every screen to /shift, where the float is counted in", () => {
      for (const path of ["/pos", "/tables", "/pay/ord_ABC", "/orders/ord_ABC", "/shift/close"]) {
        expect(shiftGate(null, path)).toEqual({ kind: "go", to: "/shift" });
      }
    });
    it("lets /shift stand, and the Z-report the close just issued", () => {
      expect(shiftGate(null, "/shift")).toEqual({ kind: "stay" });
      expect(shiftGate(null, "/shift/z/12")).toEqual({ kind: "stay" });
    });
  });

  describe("with the drawer counted", () => {
    it("holds every screen at the close, the only way on", () => {
      for (const path of ["/pos", "/tables", "/shift"]) {
        expect(shiftGate(shift({ counted: true }), path)).toEqual({ kind: "go", to: "/shift/close" });
      }
      expect(shiftGate(shift({ counted: true }), "/shift/close")).toEqual({ kind: "stay" });
    });
  });

  describe("with a shift open", () => {
    it("lets the screen asked for stand, the close included", () => {
      for (const path of ["/pos", "/tables", "/pay/ord_ABC", "/shift/close", "/shift/z/3"]) {
        expect(shiftGate(shift(), path)).toEqual({ kind: "stay" });
      }
    });
    it("sends /shift on to the order screen: there is nothing to open", () => {
      expect(shiftGate(shift(), "/shift")).toEqual({ kind: "go", to: "/pos" });
    });
  });
});
