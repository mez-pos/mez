// Who is signed in, and what they may do about it (flow map): Lock, Close the
// shift, Print queue. The avatar and name are the button; the choices open under
// it, as a disclosure (use-disclosure.ts).

import { ChevronDownIcon, DoorClosedIcon, LockIcon, PrinterIcon, type LucideIcon } from "lucide-react";
import { cx } from "../../../shared/ui/cx";
import { initials, roleText } from "../../../shared/auth/format";
import { useDisclosure } from "./use-disclosure";

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
  const { open, toggle, box, trigger, panel, panelId, pick, leaves } = useDisclosure();

  return (
    <div ref={box} onBlur={leaves} className="relative">
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={toggle}
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
