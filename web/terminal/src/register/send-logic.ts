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

const course = (l: LineResponse) => l.course ?? 0;

/** The lines still to go, apart: those Send takes as they are, and those held by course. */
function waitingLines(order: OrderResponse | undefined) {
  const lines = (order?.lines ?? []).filter(pending);
  const fired = (order?.lines ?? []).some((l) => l.course_fired);
  const ready = lines.filter((l) => course(l) === 0 || l.course_fired);
  const held = lines.filter((l) => course(l) > 0 && !l.course_fired);
  const lowest = held.reduce<number | null>((min, l) => (min === null || course(l) < min ? course(l) : min), null);
  return { fired, ready, held, lowest };
}

export function sendPlan(order: OrderResponse | undefined): SendPlan {
  const { fired, ready, held, lowest } = waitingLines(order);

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

/** "Send to Kitchen", or "Sent" when nothing is waiting. */
export function sendLabel(plan: SendPlan): string {
  return plan.waiting > 0 ? "Send to Kitchen" : "Sent";
}

/** What one Send would do, line by line, for the waiter to read before it goes (T04). */
export interface SendPreview {
  /** The lines that go, in the order's order. */
  readonly going: readonly LineResponse[];
  /** Lines the kitchen already has, which are not sent again. */
  readonly gone: number;
  /** Lines of later courses, which stay until their course is fired. */
  readonly held: number;
}

export function sendPreview(order: OrderResponse | undefined): SendPreview {
  const { fired, ready, held, lowest } = waitingLines(order);
  const first = !fired && lowest !== null ? held.filter((l) => course(l) === lowest) : [];
  const going = new Set([...ready, ...first].map((l) => l.id));
  return {
    going: (order?.lines ?? []).filter((l) => going.has(l.id)),
    gone: (order?.lines ?? []).filter((l) => l.sent && l.status !== "voided").length,
    held: held.length - first.length,
  };
}
