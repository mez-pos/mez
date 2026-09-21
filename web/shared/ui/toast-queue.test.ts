import { describe, expect, it } from "vitest";
import { emptyToasts, maxVisibleToasts, toastReducer, type ToastState } from "./toast-queue";

const show = (message: string) => ({ type: "show" as const, tone: "neutral" as const, message, durationMs: 4000 });

describe("toastReducer", () => {
  it("adds toasts with increasing ids, without changing the old state", () => {
    const one = toastReducer(emptyToasts, show("Ticket sent"));
    const two = toastReducer(one, show("Printed"));
    expect(two.toasts.map((t) => [t.id, t.message])).toEqual([
      [1, "Ticket sent"],
      [2, "Printed"],
    ]);
    expect(one.toasts).toHaveLength(1);
    expect(emptyToasts.toasts).toHaveLength(0);
  });

  it("keeps only the newest few", () => {
    let state: ToastState = emptyToasts;
    for (let i = 1; i <= maxVisibleToasts + 2; i++) state = toastReducer(state, show(`t${i}`));
    expect(state.toasts.map((t) => t.message)).toEqual(["t3", "t4", "t5"]);
    expect(state.nextId).toBe(maxVisibleToasts + 3);
  });

  it("dismisses by id, and ignores an id already gone", () => {
    const state = toastReducer(toastReducer(emptyToasts, show("a")), show("b"));
    const after = toastReducer(state, { type: "dismiss", id: 1 });
    expect(after.toasts.map((t) => t.message)).toEqual(["b"]);
    expect(toastReducer(after, { type: "dismiss", id: 1 })).toEqual(after);
  });
});
