// The "..." at the end of a row in the back office (D18, D19): what can be
// done to the row's thing without opening it. A disclosure of plain buttons
// (use-disclosure.ts).

import { EllipsisIcon } from "lucide-react";
import { useDisclosure } from "../../shared/shell/use-disclosure";
import { cx } from "../../shared/ui/cx";

export interface RowMenuProps {
  name: string;
  disabled: boolean;
  /** Opens above the button, not below. */
  up: boolean;
  actions: readonly { label: string; run: () => void }[];
  /** Nothing is drawn: what the row could do cannot be done here. */
  hidden?: boolean;
}

/** The "..." at the end of a row. */
export function RowMenu({ name, disabled, up, actions, hidden = false }: RowMenuProps) {
  const { open, toggle, box, trigger, panel, panelId, pick, leaves } = useDisclosure();
  if (hidden) return null;
  return (
    <div ref={box} onBlur={leaves} className="relative shrink-0">
      <button
        ref={trigger}
        type="button"
        aria-label={`${name}: more actions`}
        aria-expanded={open}
        aria-controls={panelId}
        // Not disabled while an action runs: a disabled button drops the focus a choice just gave back to it.
        aria-disabled={disabled || undefined}
        onClick={disabled ? undefined : toggle}
        className={cx(
          "flex size-12 items-center justify-center rounded text-fg-muted outline-offset-2 outline-focus hover:bg-raised-high focus-visible:outline-3",
          disabled && "opacity-40",
        )}
      >
        <EllipsisIcon aria-hidden="true" size={18} />
      </button>
      {open && (
        <div
          ref={panel}
          id={panelId}
          role="group"
          aria-label={`${name}: actions`}
          className={cx("absolute right-0 z-20 w-52 rounded-card border border-line bg-raised p-2 shadow-card", up ? "bottom-full mb-1" : "top-full mt-1")}
        >
          {actions.map((a) => (
            <button
              key={a.label}
              type="button"
              onClick={pick(a.run)}
              className="flex min-h-tap w-full items-center rounded px-4 text-left text-base outline-offset-2 outline-focus hover:bg-raised-high focus-visible:outline-3"
            >
              {a.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
