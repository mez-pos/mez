// What a report screen says in its figures' place: that no days are picked,
// that the range is being added up, or that it could not be read.

import { ApiError } from "../../../shared/api";
import { EmptyState, InboxIcon } from "../../../shared/ui";

interface ReportStateProps {
  picked: boolean;
  pending: boolean;
  error: unknown;
  onRetry: () => void;
}

export function ReportState({ picked, pending, error, onRetry }: ReportStateProps) {
  if (!picked) return <p className="text-base text-fg-muted">Pick a first and a last day.</p>;
  if (error) {
    return (
      <EmptyState
        icon={<InboxIcon />}
        line={error instanceof ApiError && !error.unreachable ? error.message : "Can't reach the mez server. Check this computer's connection."}
        action={{ label: "Try again", onClick: onRetry }}
      />
    );
  }
  if (pending) return <p className="text-base text-fg-muted">Adding up the range</p>;
  return null;
}
