import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { AddLine, CommandResponse, OrderResponse } from "../../../shared/api-types";
import { failureOf } from "./order-messages";
import { TicketQueue, type TicketCommands } from "./ticket-queue";

const line = (name: string): AddLine => ({
  item_id: 1,
  name,
  qty: 1,
  unit_price: { minor: 6000, currency: "INR" },
  modifiers: [],
  course: 1,
  station: "bar",
  tax_rate: 5000,
});

const answer = (id: string, lines = 0, events: CommandResponse["events"] = []): CommandResponse => ({
  order: { id, lines: Array.from({ length: lines }) } as unknown as OrderResponse,
  events,
});

/** The event an added line comes back with: its seq is the line's id. */
const added = (seq: number) => [{ seq, type: "line.added" }] as unknown as CommandResponse["events"];

/** A server in memory: records every call, fails when told to. */
function fakeServer() {
  const calls: string[] = [];
  let failNext: Error | null = null;
  let lines = 0;
  let opened = 0;
  let failLater: Error | null = null;
  let silent = false;
  const fail = () => {
    const e = failNext;
    // A failure set for the call after this one moves up as this one passes.
    failNext = failLater;
    failLater = null;
    if (e) throw e;
  };
  const commands: TicketCommands = {
    openOrder: async (open, key) => {
      calls.push(open.table_id ? `open table ${open.table_id} ${key}` : `open ${key}`);
      fail();
      opened += 1;
      return answer(`ord_${opened}`);
    },
    addLine: async (orderId, cmd, key) => {
      calls.push(`add ${orderId} ${cmd.name} ${key}`);
      fail();
      lines += 1;
      // Line ids are event sequences: 10, 20, 30 here, so none is a count.
      return answer(orderId, lines, silent ? [] : added(lines * 10));
    },
    modifyLine: async () => answer("unused"),
    addNote: async (orderId, note, key) => {
      calls.push(`note ${orderId} line ${note.line_id} "${note.text}" ${key}`);
      fail();
      return answer(orderId, lines);
    },
  };
  return { calls, commands, failWith: (e: Error) => (failNext = e), failAfter: (e: Error) => (failLater = e), answerWithoutEvents: () => (silent = true) };
}

function queue(server: ReturnType<typeof fakeServer>, initialOrderId: string | null = null) {
  let n = 0;
  const orders: OrderResponse[] = [];
  const remembered: (string | null)[] = [];
  const lost: string[] = [];
  const q = new TicketQueue({
    onNoteLost: (name, why) => lost.push(`${name}: ${why}`),
    commands: server.commands,
    newKey: () => `k${++n}`,
    onOrder: (o) => orders.push(o),
    remember: (id) => remembered.push(id),
    initialOrderId,
  });
  return { q, orders, remembered, lost };
}

const unreachable = () => new ApiError("network", "Failed to fetch");

describe("TicketQueue.openNow", () => {
  it("opens the order before any line, once, and remembers it", async () => {
    const server = fakeServer();
    const { q, remembered } = queue(server);
    await expect(q.openNow()).resolves.toBe("ord_1");
    await expect(q.openNow()).resolves.toBe("ord_1");
    q.ringUp(line("Tea"));
    await q.settled();
    expect(server.calls).toEqual(["open k1", "add ord_1 Tea k2"]);
    expect(remembered).toEqual(["ord_1"]);
  });

  it("rejects in the till's words, and a later tap opens it again", async () => {
    const server = fakeServer();
    const { q } = queue(server);
    server.failWith(unreachable());
    await expect(q.openNow()).rejects.toThrow(failureOf(unreachable()));
    await expect(q.openNow()).resolves.toBe("ord_1");
  });
});

describe("TicketQueue", () => {
  it("opens the order once, then adds each tap in order", async () => {
    const server = fakeServer();
    const { q, remembered } = queue(server);
    q.ringUp(line("Filter coffee"));
    q.ringUp(line("Upma"));
    expect(q.current().pending.map((p) => p.status)).toEqual(["adding", "adding"]);

    await q.settled();
    expect(server.calls).toEqual(["open k3", "add ord_1 Filter coffee k1", "add ord_1 Upma k2"]);
    expect(q.current()).toEqual({ orderId: "ord_1", pending: [] });
    expect(remembered).toEqual(["ord_1"]);
  });

  it("adds to the order it resumed", async () => {
    const server = fakeServer();
    const { q } = queue(server, "ord_9");
    q.ringUp(line("Latte"));
    await q.settled();
    expect(server.calls).toEqual(["add ord_9 Latte k1"]);
  });

  it("keeps a refused line, and a retry reuses its key", async () => {
    const server = fakeServer();
    const { q } = queue(server, "ord_1");
    server.failWith(unreachable());
    const key = q.ringUp(line("Latte"));
    await q.settled();
    expect(q.current().pending).toEqual([{ key, command: line("Latte"), status: "failed", error: "Not sent: the server can't be reached." }]);

    q.retry(key);
    expect(q.current().pending[0]?.status).toBe("adding");
    await q.settled();
    expect(server.calls).toEqual([`add ord_1 Latte ${key}`, `add ord_1 Latte ${key}`]);
    expect(q.current().pending).toEqual([]);
  });

  it("retries a failed open with the same key, so only one order is made", async () => {
    const server = fakeServer();
    const { q } = queue(server);
    server.failWith(unreachable());
    const key = q.ringUp(line("Latte"));
    await q.settled();
    expect(q.current().orderId).toBeNull();

    q.retry(key);
    await q.settled();
    expect(server.calls).toEqual(["open k2", "open k2", "add ord_1 Latte k1"]);
  });

  it("carries on with later taps after one is refused", async () => {
    const server = fakeServer();
    const { q } = queue(server, "ord_1");
    server.failWith(new ApiError("forbidden", "role cashier lacks order.open"));
    q.ringUp(line("Latte"));
    q.ringUp(line("Upma"));
    await q.settled();
    expect(q.current().pending.map((p) => [p.command.name, p.status, p.error])).toEqual([
      ["Latte", "failed", "You don't have permission to do that. Ask a manager."],
    ]);
  });

  it("removes a refused line, and never one still being sent", async () => {
    const server = fakeServer();
    const { q } = queue(server, "ord_1");
    const sending = q.ringUp(line("Latte"));
    q.remove(sending);
    expect(q.current().pending).toHaveLength(1);
    await q.settled();

    server.failWith(unreachable());
    const refused = q.ringUp(line("Upma"));
    await q.settled();
    q.remove(refused);
    expect(q.current().pending).toEqual([]);
  });

  it("hands every answer to the cache", async () => {
    const server = fakeServer();
    const { q, orders } = queue(server);
    q.ringUp(line("Latte"));
    await q.settled();
    expect(orders.map((o) => o.lines.length)).toEqual([0, 1]);
  });

  it("opens as the ticket was started: at a table, with covers", async () => {
    const server = fakeServer();
    const q = new TicketQueue({
      commands: server.commands,
      newKey: () => "k",
      onOrder: () => {},
      remember: () => {},
      open: { channel: "counter", table_id: 4, covers: 2 },
    });
    q.ringUp(line("Latte"));
    await q.settled();
    expect(server.calls[0]).toBe("open table 4 k");
    expect(q.opening).toEqual({ channel: "counter", table_id: 4, covers: 2 });
  });

  it("tells listeners about every change", async () => {
    const server = fakeServer();
    const { q } = queue(server, "ord_1");
    let heard = 0;
    const stop = q.subscribe(() => (heard += 1));
    q.ringUp(line("Latte"));
    await q.settled();
    stop();
    q.ringUp(line("Upma"));
    expect(heard).toBe(2); // added, then settled
  });
});

describe("failureOf", () => {
  it("words each kind of failure", () => {
    expect(failureOf(new ApiError("timeout", "x"))).toBe("Not sent: the server can't be reached.");
    expect(failureOf(new ApiError("order_closed", "db: order is closed"))).toBe("This order is closed.");
    expect(failureOf(new ApiError("modifiers_mismatch", "db: in transaction: projecting line.modified at seq 6"))).toBe(
      "This line was changed on another screen. Close this and look again.",
    );
    // A code with no words of its own still never shows the server's text.
    expect(failureOf(new ApiError("invalid_payload", "projecting line.added at seq 4: quantity 0"))).toBe(
      "The server refused this. Check the order and try again.",
    );
    expect(failureOf(new TypeError("boom"))).toBe("Not sent: something went wrong.");
  });
});

describe("TicketQueue.modify", () => {
  const oat = [{ modifier_id: 11, name: "Oat", price: { minor: 5000, currency: "INR" } }];

  function withModify() {
    const server = fakeServer();
    const calls: string[] = server.calls;
    let failNext: Error | null = null;
    server.commands.modifyLine = async (orderId, lineId, change, key) => {
      calls.push(`modify ${orderId} ${lineId} ${change.to.map((m) => m.name).join(",")} ${key}`);
      const e = failNext;
      failNext = null;
      if (e) throw e;
      return answer(orderId, 1);
    };
    return { server, failModify: (e: Error) => (failNext = e) };
  }

  it("waits for the tap before it, then sends the change", async () => {
    const { server } = withModify();
    const { q, orders } = queue(server);
    q.ringUp(line("Flat white"));
    const changed = q.modify(2, { from: [], to: oat });
    await changed;
    expect(server.calls).toEqual(["open k3", "add ord_1 Flat white k1", "modify ord_1 2 Oat k2"]);
    expect(orders).toHaveLength(3);
  });

  it("rejects with the rail's words, and the queue carries on", async () => {
    const { server, failModify } = withModify();
    const { q } = queue(server, "ord_1");
    failModify(new ApiError("line_in_kitchen", "db: line has gone to the kitchen: line 2"));
    await expect(q.modify(2, { from: [], to: oat })).rejects.toThrow("This has gone to the kitchen. Void it and add it again.");
    q.ringUp(line("Upma"));
    await q.settled();
    expect(q.current().pending).toEqual([]);
  });

  it("refuses when there is no ticket yet", async () => {
    const { server } = withModify();
    const { q } = queue(server);
    await expect(q.modify(2, { from: [], to: oat })).rejects.toThrow("There is no order to change.");
  });
});

describe("TicketQueue.run", () => {
  it("sends the caller's key when it is given one, and a new key when not", async () => {
    const server = fakeServer();
    const { q } = queue(server, "ord_9");
    const keys: string[] = [];
    const send = async (_id: string, key: string) => {
      keys.push(key);
      return answer("ord_9");
    };
    await q.run(send, "pay-1");
    await q.run(send, "pay-1");
    await q.run(send);
    expect(keys).toEqual(["pay-1", "pay-1", "k1"]);
  });
});

describe("TicketQueue, a line rung up with a kitchen note", () => {
  it("writes the note on the line the add made, after it, with a key of its own", async () => {
    const server = fakeServer();
    const { q, lost } = queue(server, "ord_1");
    q.ringUp(line("Upma"));
    q.ringUp(line("Pizza"), "cut into 8");
    q.ringUp(line("Tea"));
    await q.settled();
    expect(server.calls).toEqual(["add ord_1 Upma k1", "add ord_1 Pizza k2", 'note ord_1 line 20 "cut into 8" k3', "add ord_1 Tea k4"]);
    expect(q.current().pending).toEqual([]);
    expect(lost).toEqual([]);
  });

  it("writes no note for a line with none, or with only spaces", async () => {
    const server = fakeServer();
    const { q } = queue(server, "ord_1");
    q.ringUp(line("Upma"), "   ");
    await q.settled();
    expect(server.calls).toEqual(["add ord_1 Upma k1"]);
  });

  it("keeps the line and says the note was lost when the note is refused", async () => {
    const server = fakeServer();
    const { q, lost } = queue(server, "ord_1");
    server.failAfter(unreachable());
    q.ringUp(line("Pizza"), "no basil");
    await q.settled();
    // The line is on the order: it must not sit in pending to be added again.
    expect(q.current().pending).toEqual([]);
    expect(lost).toEqual([`Pizza: ${failureOf(unreachable())}`]);
  });

  it("says the note was lost when the answer does not name the line", async () => {
    const server = fakeServer();
    const { q, lost } = queue(server, "ord_1");
    server.answerWithoutEvents();
    q.ringUp(line("Pizza"), "no basil");
    await q.settled();
    expect(server.calls).toEqual(["add ord_1 Pizza k1"]);
    expect(lost).toHaveLength(1);
  });

  it("keeps the note with a refused line, and a retry sends both with the same keys", async () => {
    const server = fakeServer();
    const { q } = queue(server, "ord_1");
    server.failWith(unreachable());
    const key = q.ringUp(line("Pizza"), "no basil");
    await q.settled();
    expect(q.current().pending[0]).toMatchObject({ status: "failed", note: { text: "no basil" } });
    q.retry(key);
    await q.settled();
    expect(server.calls).toEqual(["add ord_1 Pizza k1", "add ord_1 Pizza k1", 'note ord_1 line 10 "no basil" k2']);
  });
});
