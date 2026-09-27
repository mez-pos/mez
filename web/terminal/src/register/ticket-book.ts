// The register's open tickets on this till (Task 5): one TicketQueue per
// ticket, and which one is in front. Switching tickets only changes which
// queue is shown; a queue still sending keeps sending to its own order, so a
// switch mid-tap loses nothing and misfiles nothing (ADR-074).

import type { OpenOrder } from "../../../shared/api-types";
import { TicketQueue, type TicketQueueOptions } from "./ticket-queue";

/** Builds a queue for a ticket; the book supplies `remember` and the ticket. */
export type MakeQueue = (opts: Pick<TicketQueueOptions, "initialOrderId" | "open" | "remember">) => TicketQueue;

export interface TicketBookOptions {
  makeQueue: MakeQueue;
  /** Keeps the current ticket's order id across a reload. */
  remember: (orderId: string | null) => void;
  initialOrderId?: string | null;
}

export interface BookState {
  /** The ticket in front. */
  readonly current: TicketQueue;
  /** Every ticket this till has touched, the current one included. */
  readonly queues: readonly TicketQueue[];
}

export class TicketBook {
  private state: BookState;
  private readonly listeners = new Set<() => void>();

  constructor(private readonly opts: TicketBookOptions) {
    const first = this.make({ initialOrderId: opts.initialOrderId ?? null });
    this.state = { current: first, queues: [first] };
  }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  current = (): BookState => this.state;

  /**
   * Starts a new ticket in front. A current ticket with no order and
   * nothing in flight is reused rather than left behind, empty.
   */
  startNew(open: OpenOrder): void {
    const keep = this.state.queues.filter((q) => !isBlank(q));
    const fresh = this.make({ initialOrderId: null, open });
    this.set({ current: fresh, queues: [...keep, fresh] });
    this.opts.remember(null);
  }

  /**
   * Seats a party: a new ticket in front whose order opens now, at the
   * table, so the floor shows it seated before the first item. Resolves
   * with the order's id.
   */
  seat(open: OpenOrder): Promise<string> {
    this.startNew(open);
    return this.state.current.openNow();
  }

  /** Brings an existing order to the front, with its own queue. */
  switchTo(orderId: string): TicketQueue {
    const found = this.state.queues.find((q) => q.current().orderId === orderId);
    const keep = this.state.queues.filter((q) => q === found || !isBlank(q));
    const queue = found ?? this.make({ initialOrderId: orderId });
    this.set({ current: queue, queues: found ? keep : [...keep, queue] });
    this.opts.remember(orderId);
    return queue;
  }

  private make(ticket: Pick<TicketQueueOptions, "initialOrderId" | "open">): TicketQueue {
    const queue: TicketQueue = this.opts.makeQueue({
      ...ticket,
      // Only the ticket in front is remembered: a ticket in the background
      // that gets its order id must not take over the reload.
      remember: (id) => {
        if (this.state?.current === queue) this.opts.remember(id);
      },
    });
    // A change in any ticket redraws the book: its pending lines, its order.
    queue.subscribe(() => this.set({ ...this.state }));
    return queue;
  }

  private set(next: BookState): void {
    this.state = next;
    for (const listener of this.listeners) listener();
  }
}

/** A ticket with no order and nothing on the way: nothing to keep. */
function isBlank(queue: TicketQueue): boolean {
  const s = queue.current();
  return s.orderId === null && s.pending.length === 0;
}
