// What the print queue sheet says about each job (ADR-081). The jobs and
// their states are the server's; this only words them and says which of
// a person's actions apply.

import type { PrintJobResponse } from "../../../shared/api-types";

/** Jobs that need a person: failing, or given up on. */
export function trouble(jobs: readonly PrintJobResponse[]): number {
  return jobs.filter((j) => j.state === "failed" || j.state === "dead").length;
}

/** Retry for a job that has not printed and is not being tried now; cancel for one still waiting its turn. */
export function actionsFor(job: PrintJobResponse): { retry: boolean; cancel: boolean } {
  return {
    retry: job.state === "failed" || job.state === "dead",
    cancel: job.state === "queued" || job.state === "failed",
  };
}

/** The line under a job's label: where it is going, and what went wrong. */
export function detailOf(job: PrintJobResponse, now: Date): string {
  const printer = job.printer || "no printer";
  switch (job.state) {
    case "queued":
      return `Waiting for ${printer}.`;
    case "printing":
      return `Sending to ${printer}.`;
    case "done":
      return `Printed on ${printer}.`;
    case "failed":
      return `${printer}: ${job.last_error ?? "it failed"}. Try ${job.attempts} of ${job.max_attempts}; ${nextTry(job, now)}`;
    case "dead":
      if (noPrinter(job)) return noPrinterWords;
      return job.last_error ? sentence(job.last_error) : "Gave up. Retry when the printer is ready.";
  }
}

/** The server's words for a job with nowhere to go (printq.NoPrinter), which are for its log, not a cashier. */
function noPrinter(job: PrintJobResponse): boolean {
  return !job.printer && (job.last_error ?? "").startsWith("no printer is set up");
}

/** What a cashier is told of it: who sets a printer up, and where (TILL-19, ADR-394). */
const noPrinterWords = "There is no printer for this. A manager adds one in the back office, under Printers; then Retry.";

/** What became of the receipt the payment printed, for the paid step, or null while the queue has not shown it (TILL-19). */
export interface ReceiptState {
  readonly text: string;
  /** It did not print, and will not by itself. */
  readonly failed: boolean;
}

/**
 * The paid step's line about its receipt, from the print queue: printed,
 * printing, trying again, or not printed and why. It said the receipt "went
 * to the print queue" even when it could not print (TILL-19, ADR-394).
 */
export function receiptState(jobs: readonly PrintJobResponse[], invoiceNo: string): ReceiptState | null {
  const job = [...jobs].reverse().find((j) => j.kind === "receipt" && j.label === `Receipt ${invoiceNo}`);
  if (!job) return null;
  const printer = job.printer || "the printer";
  switch (job.state) {
    case "done":
      return { text: `The receipt printed on ${printer}.`, failed: false };
    case "queued":
    case "printing":
      return { text: `The receipt is printing on ${printer}.`, failed: false };
    case "failed":
      return { text: `The receipt has not printed yet: ${printer} did not answer. It tries again by itself.`, failed: false };
    case "dead":
      return noPrinter(job)
        ? { text: "No receipt printed: this venue has no receipt printer. A manager adds one in the back office.", failed: true }
        : { text: "The receipt did not print. Retry it from the print queue.", failed: true };
  }
}

function nextTry(job: PrintJobResponse, now: Date): string {
  if (!job.next_attempt_at) return "trying again soon.";
  const seconds = Math.max(0, Math.ceil((Date.parse(job.next_attempt_at) - now.getTime()) / 1000));
  if (seconds === 0) return "trying again now.";
  if (seconds < 60) return `next try in ${seconds}s.`;
  return `next try in ${Math.ceil(seconds / 60)}m.`;
}

function sentence(s: string): string {
  const t = s.trim();
  const cap = t.charAt(0).toUpperCase() + t.slice(1);
  return /[.!?]$/.test(cap) ? cap : `${cap}.`;
}
