// Transient confirmation: "Ticket sent to bar". Never for an error that needs
// a decision; see toast-queue.ts, which has no tone for one.
//
// One ToastProvider per app, near the root; any component calls useToast().
// Toasts are announced politely, dismiss themselves, and can be dismissed by
// hand. A toast shown while a Modal is open sits behind its scrim, which is
// the right order: the decision in front, the news behind it.

import { createContext, useCallback, useContext, useEffect, useReducer, type ReactNode } from "react";
import { cx } from "./cx";
import { CloseIcon } from "./icons";
import { emptyToasts, toastReducer, type Toast, type ToastButton, type ToastTone } from "./toast-queue";

export interface ShowToast {
  /** A button takes back what the toast reports ("Undo"); the toast then stays longer, to be reached. */
  (message: string, options?: { tone?: ToastTone; durationMs?: number; button?: ToastButton }): void;
}

const ToastContext = createContext<ShowToast | null>(null);

// Long enough to read two lines at arm's length, short enough not to cover
// the cart for a whole transaction.
const defaultDurationMs = 4000;
// Long enough to read it, find the button and reach it with a mouse.
const buttonDurationMs = 8000;

export interface ToastProviderProps {
  children: ReactNode;
  /**
   * Where toasts stack. The terminal's are at the top, under its header: at
   * the bottom they sat over the last row of dishes and the charge button
   * (TILL-11).
   */
  placement?: "top" | "bottom";
}

// Clears the terminal's header at its tallest (the waiter's, 88px; 72px short).
const stacks = {
  top: "top-0 pt-24 short:pt-20",
  bottom: "bottom-0 p-4",
} as const;

export function ToastProvider({ children, placement = "bottom" }: ToastProviderProps) {
  const [state, dispatch] = useReducer(toastReducer, emptyToasts);

  const show = useCallback<ShowToast>((message, options = {}) => {
    dispatch({
      type: "show",
      message,
      tone: options.tone ?? "neutral",
      durationMs: options.durationMs ?? (options.button ? buttonDurationMs : defaultDurationMs),
      ...(options.button ? { button: options.button } : {}),
    });
  }, []);
  const dismiss = useCallback((id: number) => dispatch({ type: "dismiss", id }), []);

  return (
    <ToastContext.Provider value={show}>
      {children}
      <div role="status" aria-live="polite" className={cx("pointer-events-none fixed inset-x-0 z-50 flex flex-col items-center gap-2 px-4", stacks[placement])}>
        {state.toasts.map((toast) => (
          <ToastItem key={toast.id} toast={toast} onDismiss={dismiss} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ShowToast {
  const show = useContext(ToastContext);
  if (!show) throw new Error("useToast needs a ToastProvider above it");
  return show;
}

// Good news is the green fill; a warning is the warning chip's fill and ink,
// with no stripe down its side (ADR-390); the rest is a plain card.
const shells: Record<ToastTone, string> = {
  neutral: "border-line-strong bg-raised text-fg shadow-card",
  success: "border-accent bg-accent text-accent-fg shadow-action",
  warning: "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg shadow-card",
};

const dots: Record<ToastTone, string> = {
  neutral: "text-fg-muted",
  success: "text-accent-fg",
  warning: "text-chip-warning-fg",
};

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: (id: number) => void }) {
  useEffect(() => {
    const timer = setTimeout(() => onDismiss(toast.id), toast.durationMs);
    return () => clearTimeout(timer);
  }, [toast.id, toast.durationMs, onDismiss]);

  return (
    // The message lets a tap through to what is under it; only Dismiss takes one.
    <div className={cx("flex w-full max-w-md items-center gap-3 rounded border py-1 pl-4 pr-1", shells[toast.tone])}>
      <span aria-hidden="true" className={cx("size-2 shrink-0 rounded-round bg-current", dots[toast.tone])} />
      <p className="flex-1 text-base font-medium">{toast.message}</p>
      {toast.button && (
        // Not a Button either, for the same reason as Dismiss: it sits on any tone's fill.
        <button
          type="button"
          onClick={() => {
            onDismiss(toast.id);
            toast.button?.run();
          }}
          className="pointer-events-auto inline-flex min-h-tap shrink-0 items-center rounded-pill px-3 text-base font-bold underline underline-offset-4 outline-current focus-visible:outline-3"
        >
          {toast.button.label}
        </button>
      )}
      {/* Not a Button: its ghost is terracotta, and this sits on any tone's fill. */}
      <button
        type="button"
        aria-label="Dismiss"
        onClick={() => onDismiss(toast.id)}
        className="pointer-events-auto inline-flex size-tap shrink-0 items-center justify-center rounded-pill outline-current focus-visible:outline-3"
      >
        <CloseIcon />
      </button>
    </div>
  );
}
