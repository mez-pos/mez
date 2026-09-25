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
      return job.last_error ? sentence(job.last_error) : "Gave up. Retry when the printer is ready.";
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
