// One hold on the page's scroll for however many dialogs are open at once.
// Each dialog that opens takes a hold; the page scrolls again when the last
// one is let go, whichever order they close in. A lock kept per dialog
// would, with two open and closed in the wrong order, leave the page stuck.

interface Scrollable {
  style: { overflow: string };
}

let holds = 0;
let before = "";

/** Stops the page scrolling. Returns the release of this one hold; releasing twice is nothing. */
export function lockScroll(root: Scrollable = document.documentElement): () => void {
  if (holds === 0) {
    before = root.style.overflow;
    root.style.overflow = "hidden";
  }
  holds += 1;
  let held = true;
  return () => {
    if (!held) return;
    held = false;
    holds -= 1;
    if (holds === 0) root.style.overflow = before;
  };
}
