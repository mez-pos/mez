// A modal: a confirmation or a short form. Escape closes it, and so does a
// close control that is always visible (docs/design/components.md). A click
// on the scrim does not: a half-filled void reason is not lost to a stray tap.

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
  size?: "sm" | "md";
}

const widths = { sm: "max-w-sm", md: "max-w-lg" } as const;

export function Modal({ open, onClose, title, children, footer, size = "sm" }: ModalProps) {
  const ref = useDialog(open, onClose);
  const titleId = useId();

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      className={cx("m-auto w-full rounded border border-line bg-raised p-0 text-fg", widths[size])}
    >
      {open && (
        <div className="flex flex-col gap-4 p-6">
          <div className="flex items-start justify-between gap-4">
            <h2 id={titleId} className="pt-2.5 text-xl font-semibold">
              {title}
            </h2>
            <Button variant="ghost" aria-label="Close" onClick={onClose} className="-mr-2 -mt-1 shrink-0">
              <CloseIcon />
            </Button>
          </div>
          <div className="text-base text-fg-2">{children}</div>
          {footer && <div className="flex flex-wrap justify-end gap-tap-gap">{footer}</div>}
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
