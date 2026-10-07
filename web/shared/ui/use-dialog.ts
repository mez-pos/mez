// Drives a native <dialog> from an `open` prop. The platform gives the rest:
// showModal() puts it in the top layer, makes the page behind it inert, moves
// focus in and back out, and turns Escape into a "cancel" event. Modal and
// Sheet are both built on this.

import { useEffect, useLayoutEffect, useRef } from "react";
import { lockScroll } from "./scroll-lock";

export function useDialog(open: boolean, onClose: () => void) {
  const ref = useRef<HTMLDialogElement>(null);
  const latest = useRef(onClose);
  // What the owner last said. The browser fires "close" a task later, even
  // for a close this hook made, so the event alone cannot say who closed it.
  const wanted = useRef(open);
  useLayoutEffect(() => {
    latest.current = onClose;
    wanted.current = open;
  });

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    // Escape asks; the owner decides. Without preventDefault the dialog would
    // close itself and disagree with the `open` it was given.
    const cancel = (event: Event) => {
      event.preventDefault();
      latest.current();
    };
    // Closed some other way (a <form method="dialog">): tell the owner.
    const closed = () => {
      if (wanted.current) latest.current();
    };
    dialog.addEventListener("cancel", cancel);
    dialog.addEventListener("close", closed);
    return () => {
      dialog.removeEventListener("cancel", cancel);
      dialog.removeEventListener("close", closed);
    };
  }, []);

  // The page behind must not scroll under a finger dragging inside a sheet.
  // The hold is shared with every other open dialog (scroll-lock.ts).
  useEffect(() => {
    if (!open) return;
    return lockScroll();
  }, [open]);

  return ref;
}
