// What the kitchen has not seen yet, on the payment screen (TILL-1, ADR-383).
// A counter sale's waiting lines go with the payment that settles it, which
// is said quietly; a table's ticket does not send itself, so its lines are a
// warning, in a word and the warning tint, with Send now beside them.

import { SendIcon } from "lucide-react";
import { Button } from "../../../shared/ui";

interface UnsentNoteProps {
  words: string;
  /** Send now, for a table's ticket; null when the payment sends them. */
  onSend: (() => void) | null;
  busy: boolean;
}

export function UnsentNote({ words, onSend, busy }: UnsentNoteProps) {
  if (!onSend) {
    return (
      <p className="flex items-center gap-2 text-sm text-fg-2">
        <SendIcon aria-hidden="true" size={16} className="shrink-0" />
        {words}
      </p>
    );
  }
  return (
    <div
      role="status"
      className="flex flex-wrap items-center justify-between gap-tap-gap rounded-card border border-chip-warning-border bg-chip-warning-bg py-2 pl-4 pr-2"
    >
      <p className="text-base font-semibold text-chip-warning-fg">{words}</p>
      <Button disabled={busy} onClick={onSend}>
        <SendIcon aria-hidden="true" size={17} />
        {busy ? "Sending" : "Send now"}
      </Button>
    </div>
  );
}
