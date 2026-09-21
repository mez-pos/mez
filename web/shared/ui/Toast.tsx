// Transient confirmation: "Ticket sent to bar". Never for an error that needs
// a decision; see toast-queue.ts, which has no tone for one.
//
// One ToastProvider per app, near the root; any component calls useToast().
// Toasts are announced politely, dismiss themselves, and can be dismissed by
// hand. A toast shown while a Modal is open sits behind its scrim, which is
// the right order: the decision in front, the news behind it.

import { createContext, useCallback, useContext, useEffect, useReducer, type ReactNode } from "react";
import { Button } from "./Button";
import { cx } from "./cx";
import { CloseIcon } from "./icons";
import { emptyToasts, toastReducer, type Toast, type ToastTone } from "./toast-queue";

export interface ShowToast {
  (message: string, options?: { tone?: ToastTone; durationMs?: number }): void;
}

const ToastContext = createContext<ShowToast | null>(null);

// Long enough to read two lines at arm's length, short enough not to cover
// the cart for a whole transaction.
const defaultDurationMs = 4000;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(toastReducer, emptyToasts);

  const show = useCallback<ShowToast>((message, options = {}) => {
    dispatch({ type: "show", message, tone: options.tone ?? "neutral", durationMs: options.durationMs ?? defaultDurationMs });
  }, []);
  const dismiss = useCallback((id: number) => dispatch({ type: "dismiss", id }), []);

  return (
    <ToastContext.Provider value={show}>
      {children}
      <div
        role="status"
        aria-live="polite"
        className="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex flex-col items-center gap-2 p-4"
      >
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

const dots: Record<ToastTone, string> = {
  neutral: "text-fg-muted",
  success: "text-success",
  warning: "text-warning",
};

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: (id: number) => void }) {
  useEffect(() => {
    const timer = setTimeout(() => onDismiss(toast.id), toast.durationMs);
    return () => clearTimeout(timer);
  }, [toast.id, toast.durationMs, onDismiss]);

  return (
    <div className="pointer-events-auto flex w-full max-w-md items-center gap-3 rounded border border-line-strong bg-raised-high py-1 pl-4 pr-1 text-fg">
      <span aria-hidden="true" className={cx("size-2 shrink-0 rounded-round bg-current", dots[toast.tone])} />
      <p className="flex-1 text-base">{toast.message}</p>
      <Button variant="ghost" aria-label="Dismiss" onClick={() => onDismiss(toast.id)}>
        <CloseIcon />
      </Button>
    </div>
  );
}
