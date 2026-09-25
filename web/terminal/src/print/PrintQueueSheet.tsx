// The print queue (docs/design/screens/terminal-print-queue.md): every job
// not printed yet and what printed lately, newest first, each with its
// state as a chip and a word, what went wrong, and Retry or Cancel.

import { useEffect, useState } from "react";
import type { PrintJobResponse } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon, Sheet, StatusChip } from "../../../shared/ui";
import { actionsFor, detailOf } from "./print-queue";
import type { PrintJobs } from "./use-print-jobs";

interface PrintQueueSheetProps {
  queue: PrintJobs;
  onClose: () => void;
}

export function PrintQueueSheet({ queue, onClose }: PrintQueueSheetProps) {
  // "Next try in 8s" counts down while the sheet is open.
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(t);
  }, []);

  return (
    <Sheet open onClose={onClose} title="Print queue">
      <div className="px-6 py-4">
        {queue.loading && <p className="text-base text-fg-muted">Loading the queue</p>}
        {queue.error && (
          <p role="alert" className="text-base text-danger">
            {queue.error}
          </p>
        )}
        {!queue.loading && !queue.error && queue.jobs.length === 0 && (
          <EmptyState icon={<InboxIcon />} line="Nothing to print, and nothing printed in the last two hours." />
        )}
        <ol>
          {queue.jobs.map((job) => (
            <JobRow key={job.id} job={job} now={now} queue={queue} />
          ))}
        </ol>
      </div>
    </Sheet>
  );
}

function JobRow({ job, now, queue }: { job: PrintJobResponse; now: Date; queue: PrintJobs }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const actions = actionsFor(job);
  const act = (fn: (id: number) => Promise<void>) => {
    setBusy(true);
    setError(null);
    fn(job.id)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setBusy(false));
  };
  return (
    <li className="border-b border-line py-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-base font-semibold">{job.label}</p>
          <p className={job.state === "dead" ? "mt-1 text-sm text-danger" : "mt-1 text-sm text-fg-muted"}>{detailOf(job, now)}</p>
        </div>
        <StatusChip status={{ kind: "print", status: job.state }} />
      </div>
      {(actions.retry || actions.cancel) && (
        <div className="mt-2 flex gap-tap-gap">
          {actions.retry && (
            <Button disabled={busy} onClick={() => act(queue.retry)}>
              Retry
            </Button>
          )}
          {actions.cancel && (
            <Button variant="ghost" disabled={busy} onClick={() => act(queue.cancel)}>
              Cancel
            </Button>
          )}
        </div>
      )}
      {error && (
        <p role="alert" className="mt-2 text-sm text-danger">
          {error}
        </p>
      )}
    </li>
  );
}
