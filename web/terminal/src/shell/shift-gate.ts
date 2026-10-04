// Where the terminal must be, given the shift (ADR-094, ADR-148). With no shift
// open nothing can be rung up, so every screen waits at /shift, where the float
// is counted in. A shift whose drawer has been counted is being closed, and the
// close is the only way on. Otherwise the screen asked for stands.

import type { ShiftResponse } from "../../../shared/api-types";
import { route } from "./routes";

export type ShiftGate = { kind: "wait" } | { kind: "go"; to: string } | { kind: "stay" };

/**
 * shift is the open shift, null when the till is closed, undefined until the
 * server has said.
 */
export function shiftGate(shift: ShiftResponse | null | undefined, path: string): ShiftGate {
  if (shift === undefined) return { kind: "wait" };
  if (shift === null) {
    // A Z-report just issued is read at /shift/z/:n, with the till closed.
    return path === route.shift || path.startsWith("/shift/z/") ? { kind: "stay" } : { kind: "go", to: route.shift };
  }
  if (shift.counted) return path === route.shiftClose ? { kind: "stay" } : { kind: "go", to: route.shiftClose };
  if (path === route.shift) return { kind: "go", to: route.order };
  return { kind: "stay" };
}
