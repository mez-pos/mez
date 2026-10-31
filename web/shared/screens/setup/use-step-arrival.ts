// A new step arrives at the top of the page with its heading focused (SET-3).
// Continue sits at the foot of the step before it: the page stayed scrolled
// where it was, and a keyboard or a screen reader was left on a button whose
// step had gone. A step that puts the focus in a field of its own (the first
// field of "Your venue" and "You", the phone number of "I had mez before")
// keeps it there.
//
// The heading is the card's first h2. The steps draw their own, so one with no
// tabindex is given -1 here: focusable by script, never by Tab.

import { useEffect, useRef, type RefObject } from "react";

export function useStepArrival(step: string | null, page: RefObject<HTMLElement>, card: RefObject<HTMLElement>): void {
  const shown = useRef(step);

  useEffect(() => {
    if (step === shown.current) return;
    shown.current = step;
    const here = card.current;
    if (step === null || !here) return;
    page.current?.scrollTo({ top: 0 });
    const active = document.activeElement;
    if (active instanceof HTMLElement && here.contains(active) && active.matches("input, select, textarea")) return;
    const heading = here.querySelector("h2");
    if (!heading) return;
    if (!heading.hasAttribute("tabindex")) heading.tabIndex = -1;
    heading.focus({ preventScroll: true });
  }, [step, page, card]);
}
