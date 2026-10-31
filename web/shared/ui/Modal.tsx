// A modal: a confirmation or a short form. Escape closes it, and so does a
// close control that is always visible (docs/design/components.md). A click
// on the scrim does not: a half-filled void reason is not lost to a stray tap.
//
// On a short screen the body scrolls and the title and footer stay put, so
// the action a manager's PIN is typed for is never below the fold (TECH-2).

import { useId, type ReactNode } from "react";
import { Button, type ButtonVariant } from "./Button";
import { cx } from "./cx";
import { CloseIcon } from "./icons";
import { useDialog } from "./use-dialog";

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  /** Actions, right-aligned under the body. */
  footer?: ReactNode;
  size?: "sm" | "md" | "lg";
  /** A round mark over the title, for a modal that asks something serious. */
  icon?: ReactNode;
}

const widths = { sm: "max-w-sm", md: "max-w-lg", lg: "max-w-2xl" } as const;

export function Modal({ open, onClose, title, children, footer, size = "sm", icon }: ModalProps) {
  const ref = useDialog(open, onClose);
  const titleId = useId();

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      className={cx("m-auto w-full rounded-sheet border border-line bg-raised p-0 text-fg shadow-modal", widths[size])}
    >
      {open && (
        // Under the dialog's own height limit (about 100dvh less 2em), so the
        // dialog never scrolls as a whole; only the body does.
        <div className="flex max-h-[calc(100dvh-3rem)] flex-col">
          <div className="flex items-start justify-between gap-4 px-6 pt-6 short:pt-5">
            <div className="min-w-0">
              {icon && (
                <span aria-hidden="true" className="mb-3 flex size-14 items-center justify-center rounded-round bg-danger-tint text-danger">
                  {icon}
                </span>
              )}
              <h2 id={titleId} className={icon ? "font-serif text-xl 2xl:text-total" : "pt-2.5 font-serif text-xl"}>
                {title}
              </h2>
            </div>
            <Button variant="ghost" aria-label="Close" onClick={onClose} className="-mr-2 -mt-1 shrink-0">
              <CloseIcon />
            </Button>
          </div>
          <div className={cx("min-h-0 overflow-y-auto px-6 pt-4 text-base text-fg-2", footer ? "pb-2" : "pb-6 short:pb-5")}>{children}</div>
          {footer && <div className="flex flex-wrap justify-end gap-tap-gap px-6 pb-6 pt-2 short:pb-5">{footer}</div>}
        </div>
      )}
    </dialog>
  );
}

export interface ConfirmProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  children: ReactNode;
  confirmLabel: string;
  /** `danger` for a void or a refund; `primary` when it moves money forward. */
  tone?: Extract<ButtonVariant, "primary" | "danger">;
}

/** Yes or no. The confirm label says what happens: "Void line", never "OK". */
export function Confirm({ open, onClose, onConfirm, title, children, confirmLabel, tone = "primary" }: ConfirmProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant={tone} onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      {children}
    </Modal>
  );
}
