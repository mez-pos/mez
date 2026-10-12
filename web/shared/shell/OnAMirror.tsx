// What a screen says in the place of a change, when the back office is open
// on a mirror: a copy of the venue from its backup (ADR-326, ADR-330). One
// sentence, the same everywhere, as AtTheVenue is for what cannot be done
// from away. A mirror refuses every change either way; this is so nobody
// fills in a form to be told afterwards.

import { HistoryIcon } from "lucide-react";
import { cx } from "../ui/cx";

/** The sentence, where there is no room or no need for one of a screen's own. */
export const onAMirrorText = "This is a copy to read. Changes are made at the venue.";

interface OnAMirrorProps {
  /** What it is that a copy does not have, when the default sentence would not say. */
  children?: string;
  className?: string;
}

export function OnAMirror({ children = onAMirrorText, className }: OnAMirrorProps) {
  return (
    <p className={cx("flex items-center gap-2 text-base text-fg-2", className)}>
      <HistoryIcon aria-hidden="true" size={17} className="shrink-0 text-fg-muted" />
      {children}
    </p>
  );
}
