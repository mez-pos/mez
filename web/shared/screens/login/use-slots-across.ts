// How many tiles a row holds, measured rather than assumed (ADR-255): the
// row's own width, its first tile's width and the gap between them, read again
// whenever the row changes size (a window resized, a tablet turned round).
// Every tile in the row, Back and More included, is the same width, so one of
// them stands for all.

import { useLayoutEffect, useRef, useState, type RefObject } from "react";
import { slotsAcross } from "./login-logic";

export function useSlotsAcross<T extends HTMLElement>(): [RefObject<T>, number] {
  const row = useRef<T>(null);
  const [slots, setSlots] = useState(0);

  // Before paint, so the first frame already shows a whole row.
  useLayoutEffect(() => {
    const el = row.current;
    if (!el) return;
    const measure = () => {
      const tile = el.firstElementChild?.getBoundingClientRect().width ?? 0;
      const gap = Number.parseFloat(getComputedStyle(el).columnGap);
      setSlots(slotsAcross(el.getBoundingClientRect().width, tile, gap));
    };
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return [row, slots];
}
