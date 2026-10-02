// A sheet: from the bottom on a touch screen, from the right with a mouse
// (docs/design/components.md). The terminal is a 12-inch touch tablet, so
// "touch" is decided by the pointer, not by width: a wide screen with a
// finger on it still gets a bottom sheet, within thumb reach.
//
// Escape, the close control and a tap on the scrim all close it. There is no
// drag to dismiss: a drag must never be the only path to an action (tokens.md).

import { useId, type MouseEvent, type ReactNode } from "react";
import { Button } from "./Button";
import { cx } from "./cx";
import { CloseIcon } from "./icons";
import { useDialog } from "./use-dialog";

export interface SheetProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  /** Stays in view under a scrolling body: the charge button, a save. */
  footer?: ReactNode;
}

export function Sheet({ open, onClose, title, children, footer }: SheetProps) {
  const ref = useDialog(open, onClose);
  const titleId = useId();

  // The panel fills the dialog, so a click whose target is the dialog itself
  // landed on the scrim around it.
  const scrimClick = (event: MouseEvent<HTMLDialogElement>) => {
    if (event.target === event.currentTarget) onClose();
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
          <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">{children}</div>
          {footer && <div className="flex flex-wrap justify-end gap-tap-gap border-t border-line px-6 py-4">{footer}</div>}
        </div>
      )}
    </dialog>
  );
}
