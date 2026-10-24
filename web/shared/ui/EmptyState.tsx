// Nothing to show yet: an icon, one line, and at most one action
// (docs/design/components.md). One line, because an empty screen during
// service is read in a glance; one action, because two is a decision.

import type { ReactNode } from "react";
import { Button } from "./Button";
import { cx } from "./cx";

export interface EmptyStateProps {
  icon: ReactNode;
  line: string;
  /** What to do about it: a button, or a link to another page (the back office, another bundle). */
  action?: { label: string; onClick: () => void } | { label: string; href: string };
  className?: string;
}

export function EmptyState({ icon, line, action, className }: EmptyStateProps) {
  return (
    <div className={cx("flex flex-col items-center gap-4 px-6 py-12 text-center", className)}>
      <div aria-hidden="true" className="flex size-14 items-center justify-center rounded-round bg-sunken text-fg-muted">
        {icon}
      </div>
      <p className="max-w-md font-serif text-lg">{line}</p>
      {action &&
        ("href" in action ? (
          // A plain link, drawn as the button: the page it leads to is another bundle.
          <a
            href={action.href}
            className="inline-flex min-h-tap items-center justify-center rounded-pill border border-line-strong bg-raised px-5 text-base font-semibold text-fg outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
          >
            {action.label}
          </a>
        ) : (
          <Button onClick={action.onClick}>{action.label}</Button>
        ))}
    </div>
  );
}
