// The print queue in the header (ADR-081): always one tap away, and loud
// while anything has failed to print, so a receipt that never came out is
// noticed before the guest asks for it.

import { useState } from "react";
import { Button } from "../../../shared/ui";
import { trouble } from "./print-queue";
import { PrintQueueSheet } from "./PrintQueueSheet";
import { usePrintJobs } from "./use-print-jobs";

export function PrintStatus() {
  const queue = usePrintJobs();
  const [open, setOpen] = useState(false);
  const failing = trouble(queue.jobs);
  return (
    <>
      {failing > 0 ? (
        <Button variant="danger" onClick={() => setOpen(true)} aria-label={`Print queue: ${failing} not printed`}>
          Not printed <span className="font-mono tabular-nums">{failing}</span>
        </Button>
      ) : (
        <Button variant="ghost" onClick={() => setOpen(true)}>
          Print queue
        </Button>
      )}
      {open && <PrintQueueSheet queue={queue} onClose={() => setOpen(false)} />}
    </>
  );
}
