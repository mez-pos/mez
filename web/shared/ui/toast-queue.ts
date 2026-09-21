// The toast queue, as a reducer, apart from the component.
//
// There is no "danger" tone, on purpose. A toast disappears on its own, so it
// must never carry an error someone has to act on (docs/design/components.md):
// that is a Modal. Leaving the tone out makes the rule a type error instead of
// a review comment.

export type ToastTone = "neutral" | "success" | "warning";

export interface Toast {
  readonly id: number;
  readonly tone: ToastTone;
  readonly message: string;
  readonly durationMs: number;
}

export interface ToastState {
  readonly toasts: readonly Toast[];
  readonly nextId: number;
}

export type ToastAction =
  | { readonly type: "show"; readonly tone: ToastTone; readonly message: string; readonly durationMs: number }
  | { readonly type: "dismiss"; readonly id: number };

/** More than this at once is noise; the oldest goes first. */
export const maxVisibleToasts = 3;

export const emptyToasts: ToastState = { toasts: [], nextId: 1 };

export function toastReducer(state: ToastState, action: ToastAction): ToastState {
  switch (action.type) {
    case "show": {
      const toast: Toast = { id: state.nextId, tone: action.tone, message: action.message, durationMs: action.durationMs };
      return { toasts: [...state.toasts, toast].slice(-maxVisibleToasts), nextId: state.nextId + 1 };
    }
    case "dismiss":
      return { ...state, toasts: state.toasts.filter((t) => t.id !== action.id) };
  }
}
