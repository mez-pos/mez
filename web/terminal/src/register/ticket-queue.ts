// The register's outbox. A tap on an item becomes a pending line at once and
// is sent in order, one command at a time, so the first item's order exists
// before the second item is added to it (terminal-register.md, Constraints).
//
// Every command carries an idempotency key, kept for its retries: a till on
// flaky wifi that retries a tap whose answer was lost must not ring the item
// up twice (docs/03-conventions.md, ADR-053). The order being opened has one
// key too, kept until an order exists.

import { failureOf } from "./order-messages";
import type { AddLine, CommandResponse, LineModifier, OrderResponse } from "../../../shared/api-types";

export interface PendingLine {
  readonly key: string;
  readonly command: AddLine;
  readonly status: "adding" | "failed";
  /** Why the server refused it, or why it could not be asked. */
  readonly error?: string;
}

export interface TicketState {
  /** The current order's public id, once it exists. */
  readonly orderId: string | null;
  readonly pending: readonly PendingLine[];
}

export interface TicketCommands {
  openOrder(key: string): Promise<CommandResponse>;
  addLine(orderId: string, command: AddLine, key: string): Promise<CommandResponse>;
  modifyLine(orderId: string, lineId: number, change: ModifyLine, key: string): Promise<CommandResponse>;
}

/** A line's modifiers, from what the till showed to what was chosen. */
export interface ModifyLine {
  readonly from: readonly LineModifier[];
  readonly to: readonly LineModifier[];
}

export interface TicketQueueOptions {
  commands: TicketCommands;
  newKey: () => string;
  /** Every order the server answers with, for the cache. */
  onOrder: (order: OrderResponse) => void;
  /** Where the current order's id is kept across a reload. */
  remember: (orderId: string | null) => void;
  initialOrderId?: string | null;
}

export class TicketQueue {
  private state: TicketState;
  private readonly listeners = new Set<() => void>();
  private chain: Promise<void> = Promise.resolve();
  private openKey: string | null = null;

  constructor(private readonly opts: TicketQueueOptions) {
    this.state = { orderId: opts.initialOrderId ?? null, pending: [] };
  }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  current = (): TicketState => this.state;

  /** Rings a line up. Returns its key, which Retry and Remove take. */
  ringUp(command: AddLine): string {
    const key = this.opts.newKey();
    this.update({ pending: [...this.state.pending, { key, command, status: "adding" }] });
    this.enqueue(key);
    return key;
  }

  /** Sends a refused line again, with the same key. */
  retry(key: string): void {
    const line = this.state.pending.find((p) => p.key === key);
    if (!line || line.status !== "failed") return;
    this.replace(key, { key, command: line.command, status: "adding" });
    this.enqueue(key);
  }

  /** Forgets a refused line. One still being sent cannot be taken back here. */
  remove(key: string): void {
    const line = this.state.pending.find((p) => p.key === key);
    if (!line || line.status !== "failed") return;
    this.update({ pending: this.state.pending.filter((p) => p.key !== key) });
  }

  /** Changes a line's modifiers. See run. */
  modify(lineId: number, change: ModifyLine): Promise<void> {
    return this.run((orderId, key) => this.opts.commands.modifyLine(orderId, lineId, change, key));
  }

  /**
   * Sends one command to the current order, after anything already queued,
   * so a change made straight after a tap cannot overtake the tap. It gets
   * its own idempotency key. Resolves when the server has it; rejects with
   * the till's words when it does not. The chain carries on either way.
   */
  run(command: (orderId: string, key: string) => Promise<CommandResponse>): Promise<void> {
    const key = this.opts.newKey();
    const done = this.chain.then(async () => {
      const orderId = this.state.orderId;
      if (!orderId) throw new Error("There is no ticket to change.");
      try {
        const res = await command(orderId, key);
        this.opts.onOrder(res.order);
      } catch (err) {
        throw new Error(failureOf(err), { cause: err });
      }
    });
    this.chain = done.catch(() => undefined);
    return done;
  }

  /** Starts a new ticket. Lines still being sent keep going to the old one. */
  startNew(): void {
    this.openKey = null;
    this.update({ orderId: null, pending: this.state.pending.filter((p) => p.status === "adding") });
    this.opts.remember(null);
  }

  /** Resolves when everything queued so far has been sent or refused. */
  settled(): Promise<void> {
    return this.chain;
  }

  private enqueue(key: string): void {
    this.chain = this.chain.then(() => this.send(key));
  }

  private async send(key: string): Promise<void> {
    const line = this.state.pending.find((p) => p.key === key);
    if (!line) return;
    try {
      const orderId = await this.ensureOrder();
      const res = await this.opts.commands.addLine(orderId, line.command, key);
      this.opts.onOrder(res.order);
      this.update({ pending: this.state.pending.filter((p) => p.key !== key) });
    } catch (err) {
      this.replace(key, { key, command: line.command, status: "failed", error: failureOf(err) });
    }
  }

  private async ensureOrder(): Promise<string> {
    if (this.state.orderId) return this.state.orderId;
    this.openKey ??= this.opts.newKey();
    const res = await this.opts.commands.openOrder(this.openKey);
    this.openKey = null;
    this.opts.onOrder(res.order);
    this.opts.remember(res.order.id);
    this.update({ orderId: res.order.id });
    return res.order.id;
  }

  private replace(key: string, next: PendingLine): void {
    this.update({ pending: this.state.pending.map((p) => (p.key === key ? next : p)) });
  }

  private update(patch: Partial<TicketState>): void {
    this.state = { ...this.state, ...patch };
    for (const listener of this.listeners) listener();
  }
}
