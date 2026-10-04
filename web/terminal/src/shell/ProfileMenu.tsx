// Who is signed in, and what they may do about it (flow map): Lock, Close the
// shift, Print queue. The avatar and name are the button; the choices open under
// it. It is a disclosure, not an ARIA menu: plain buttons, reached by Tab, with
// focus moved in on opening, returned to the button on Escape, and the whole
// thing closed by a tap elsewhere, by Tab leaving it, and after a choice.

import { ChevronDownIcon, DoorClosedIcon, LockIcon, PrinterIcon, type LucideIcon } from "lucide-react";
import { useEffect, useId, useRef, useState, type FocusEvent } from "react";
import { cx } from "../../../shared/ui/cx";
import { initials, roleText } from "../../../shared/auth/format";

interface ProfileMenuProps {
  name: string;
  role: string;
  /** Closing the shift needs shift.manage and a shift to close. */
  canCloseShift: boolean;
  onLock: () => void;
  onCloseShift: () => void;
  onPrintQueue: () => void;
}

export function ProfileMenu({ name, role, canCloseShift, onLock, onCloseShift, onPrintQueue }: ProfileMenuProps) {
  const [open, setOpen] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const panelId = useId();

  // Focus goes to the first choice when it opens, so a keyboard has somewhere to start.
  useEffect(() => {
    if (open) panel.current?.querySelector("button")?.focus();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const away = (e: PointerEvent) => {
      if (box.current && !box.current.contains(e.target as Node)) setOpen(false);
    };
    const escape = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        trigger.current?.focus();
      }
    };
    document.addEventListener("pointerdown", away);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("pointerdown", away);
      document.removeEventListener("keydown", escape);
    };
  }, [open]);

  const pick = (action: () => void) => () => {
    setOpen(false);
    trigger.current?.focus();
    action();
  };

  // Tab out of the choices, or out of the button past them, closes it.
  const leaves = (e: FocusEvent<HTMLDivElement>) => {
    if (open && !box.current?.contains(e.relatedTarget)) setOpen(false);
  };

  return (
    <div ref={box} onBlur={leaves} className="relative">
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((o) => !o)}
        className="flex min-h-tap items-center gap-2.5 rounded-pill pl-1 pr-3 outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
      >
        <span aria-hidden="true" className="flex size-10 items-center justify-center rounded-round bg-accent-tint text-sm font-bold text-accent">
          {initials(name) || "?"}
        </span>
        <span className="text-left leading-tight">
          <span className="block text-base font-bold">{name}</span>
          <span className="block text-sm text-fg-muted">{roleText(role)}</span>
        </span>
        <ChevronDownIcon aria-hidden="true" size={16} className={cx("text-fg-muted transition-transform", open && "rotate-180")} />
      </button>
      {open && (
        <div
          ref={panel}
          id={panelId}
          role="group"
          aria-label="Your account"
          className="absolute right-0 top-full z-20 mt-2 w-64 rounded-card border border-line bg-raised p-2 shadow-card"
        >
          <Item icon={LockIcon} label="Lock" onClick={pick(onLock)} />
          {canCloseShift && <Item icon={DoorClosedIcon} label="Close the shift" onClick={pick(onCloseShift)} />}
          <Item icon={PrinterIcon} label="Print queue" onClick={pick(onPrintQueue)} />
        </div>
      )}
    </div>
  );
}

function Item({ icon: Icon, label, onClick }: { icon: LucideIcon; label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex min-h-tap w-full items-center gap-3 rounded px-4 text-left text-base outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
    >
      <Icon aria-hidden="true" size={18} />
      {label}
    </button>
  );
}
