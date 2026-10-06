// The screen for when the venue cannot be shown (docs/design/screens/
// guest-landing.md, States; M11d and M11e). It says what happened in words a
// guest can act on, and always names the counter: nobody leaves hungry
// because a phone could not reach a kitchen.

import { Button } from "../../shared/ui/Button";
import { Logo } from "../../shared/ui/Logo";
import type { Problem as Kind } from "./menu-logic";

const words: Record<Kind, { title: string; line: string }> = {
  unknown_code: {
    title: "This code does not lead to a table",
    line: "Scan the code on your table again, or ask a member of staff.",
  },
  collect_off: {
    title: "Not taking orders to collect online",
    line: "Please order at the counter.",
  },
  unreachable: {
    title: "Can't reach the kitchen",
    line: "Check your connection and try again. You can always order at the counter.",
  },
  failed: {
    title: "Something went wrong",
    line: "Try again in a moment, or order at the counter.",
  },
};

/** A pickup code that leads nowhere: nobody who scanned a poster is at a table. */
const unknownCollect = {
  title: "This code does not lead anywhere",
  line: "Scan the code again, or order at the counter.",
};

interface ProblemProps {
  kind: Kind;
  /** The code was a venue's pickup code, so nothing is said of a table. */
  collect?: boolean;
  /** Asks the venue again. Left out when asking again cannot help: a code is wrong until it is scanned again. */
  onRetry?: (() => void) | undefined;
}

export function Problem({ kind, collect = false, onRetry }: ProblemProps) {
  const { title, line } = collect && kind === "unknown_code" ? unknownCollect : words[kind];
  return (
    <main className="flex min-h-dvh flex-col items-center justify-center gap-6 bg-canvas p-6 text-center text-fg">
      <Logo className="h-10" />
      <div role="alert" className="w-full max-w-sm rounded-card border border-line bg-raised p-6 shadow-card">
        <h1 className="font-serif text-xl">{title}</h1>
        <p className="mt-2 text-base text-fg-2">{line}</p>
        {onRetry && (
          <Button variant="secondary" className="mt-5" onClick={onRetry}>
            Try again
          </Button>
        )}
      </div>
    </main>
  );
}
