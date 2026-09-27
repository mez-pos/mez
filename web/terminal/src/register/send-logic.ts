// What Send and Fire would do for the ticket in front (ADR-089), so the
// buttons can say it before they are tapped. The server decides; this is
// order.ToSend and order.Fire read the same way from the lines.

import type { LineResponse, OrderResponse } from "../../../shared/api-types";

/** A line still to go: not sent, not voided, with a station to go to. */
function pending(l: LineResponse): boolean {
  return !l.sent && l.status !== "voided" && (l.station ?? "") !== "";
}

export interface SendPlan {
  /** Lines one tap of Send would send. */
  readonly waiting: number;
  /** The course Send would fire with them, while nothing has been fired. */
  readonly firing: number | null;
  /** The next course held back, once one has gone: what Fire would send. */
  readonly next: { readonly course: number; readonly lines: number } | null;
}

export function sendPlan(order: OrderResponse | undefined): SendPlan {
  const lines = (order?.lines ?? []).filter(pending);
  const fired = (order?.lines ?? []).some((l) => l.course_fired);
  const course = (l: LineResponse) => l.course ?? 0;
  const ready = lines.filter((l) => course(l) === 0 || l.course_fired);
  const held = lines.filter((l) => course(l) > 0 && !l.course_fired);
  const lowest = held.reduce<number | null>((min, l) => (min === null || course(l) < min ? course(l) : min), null);

  if (!fired && lowest !== null) {
    const first = held.filter((l) => course(l) === lowest).length;
    return { waiting: ready.length + first, firing: lowest, next: null };
  }
  const next = lowest === null ? null : { course: lowest, lines: held.filter((l) => course(l) === lowest).length };
  return { waiting: ready.length, firing: null, next };
}

/** The lines whose ticket the kitchen has bumped: on the pass (ADR-090). */
export function readyLines(order: OrderResponse | undefined): ReadonlySet<number> {
  const ready = new Set<number>();
  for (const t of order?.tickets ?? []) {
    if (t.status === "ready") for (const id of t.line_ids) ready.add(id);
  }
  return ready;
}

/** "Send 3", "Send", or "Sent" when nothing is waiting. */
export function sendLabel(plan: SendPlan): string {
  return plan.waiting > 0 ? `Send ${plan.waiting}` : "Sent";
}
