// Nothing to show yet: an icon, one line, and at most one action
// (docs/design/components.md). One line, because an empty screen during
// service is read in a glance; one action, because two is a decision.

import type { ReactNode } from "react";
import { Button } from "./Button";
import { cx } from "./cx";

export interface EmptyStateProps {
  icon: ReactNode;
  line: string;
  action?: { label: string; onClick: () => void };
  className?: string;
}

export function EmptyState({ icon, line, action, className }: EmptyStateProps) {
  return (
    <div className={cx("flex flex-col items-center gap-4 px-6 py-12 text-center", className)}>
      <div aria-hidden="true" className="flex size-14 items-center justify-center rounded-round bg-sunken text-fg-muted">
        {icon}
      </div>
      <p className="max-w-md font-serif text-lg">{line}</p>
      {action && <Button onClick={action.onClick}>{action.label}</Button>}
    </div>
  );
}
