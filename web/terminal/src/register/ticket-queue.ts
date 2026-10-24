// The register's outbox. A tap on an item becomes a pending line at once and
// is sent in order, one command at a time, so the first item's order exists
// before the second item is added to it (terminal-register.md, Constraints).
//
// Every command carries an idempotency key, kept for its retries: a till on
// flaky wifi that retries a tap whose answer was lost must not ring the item
// up twice (docs/03-conventions.md, ADR-053). The order being opened has one
// key too, kept until an order exists.

import { failureOf } from "./order-messages";
import { tenderFailure } from "../tender/tender-logic";
import type { AddLine, AddNote, CommandResponse, LineModifier, OpenOrder, OrderResponse } from "../../../shared/api-types";

export interface PendingLine {
  readonly key: string;
  readonly command: AddLine;
  readonly status: "adding" | "failed";
  /** A kitchen note to write on the line once it exists, with the key it is sent under. */
  readonly note?: { readonly text: string; readonly key: string };
  /** Why the server refused it, or why it could not be asked. */
  readonly error?: string;
  /**
   * A failed line whose answer was lost rather than refused: the server may
   * have added it. Only Retry, which replays it under the same key, is safe;
   * removing it and ringing it up again would be a second line.
   */
  readonly uncertain?: boolean;
}

export interface TicketState {
  /** The current order's public id, once it exists. */
  readonly orderId: string | null;
  readonly pending: readonly PendingLine[];
}

export interface TicketCommands {
  openOrder(open: OpenOrder, key: string): Promise<CommandResponse>;
  addLine(orderId: string, command: AddLine, key: string): Promise<CommandResponse>;
  modifyLine(orderId: string, lineId: number, change: ModifyLine, key: string): Promise<CommandResponse>;
  addNote(orderId: string, note: AddNote, key: string): Promise<CommandResponse>;
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
  /**
   * A line went on the order and the note rung up with it did not. The line
   * stays: taking it back is the person's call, and so is writing it again.
   */
  onNoteLost?: (lineName: string, why: string) => void;
  /** Where the current order's id is kept across a reload. */
  remember: (orderId: string | null) => void;
  initialOrderId?: string | null;
  /** How this ticket opens when its first item is rung up: counter or table, covers. */
  open?: OpenOrder;
}

const counterSale: OpenOrder = { channel: "counter", covers: 0 };

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

  /**
   * Rings a line up, with a kitchen note if one was written. Returns its key,
   * which Retry and Remove take. The note has a key of its own from the start,
   * so a retry cannot write it twice.
   */
  ringUp(command: AddLine, note = ""): string {
    const key = this.opts.newKey();
    const text = note.trim();
    const line: PendingLine = { key, command, status: "adding", ...(text ? { note: { text, key: this.opts.newKey() } } : {}) };
    this.update({ pending: [...this.state.pending, line] });
    this.enqueue(key);
    return key;
  }

  /** Sends a refused line again, with the same key. */
  retry(key: string): void {
    const line = this.state.pending.find((p) => p.key === key);
    if (!line || line.status !== "failed") return;
    this.replace(key, { key, command: line.command, status: "adding", ...(line.note ? { note: line.note } : {}) });
    this.enqueue(key);
  }

  /** Forgets a refused line. One still being sent, or one the server may have, cannot be taken back here. */
  remove(key: string): void {
    const line = this.state.pending.find((p) => p.key === key);
    if (!line || line.status !== "failed" || line.uncertain) return;
    this.update({ pending: this.state.pending.filter((p) => p.key !== key) });
  }

  /** Changes a line's modifiers. See run. */
  modify(lineId: number, change: ModifyLine): Promise<void> {
    return this.run((orderId, key) => this.opts.commands.modifyLine(orderId, lineId, change, key));
  }

  /**
   * Sends one command to the current order, after anything already queued,
   * so a change made straight after a tap cannot overtake the tap. It gets
   * its own idempotency key, or `key` when the caller keeps one to retry
   * with (a payment). Resolves when the server has it; rejects with the
   * till's words when it does not. The chain carries on either way.
   */
  run(command: (orderId: string, key: string) => Promise<CommandResponse>, key = this.opts.newKey()): Promise<void> {
    const done = this.chain.then(async () => {
      const orderId = this.state.orderId;
      if (!orderId) throw new Error("There is no order to change.");
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

  /**
   * Opens the order now, not with the first line: a party seated at a
   * table is on the floor before anyone orders (terminal-floor.md). It
   * waits its turn behind anything queued, and an order already open is
   * simply answered. Rejects with the till's words.
   */
  openNow(): Promise<string> {
    const done = this.chain.then(() => this.ensureOrder());
    this.chain = done.then(
      () => undefined,
      () => undefined,
    );
    return done.catch((err: unknown) => {
      throw new Error(failureOf(err), { cause: err });
    });
  }

  /** How this ticket opens, or opened. */
  get opening(): OpenOrder {
    return this.opts.open ?? counterSale;
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
    let orderId: string;
    let res: CommandResponse;
    try {
      orderId = await this.ensureOrder();
      res = await this.opts.commands.addLine(orderId, line.command, key);
    } catch (err) {
      // A refusal is the server's word; a lost answer (no reach, a timeout,
      // a relay's 502) is not, and the line may be on the order already.
      const uncertain = tenderFailure(err) === "uncertain";
      this.replace(key, { ...line, status: "failed", error: failureOf(err), ...(uncertain ? { uncertain } : {}) });
      return;
    }
    this.opts.onOrder(res.order);
    this.update({ pending: this.state.pending.filter((p) => p.key !== key) });
    if (line.note) await this.sendNote(orderId, line.command.name, line.note, res);
  }

  /**
   * The note of a line just added. A line's id is the sequence of the event
   * that added it (docs/02-domain.md), which the answer carries. The line is
   * on the order whatever happens here, so a failure is said, not retried.
   */
  private async sendNote(orderId: string, name: string, note: NonNullable<PendingLine["note"]>, added: CommandResponse): Promise<void> {
    const lineId = added.events.find((e) => e.type === "line.added")?.seq;
    if (lineId === undefined) {
      this.opts.onNoteLost?.(name, "The server did not say which line it added.");
      return;
    }
    try {
      const res = await this.opts.commands.addNote(orderId, { scope: "line", line_id: lineId, text: note.text }, note.key);
      this.opts.onOrder(res.order);
    } catch (err) {
      this.opts.onNoteLost?.(name, failureOf(err));
    }
  }

  private async ensureOrder(): Promise<string> {
    if (this.state.orderId) return this.state.orderId;
    this.openKey ??= this.opts.newKey();
    const res = await this.opts.commands.openOrder(this.opening, this.openKey);
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
