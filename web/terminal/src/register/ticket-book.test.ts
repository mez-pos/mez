import { describe, expect, it } from "vitest";
import type { AddLine, CommandResponse, OrderResponse } from "../../../shared/api-types";
import { TicketBook } from "./ticket-book";
import { TicketQueue, type TicketCommands } from "./ticket-queue";

const line = (name: string): AddLine => ({
  item_id: 1, name, qty: 1, unit_price: { minor: 6000, currency: "INR" }, modifiers: [], course: 1, station: "bar", tax_rate: 5000,
});
const answer = (id: string): CommandResponse => ({ order: { id, lines: [] } as unknown as OrderResponse, events: [] });

/** A server whose opens can be held back, to switch tickets mid-send. */
function world(initialOrderId: string | null = null) {
  const calls: string[] = [];
  let opened = 0;
  let gate: Promise<void> = Promise.resolve();
  const commands: TicketCommands = {
    openOrder: async (open) => {
      await gate;
      opened += 1;
      calls.push(`open ord_${opened}${open.table_id ? ` table ${open.table_id}` : ""}`);
      return answer(`ord_${opened}`);
    },
    addLine: async (orderId, cmd) => {
      calls.push(`add ${orderId} ${cmd.name}`);
      return answer(orderId);
    },
    modifyLine: async (orderId) => answer(orderId),
  };
  const remembered: (string | null)[] = [];
  let n = 0;
  const book = new TicketBook({
    makeQueue: (t) => new TicketQueue({ commands, newKey: () => `k${++n}`, onOrder: () => {}, ...t }),
    remember: (id) => remembered.push(id),
    initialOrderId,
  });
  return {
    book, calls, remembered,
    holdOpens: () => {
      let release!: () => void;
      gate = new Promise((r) => (release = r));
      return release;
    },
  };
}

describe("TicketBook.seat", () => {
  it("opens the table's order at once, in front, and remembers it", async () => {
    const { book, calls, remembered } = world();
    const id = await book.seat({ channel: "counter", table_id: 4, covers: 3 });
    expect(id).toBe("ord_1");
    expect(calls).toEqual(["open ord_1 table 4"]);
    expect(book.current().current.current().orderId).toBe("ord_1");
    expect(book.current().current.opening).toEqual({ channel: "counter", table_id: 4, covers: 3 });
    expect(remembered).toEqual([null, "ord_1"]);
  });
});

describe("TicketBook", () => {
  it("starts on the remembered ticket", () => {
    const { book } = world("ord_7");
    expect(book.current().current.current().orderId).toBe("ord_7");
  });

  it("keeps each ticket's lines on its own order when switching mid-send", async () => {
    const w = world();
    const release = w.holdOpens();
    const first = w.book.current().current;
    first.ringUp(line("Filter coffee")); // waiting on its open

    w.book.startNew({ channel: "counter", table_id: 4, covers: 2 });
    const second = w.book.current().current;
    expect(second).not.toBe(first);
    second.ringUp(line("Upma"));

    release();
    await Promise.all([first.settled(), second.settled()]);
    expect(w.calls).toContain("add ord_1 Filter coffee");
    expect(w.calls.some((c) => c.startsWith("open") && c.endsWith("table 4"))).toBe(true);
    expect(w.calls.filter((c) => c.includes("Upma"))).toEqual([`add ${second.current().orderId} Upma`]);
    expect(first.current().orderId).not.toBe(second.current().orderId);
  });

  it("remembers only the ticket in front, even when another gets its order later", async () => {
    const w = world();
    const release = w.holdOpens();
    w.book.current().current.ringUp(line("Latte"));
    w.book.startNew({ channel: "counter", covers: 0 });
    release();
    await Promise.all(w.book.current().queues.map((q) => q.settled()));
    expect(w.remembered).toEqual([null]); // the background ticket's id did not take over
  });

  it("reuses a blank ticket instead of piling up empty ones", () => {
    const { book } = world();
    book.startNew({ channel: "counter", covers: 0 });
    book.startNew({ channel: "counter", covers: 0 });
    expect(book.current().queues).toHaveLength(1);
  });

  it("switches to an order, and back to the same queue", async () => {
    const w = world();
    const a = w.book.switchTo("ord_3");
    a.ringUp(line("Latte"));
    await a.settled();
    w.book.switchTo("ord_9");
    expect(w.book.switchTo("ord_3")).toBe(a);
    expect(w.calls).toEqual(["add ord_3 Latte"]);
    expect(w.remembered).toEqual(["ord_3", "ord_9", "ord_3"]);
  });

  it("redraws when any ticket changes", async () => {
    const w = world("ord_1");
    let heard = 0;
    w.book.subscribe(() => (heard += 1));
    const q = w.book.current().current;
    q.ringUp(line("Latte"));
    await q.settled();
    expect(heard).toBeGreaterThanOrEqual(2);
  });
});
