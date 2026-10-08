// Who is signed in, and what they may do about it. The avatar and name are the
// button; the choices open under it, as a disclosure (use-disclosure.ts). The
// terminal gives it Lock, Close the shift and Print queue (flow map); the back
// office gives it Sign out (D12).

import { ChevronDownIcon, type LucideIcon } from "lucide-react";
import { initials, roleText } from "../auth/format";
import { cx } from "../ui/cx";
import { useDisclosure } from "./use-disclosure";

export interface ProfileAction {
  icon: LucideIcon;
  label: string;
  onClick: () => void;
}

interface ProfileMenuProps {
  name: string;
  role: string;
  /** The choices, in the order they are listed. */
  actions: readonly ProfileAction[];
  /**
   * Under 640px the button is the initials alone, and the name and role
   * are in the panel it opens: for a bar that is also shown on a phone.
   */
  compact?: boolean;
}

export function ProfileMenu({ name, role, actions, compact = false }: ProfileMenuProps) {
  const { open, toggle, box, trigger, panel, panelId, pick, leaves } = useDisclosure();

  return (
    <div ref={box} onBlur={leaves} className="relative">
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={toggle}
        aria-label={compact ? `${name}, ${roleText(role)}` : undefined}
        className={cx(
          "flex min-h-tap shrink-0 items-center gap-2.5 rounded-pill pl-1 outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2",
          compact ? "pr-1 sm:pr-3" : "pr-3",
        )}
      >
        <span aria-hidden="true" className="flex size-10 items-center justify-center rounded-round bg-accent-tint text-sm font-bold text-accent">
          {initials(name) || "?"}
        </span>
        <span className={cx("text-left leading-tight", compact && "hidden sm:block")}>
          <span className="block text-base font-bold">{name}</span>
          <span className="block text-sm text-fg-muted">{roleText(role)}</span>
        </span>
        <ChevronDownIcon
          aria-hidden="true"
          size={16}
          className={cx("text-fg-muted transition-transform", open && "rotate-180", compact && "hidden sm:block")}
        />
      </button>
      {open && (
        <div
          ref={panel}
          id={panelId}
          role="group"
          aria-label="Your account"
          className="absolute right-0 top-full z-20 mt-2 w-64 rounded-card border border-line bg-raised p-2 shadow-card"
        >
          {compact && (
            <p className="border-b border-line px-4 pb-2 pt-1 leading-tight sm:hidden">
              <span className="block text-base font-bold">{name}</span>
              <span className="block text-sm text-fg-muted">{roleText(role)}</span>
            </p>
          )}
          {actions.map((a) => (
            <Item key={a.label} icon={a.icon} label={a.label} onClick={pick(a.onClick)} />
          ))}
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
