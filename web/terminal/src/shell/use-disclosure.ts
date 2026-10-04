// The behaviour of a menu that opens under its button (ProfileMenu, the order
// rail's "..."). It is a disclosure, not an ARIA menu: plain buttons, reached by
// Tab, with focus moved in on opening, returned to the button on Escape, and the
// whole thing closed by a tap elsewhere, by Tab leaving it, and after a choice.

import { useEffect, useId, useRef, useState, type FocusEvent, type RefObject } from "react";

export interface Disclosure {
  open: boolean;
  toggle: () => void;
  box: RefObject<HTMLDivElement>;
  trigger: RefObject<HTMLButtonElement>;
  panel: RefObject<HTMLDivElement>;
  panelId: string;
  /** Wrap a choice: it closes the menu, gives focus back, then acts. */
  pick: (action: () => void) => () => void;
  /** For the box's onBlur: Tab leaving the box closes it. */
  leaves: (e: FocusEvent<HTMLDivElement>) => void;
}

export function useDisclosure(): Disclosure {
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

  return {
    open,
    toggle: () => setOpen((o) => !o),
    box,
    trigger,
    panel,
    panelId,
    pick: (action) => () => {
      setOpen(false);
      trigger.current?.focus();
      action();
    },
    leaves: (e) => {
      if (open && !box.current?.contains(e.relatedTarget)) setOpen(false);
    },
  };
}
