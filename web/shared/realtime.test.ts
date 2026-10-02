import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RealtimeConnection, type ProbeResult, type RealtimeNotice, type RealtimeOptions, type SocketLike } from "./realtime";

class FakeSocket implements SocketLike {
  onopen: ((ev: Event) => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  onclose: ((ev: CloseEvent) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;
  readonly sent: Record<string, unknown>[] = [];
  closed = false;

  constructor(
    readonly url: string,
    readonly protocols: string[],
  ) {}

  send(data: string): void {
    this.sent.push(JSON.parse(data) as Record<string, unknown>);
  }
  close(): void {
    this.closed = true;
  }

  // The server's side.
  open(): void {
    this.onopen?.(new Event("open"));
  }
  receive(message: unknown): void {
    this.onmessage?.(new MessageEvent("message", { data: typeof message === "string" ? message : JSON.stringify(message) }));
  }
  drop(): void {
    // Node has no CloseEvent; the connection only cares that one arrived.
    this.onclose?.(new Event("close") as CloseEvent);
  }
}

const welcome = (from: number, latest: number) => ({
  type: "welcome", venue_id: 1, device_id: 2, from_event_id: from, latest_event_id: latest, topics: ["venue"],
});
const event = (id: number) => ({ type: "event", id, order_id: `o${id}`, order_status: "open", event: { type: "order.opened" } });

interface Harness {
  conn: RealtimeConnection;
  sockets: FakeSocket[];
  notices: RealtimeNotice[];
  probes: { count: number };
  latest(): FakeSocket;
  setOnline(online: boolean): void;
  setProbe(result: ProbeResult | Error): void;
}

function harness(overrides: Partial<RealtimeOptions> = {}): Harness {
  const sockets: FakeSocket[] = [];
  const notices: RealtimeNotice[] = [];
  const probes = { count: 0 };
  let online = true;
  let probeResult: ProbeResult | Error = "ok";

  const conn = new RealtimeConnection({
    url: "ws://till/api/v1/stream",
    deviceToken: () => "nfd_abc",
    probe: async () => {
      probes.count += 1;
      if (probeResult instanceof Error) throw probeResult;
      return probeResult;
    },
    createSocket: (url, protocols) => {
      const s = new FakeSocket(url, protocols);
      sockets.push(s);
      return s;
    },
    isOnline: () => online,
    backoff: { baseMs: 1000, capMs: 8000 },
    heartbeatMs: 1000,
    random: () => 0,
    ...overrides,
  });
  conn.onNotice((n) => notices.push(n));

  return {
    conn, sockets, notices, probes,
    latest: () => {
      const s = sockets.at(-1);
      if (!s) throw new Error("no socket was opened");
      return s;
    },
    setOnline: (value) => (online = value),
    setProbe: (value) => (probeResult = value),
  };
}

// Connect, handshake at `from`, and return the live socket.
function connectAt(h: Harness, from: number, latest = from): FakeSocket {
  h.conn.start();
  const s = h.latest();
  s.open();
  s.receive(welcome(from, latest));
  return s;
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(1_000_000);
});
afterEach(() => {
  vi.useRealTimers();
});

describe("RealtimeConnection: connecting", () => {
  it("stays idle with no device token", () => {
    const h = harness({ deviceToken: () => null });
    h.conn.start();
    expect(h.conn.current()).toEqual({ state: "idle" });
    expect(h.sockets).toHaveLength(0);
  });

  it("offers the subprotocol and the token, then says hello with no position", () => {
    const h = harness({ topics: ["station:bar"] });
    h.conn.start();
    expect(h.conn.current()).toEqual({ state: "connecting", attempt: 1 });

    const s = h.latest();
    expect(s.url).toBe("ws://till/api/v1/stream");
    expect(s.protocols).toEqual(["mez.v1", "token.nfd_abc"]);
    s.open();
    expect(s.sent).toEqual([{ type: "hello", topics: ["station:bar"] }]);
  });

  it("is live at once when there is nothing to replay, and tells screens to refetch", () => {
    const h = harness();
    connectAt(h, 40, 40);
    expect(h.conn.current()).toEqual({ state: "live", since: 1_000_000 });
    expect(h.notices).toEqual([{ kind: "resync", from: 40 }]);
    expect(h.conn.lastEventId()).toBe(40);
  });

  it("syncs through a replay, then goes live at the target", () => {
    const h = harness();
    const s = connectAt(h, 10, 13);
    expect(h.conn.current()).toEqual({ state: "syncing", target: 13 });

    s.receive(event(11));
    s.receive(event(12));
    expect(h.conn.current().state).toBe("syncing");
    // Event 13 was for a topic this screen does not follow; caught_up still moves it.
    s.receive({ type: "caught_up", id: 13 });

    expect(h.conn.current().state).toBe("live");
    expect(h.conn.lastEventId()).toBe(13);
    expect(h.notices.filter((n) => n.kind === "event")).toHaveLength(2);
  });

  it("ignores messages it cannot read", () => {
    const h = harness();
    const s = connectAt(h, 5);
    s.receive("not json");
    s.receive({ no: "type" });
    s.receive({ type: "error", code: "unknown_message", message: "x" });
    expect(h.conn.current().state).toBe("live");
    expect(h.conn.lastEventId()).toBe(5);
  });
});

describe("RealtimeConnection: losing and regaining the server", () => {
  it("resumes from the last position, and does not resync when nothing was lost", () => {
    const h = harness();
    const first = connectAt(h, 5);
    first.receive(event(6));
    first.receive({ type: "caught_up", id: 9 });
    first.drop();

    vi.advanceTimersByTime(500); // attempt 1: equal jitter at random 0 is half of 1000
    const second = h.latest();
    expect(second).not.toBe(first);
    second.open();
    expect(second.sent[0]).toEqual({ type: "hello", last_event_id: 9 });

    second.receive(welcome(9, 9));
    expect(h.notices.filter((n) => n.kind === "resync")).toHaveLength(1); // only the first connection's
    expect(h.conn.current().state).toBe("live");
  });

  it("resyncs when the server cannot resume where asked", () => {
    const h = harness();
    connectAt(h, 50).drop();
    vi.advanceTimersByTime(500);
    const s = h.latest();
    s.open();
    s.receive(welcome(0, 12)); // a restored backup: the server is behind the client

    expect(h.notices.at(-1)).toEqual({ kind: "resync", from: 0 });
    expect(h.conn.lastEventId()).toBe(0);
    expect(h.conn.current()).toEqual({ state: "syncing", target: 12 });
  });

  it("backs off between attempts and counts them, then resets on success", async () => {
    const h = harness();
    connectAt(h, 1).drop();
    expect(h.conn.current()).toEqual({ state: "reconnecting", attempt: 1, retryAt: 1_000_500, reason: "checking" });

    vi.advanceTimersByTime(500);
    expect(h.conn.current()).toEqual({ state: "connecting", attempt: 2 });
    h.latest().drop();
    expect(h.conn.current()).toMatchObject({ state: "reconnecting", attempt: 2, retryAt: Date.now() + 1000 });

    vi.advanceTimersByTime(1000);
    const s = h.latest();
    s.open();
    s.receive(welcome(1, 1));
    s.drop();
    expect(h.conn.current()).toMatchObject({ state: "reconnecting", attempt: 1 });
  });

  it("treats a socket that cannot even be created as a failed attempt", () => {
    const h = harness({
      createSocket: () => {
        throw new Error("SecurityError");
      },
    });
    h.conn.start();
    expect(h.conn.current()).toMatchObject({ state: "reconnecting", attempt: 1 });
  });

  it("says the stream dropped when the server still answers HTTP", async () => {
    const h = harness();
    connectAt(h, 1).drop();
    await vi.advanceTimersByTimeAsync(0);
    expect(h.conn.current()).toMatchObject({ state: "reconnecting", reason: "stream_lost" });
  });

  it("says the server is unreachable when HTTP fails too", async () => {
    const h = harness();
    h.setProbe(new TypeError("Failed to fetch"));
    connectAt(h, 1).drop();
    await vi.advanceTimersByTimeAsync(0);
    expect(h.conn.current()).toMatchObject({ state: "reconnecting", reason: "unreachable" });
  });

  it("says no network without probing when the browser is offline", async () => {
    const h = harness();
    h.setOnline(false);
    connectAt(h, 1).drop();
    await vi.advanceTimersByTimeAsync(0);
    expect(h.conn.current()).toMatchObject({ state: "reconnecting", reason: "offline" });
    expect(h.probes.count).toBe(0);
  });

  const unpaired: ProbeResult[] = ["device_revoked", "unauthenticated"];
  for (const code of unpaired) {
    it(`stops retrying when the probe says ${code}`, async () => {
      const h = harness();
      h.setProbe(code);
      connectAt(h, 1).drop();
      await vi.advanceTimersByTimeAsync(0);
      expect(h.conn.current()).toEqual({ state: "unpaired", code });

      const opened = h.sockets.length;
      await vi.advanceTimersByTimeAsync(60_000);
      h.conn.reconnectNow();
      expect(h.sockets).toHaveLength(opened);
    });
  }

  it("ignores a probe answer that arrives after the next attempt started", async () => {
    let answer: (r: ProbeResult) => void = () => undefined;
    const slow = harness({ probe: () => new Promise<ProbeResult>((resolve) => (answer = resolve)) });
    connectAt(slow, 1).drop();
    vi.advanceTimersByTime(500); // retry fires, attempt 2 is connecting
    answer("device_revoked");
    await vi.advanceTimersByTimeAsync(0);
    expect(slow.conn.current()).toEqual({ state: "connecting", attempt: 2 });
  });

  it("retries at once when asked", () => {
    const h = harness();
    connectAt(h, 1).drop();
    const opened = h.sockets.length;
    h.conn.reconnectNow();
    expect(h.sockets).toHaveLength(opened + 1);
    expect(h.conn.current().state).toBe("connecting");
  });

  it("does nothing on reconnectNow while connected", () => {
    const h = harness();
    connectAt(h, 1);
    h.conn.reconnectNow();
    expect(h.sockets).toHaveLength(1);
  });

  it("stops for good on stop", () => {
    const h = harness();
    const s = connectAt(h, 1);
    h.conn.stop();
    expect(s.closed).toBe(true);
    expect(h.conn.current()).toEqual({ state: "idle" });
    vi.advanceTimersByTime(60_000);
    expect(h.sockets).toHaveLength(1);
  });

  it("starts over on restart, as after pairing again", async () => {
    const h = harness();
    h.setProbe("device_revoked");
    connectAt(h, 1).drop();
    await vi.advanceTimersByTimeAsync(0);
    h.setProbe("ok");

    h.conn.restart();
    expect(h.conn.current()).toEqual({ state: "connecting", attempt: 1 });
  });
});

describe("RealtimeConnection: heartbeat", () => {
  it("asks every interval, and a welcome in reply is not a new session", () => {
    const h = harness();
    const s = connectAt(h, 3);
    vi.advanceTimersByTime(1000);
    expect(s.sent.at(-1)).toEqual({ type: "subscribe" });

    s.receive(welcome(3, 3));
    vi.advanceTimersByTime(1000);
    expect(h.notices.filter((n) => n.kind === "resync")).toHaveLength(1);
    expect(h.conn.current().state).toBe("live");
  });

  it("declares a silent link dead without waiting for the browser to notice", () => {
    const h = harness();
    const s = connectAt(h, 3);
    vi.advanceTimersByTime(3000); // three intervals, no answer
    expect(s.closed).toBe(true);
    expect(h.conn.current().state).toBe("reconnecting");
  });

  it("keeps a link alive while events flow", () => {
    const h = harness();
    const s = connectAt(h, 3);
    for (let id = 4; id < 10; id++) {
      vi.advanceTimersByTime(1000);
      s.receive(event(id));
    }
    expect(s.closed).toBe(false);
    expect(h.conn.current().state).toBe("live");
  });
});

describe("RealtimeConnection: the menu version", () => {
  it("names the menu on every connection, and whenever it changes", () => {
    const h = harness();
    h.conn.start();
    const first = h.latest();
    first.open();
    first.receive({ ...welcome(5, 5), menu_version: 3 });
    first.receive({ type: "menu_changed", version: 4 });
    first.drop();

    vi.advanceTimersByTime(500);
    const second = h.latest();
    second.open();
    second.receive({ ...welcome(5, 5), menu_version: 6 }); // changed twice while away

    expect(h.notices.filter((n) => n.kind === "menu")).toEqual([
      { kind: "menu", version: 3 },
      { kind: "menu", version: 4 },
      { kind: "menu", version: 6 },
    ]);
  });

  it("does not repeat it on a heartbeat's welcome", () => {
    const h = harness();
    h.conn.start();
    const s = h.latest();
    s.open();
    s.receive({ ...welcome(1, 1), menu_version: 2 });
    s.receive({ ...welcome(1, 1), menu_version: 2 });
    expect(h.notices.filter((n) => n.kind === "menu")).toHaveLength(1);
  });

  it("ignores a version it cannot use", () => {
    const h = harness();
    const s = connectAt(h, 1);
    for (const version of [undefined, null, "7", 0, -1, 1.5]) {
      s.receive({ type: "menu_changed", version });
    }
    expect(h.notices.filter((n) => n.kind === "menu")).toEqual([]);
  });
});

describe("RealtimeConnection: the print queue version", () => {
  it("names it on every connection, zero included, and whenever it moves", () => {
    const h = harness();
    h.conn.start();
    const s = h.latest();
    s.open();
    s.receive({ ...welcome(5, 5), print_version: 0 });
    s.receive({ type: "print_changed", version: 1 });
    s.receive({ type: "print_changed", version: 2 });
    expect(h.notices.filter((n) => n.kind === "print")).toEqual([
      { kind: "print", version: 0 },
      { kind: "print", version: 1 },
      { kind: "print", version: 2 },
    ]);
  });

  it("ignores a version it cannot use", () => {
    const h = harness();
    const s = connectAt(h, 1);
    const before = h.notices.filter((n) => n.kind === "print").length;
    for (const version of [undefined, null, "7", -1, 1.5]) {
      s.receive({ type: "print_changed", version });
    }
    expect(h.notices.filter((n) => n.kind === "print")).toHaveLength(before);
  });
});

describe("RealtimeConnection: the floor version", () => {
  it("names it on every connection, zero included, and whenever it moves", () => {
    const h = harness();
    h.conn.start();
    const s = h.latest();
    s.open();
    s.receive({ ...welcome(5, 5), floor_version: 0 });
    s.receive({ type: "floor_changed", version: 3 });
    expect(h.notices.filter((n) => n.kind === "floor")).toEqual([
      { kind: "floor", version: 0 },
      { kind: "floor", version: 3 },
    ]);
  });

  it("ignores a version it cannot use", () => {
    const h = harness();
    const s = connectAt(h, 1);
    const before = h.notices.filter((n) => n.kind === "floor").length;
    for (const version of [undefined, null, "7", -1, 1.5]) {
      s.receive({ type: "floor_changed", version });
    }
    expect(h.notices.filter((n) => n.kind === "floor")).toHaveLength(before);
  });
});
