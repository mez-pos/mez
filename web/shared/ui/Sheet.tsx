// A sheet: from the bottom on a touch screen, from the right with a mouse
// (docs/design/components.md). The terminal is a 12-inch touch tablet, so
// "touch" is decided by the pointer, not by width: a wide screen with a
// finger on it still gets a bottom sheet, within thumb reach.
//
// Escape, the close control and a tap on the scrim all close it, except that a
// sheet holding something typed or chosen (holdsForm) is not closed by the
// scrim, as a Modal never is: a stray tap does not throw away a half-typed
// note or approval (TECH-9, ADR-387). There is no drag to dismiss: a drag must
// never be the only path to an action (tokens.md).

import { useId, type MouseEvent, type ReactNode } from "react";
import { Button } from "./Button";
import { cx } from "./cx";
import { ErrorBoundary } from "./ErrorBoundary";
import { CloseIcon } from "./icons";
import { useDialog } from "./use-dialog";

export interface SheetProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  /** Stays in view under a scrolling body: the charge button, a save. */
  footer?: ReactNode;
  /** It holds something typed or chosen: a tap on the scrim leaves it open. Escape and Close still close it. */
  holdsForm?: boolean;
}

export function Sheet({ open, onClose, title, children, footer, holdsForm = false }: SheetProps) {
  const ref = useDialog(open, onClose);
  const titleId = useId();

  // The panel fills the dialog, so a click whose target is the dialog itself
  // landed on the scrim around it.
  const scrimClick = (event: MouseEvent<HTMLDialogElement>) => {
    if (event.target === event.currentTarget && !holdsForm) onClose();
  };

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      onClick={scrimClick}
      className={cx(
        "max-h-none max-w-none bg-transparent p-0 text-fg",
        // Touch: a bottom sheet across the width, up to most of the height.
        // No height class: the dialog keeps its fit-content height, and the auto
        // top margin pushes it to the bottom. h-auto would stretch it to full.
        "mb-0 mt-auto w-full",
        // Mouse: a panel down the right-hand side.
        "pointer-fine:ml-auto pointer-fine:mr-0 pointer-fine:mt-0 pointer-fine:h-full pointer-fine:w-96",
      )}
    >
      {open && (
        <div
          className={cx(
            "flex max-h-[85dvh] flex-col border-line bg-raised",
            // A phone drawn to its edges (the guest app's viewport-fit=cover):
            // the panel's ground runs under the home bar and the notch, and
            // its head, body and foot stay inside them (GST-6). Nothing on a
            // screen without insets.
            "pr-[env(safe-area-inset-right)] pb-[env(safe-area-inset-bottom)] pl-[env(safe-area-inset-left)]",
            "rounded-t-sheet border-t shadow-modal",
            "pointer-fine:h-full pointer-fine:max-h-none pointer-fine:rounded-none pointer-fine:border-l pointer-fine:border-t-0",
          )}
        >
          <div className="flex items-center justify-between gap-4 border-b border-line px-6 py-2">
            <h2 id={titleId} className="font-serif text-xl">
              {title}
            </h2>
            <Button variant="ghost" aria-label="Close" onClick={onClose} className="-mr-2 shrink-0">
              <CloseIcon />
            </Button>
          </div>
          {/* A body that fails to draw takes its actions with it: the title and Close stay (TECH-8). */}
          <ErrorBoundary where="modal" size="modal" onClose={onClose}>
            <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">{children}</div>
            {footer && <div className="flex flex-wrap justify-end gap-tap-gap border-t border-line px-6 py-4 *:grow">{footer}</div>}
          </ErrorBoundary>
        </div>
      )}
    </dialog>
  );
}
