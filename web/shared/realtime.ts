// The realtime stream, without React. One RealtimeConnection per app, owned by
// RealtimeProvider (use-realtime.tsx). Protocol: internal/hub, ADR-060.
//
//   open    -> send hello { last_event_id }
//   welcome -> the server's position; replay follows, then caught_up
//   event   -> one event; its id is the stream position
//   menu_changed -> the venue's menu is at a new version; fetch it
//   print_changed -> the venue's print jobs moved; read the queue again
//   floor_changed -> the floor moved other than by an order; read it again
//
// What this adds on top of a WebSocket:
// - The last position seen, so a reconnect replays the gap and loses nothing.
// - Reconnect with jittered backoff, forever, unless the device is unpaired.
// - A heartbeat. The server pings at the protocol level, which a browser
//   answers but page code cannot see, so a socket that died silently (wifi
//   gone, cable pulled) would otherwise look open for minutes. Every
//   heartbeatMs a `subscribe` with unchanged topics is sent; the server
//   answers it with a `welcome`. Silence for longer than that is a dead link.
// - A reason when it is down. A browser reports a refused handshake and an
//   unplugged server identically, so after a failure one authenticated HTTP
//   request tells "server unreachable" apart from "this device was unpaired".
// - Knowing when there will never be a stream. The back office opened through
//   the relay, away from the venue, has none: the relay carries no WebSocket
//   (ADR-201). The same request that explains a failure says so, and the
//   connection then stops dialling and is "away" (ADR-309).

import { backoffDelay, defaultBackoff, type Backoff } from "./backoff";
import type { ClientMessage, EventMessage, FloorChangedMessage, MenuChangedMessage, PrintChangedMessage, WelcomeMessage } from "./api-types";

export const streamPath = "/api/v1/stream";
const subprotocol = "mez.v1"; // hub.Subprotocol
const tokenPrefix = "token."; // api.tokenProtocolPrefix

export type DownReason =
  | "checking" // lost; finding out why
  | "offline" // the browser has no network at all
  | "unreachable" // the server does not answer HTTP either
  | "stream_lost"; // the server answers, but the stream dropped

export type RealtimeStatus =
  | { readonly state: "idle" } // no device token, so nothing to connect as
  | { readonly state: "connecting"; readonly attempt: number }
  | { readonly state: "syncing"; readonly target: number } // replaying up to target
  | { readonly state: "live"; readonly since: number }
  | { readonly state: "reconnecting"; readonly attempt: number; readonly retryAt: number; readonly reason: DownReason }
  | { readonly state: "unpaired"; readonly code: "unauthenticated" | "device_revoked" }
  /** Reached through the relay: the server answers, and there is no stream to have. */
  | { readonly state: "away" };

/**
 * What screens hear. After a `resync`, anything held from before `from` may
 * be stale or missing (first connection, a restored backup): refetch it.
 * Events after that are deltas on top of the refetch.
 *
 * `menu` names the venue's current menu version. It is heard on every
 * connection, from the welcome, and whenever the menu changes; a cached menu
 * of any other version is stale.
 */
export type RealtimeNotice =
  | { readonly kind: "resync"; readonly from: number }
  | { readonly kind: "event"; readonly message: EventMessage }
  | { readonly kind: "menu"; readonly version: number }
  | { readonly kind: "print"; readonly version: number }
  | { readonly kind: "floor"; readonly version: number };

export type ProbeResult = "ok" | "unreachable" | "unauthenticated" | "device_revoked" | "away";

/** The part of a browser WebSocket this uses. Tests supply a fake. */
export interface SocketLike {
  onopen: ((ev: Event) => void) | null;
  onmessage: ((ev: MessageEvent) => void) | null;
  onclose: ((ev: CloseEvent) => void) | null;
  onerror: ((ev: Event) => void) | null;
  send(data: string): void;
  close(code?: number, reason?: string): void;
}

export interface RealtimeOptions {
  url: string;
  deviceToken: () => string | null;
  /** One authenticated HTTP request, run after a failure to learn why. */
  probe: () => Promise<ProbeResult>;
  topics?: readonly string[];
  createSocket?: (url: string, protocols: string[]) => SocketLike;
  isOnline?: () => boolean;
  backoff?: Backoff;
  heartbeatMs?: number;
  /** How long the stream must stay up for the count of failures to start again. Default 5 s. */
  settledAfterMs?: number;
  /** How long an open socket may take to say welcome before it is given up. */
  handshakeMs?: number;
  now?: () => number;
  random?: () => number;
}

type Listener = () => void;
type NoticeListener = (notice: RealtimeNotice) => void;

/** A listener's own failure, said where a developer looks and nowhere else. */
function reportListener(err: unknown): void {
  // eslint-disable-next-line no-console -- the one place a listener's throw is heard
  console.error("a realtime listener threw", err);
}

export class RealtimeConnection {
  private status: RealtimeStatus = { state: "idle" };
  private lastId = 0;
  private everSynced = false;
  private attempt = 0;
  private socket: SocketLike | null = null;
  private handshaken = false;
  private lastHeardAt = 0;
  private retryTimer: ReturnType<typeof setTimeout> | null = null;
  private handshakeTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  /** Runs once the stream has stayed up long enough to forget past failures. */
  private settleTimer: ReturnType<typeof setTimeout> | null = null;
  private running = false;
  private readonly listeners = new Set<Listener>();
  private readonly noticeListeners = new Set<NoticeListener>();
  private readonly opts: Required<Omit<RealtimeOptions, "topics">> & { topics: readonly string[] };

  constructor(options: RealtimeOptions) {
    this.opts = {
      topics: [],
      createSocket: (url, protocols) => new WebSocket(url, protocols),
      isOnline: () => typeof navigator === "undefined" || navigator.onLine,
      backoff: defaultBackoff,
      heartbeatMs: 15_000,
      settledAfterMs: 5_000,
      handshakeMs: 10_000,
      now: () => Date.now(),
      random: Math.random,
      ...options,
    };
  }

  current = (): RealtimeStatus => this.status;
  lastEventId = (): number => this.lastId;

  subscribe = (listener: Listener): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  onNotice(listener: NoticeListener): () => void {
    this.noticeListeners.add(listener);
    return () => this.noticeListeners.delete(listener);
  }

  start(): void {
    if (this.running) return;
    this.running = true;
    this.connect();
  }

  stop(): void {
    this.running = false;
    this.teardown();
    this.clearRetry();
    this.setStatus({ state: "idle" });
  }

  /** New credentials, or a staff member asking to try again after unpairing. */
  restart(): void {
    this.stop();
    this.attempt = 0;
    this.start();
  }

  /** Skip the wait: "retry now", the network coming back, the tab reappearing. */
  reconnectNow(): void {
    if (!this.running || this.status.state === "unpaired") return;
    if (this.socket) {
      // A socket that has heard nothing for a heartbeat may be half dead:
      // the wifi dropped and came back under it, and the browser has not
      // noticed. Waiting for the heartbeat to say so takes two intervals
      // more; the network coming back is the moment to ask again.
      if (this.handshaken && this.opts.now() - this.lastHeardAt > this.opts.heartbeatMs) {
        this.teardown();
        this.connect();
      }
      return;
    }
    // Away, there is nothing to reconnect. The moments that would have
    // retried are the moments someone has come back to the page, so what
    // the screens hold is read again in its place: nothing has told them of
    // a change since they last asked.
    if (this.status.state === "away") {
      this.emit({ kind: "resync", from: this.lastId });
      return;
    }
    this.clearRetry();
    this.connect();
  }

  private connect(): void {
    const token = this.opts.deviceToken();
    if (!token) {
      this.setStatus({ state: "idle" });
      return;
    }
    this.setStatus({ state: "connecting", attempt: this.attempt + 1 });

    let socket: SocketLike;
    try {
      socket = this.opts.createSocket(this.opts.url, [subprotocol, tokenPrefix + token]);
    } catch {
      this.lost();
      return;
    }
    this.socket = socket;
    this.handshaken = false;
    // The heartbeat only starts once welcomed. Until then this is the only
    // clock: a socket that opens but never answers hello (a proxy holding
    // the upgrade, a server stuck in startup) would otherwise sit at
    // "connecting" for good, with no retry.
    this.handshakeTimer = setTimeout(() => this.lost(), this.opts.handshakeMs);
    socket.onopen = () => this.send(this.hello());
    socket.onmessage = (ev) => this.receive(ev.data);
    socket.onclose = () => this.lost();
    socket.onerror = () => this.lost();
  }

  private hello(): ClientMessage {
    return {
      type: "hello",
      ...(this.lastId > 0 ? { last_event_id: this.lastId } : {}),
      ...(this.opts.topics.length > 0 ? { topics: [...this.opts.topics] } : {}),
    };
  }

  private receive(data: unknown): void {
    this.lastHeardAt = this.opts.now();
    const message = parse(data);
    if (!message) return;

    switch (message.type) {
      case "welcome":
        if (!this.handshaken) this.welcomed(message as WelcomeMessage);
        return; // a later welcome is a heartbeat answer
      case "event":
        this.advance((message as EventMessage).id);
        this.emit({ kind: "event", message: message as EventMessage });
        return;
      case "caught_up":
        this.advance(message.id ?? 0);
        return;
      case "menu_changed":
        this.menuAt((message as MenuChangedMessage).version);
        return;
      case "print_changed":
        this.printAt((message as PrintChangedMessage).version);
        return;
      case "floor_changed":
        this.floorAt((message as FloorChangedMessage).version);
        return;
      default:
        return; // "error" answers something this client sent; nothing to do but carry on
    }
  }

  private welcomed(welcome: WelcomeMessage): void {
    this.handshaken = true;
    this.clearHandshake();
    // The count of failures starts again only once the stream has stayed
    // up a while: a server that welcomes and then drops the socket each
    // time would otherwise be dialled every quarter second for as long as
    // it did.
    this.clearSettle();
    this.settleTimer = setTimeout(() => {
      this.settleTimer = null;
      this.attempt = 0;
    }, this.opts.settledAfterMs);
    const from = welcome.from_event_id;
    // The server starts where asked unless it cannot: a first connection, or
    // a position ahead of its stream (a restored backup). Either way, what
    // screens hold is not a base the next event can be applied to.
    const resync = !this.everSynced || from !== this.lastId;
    this.everSynced = true;
    this.lastId = from;
    this.startHeartbeat();
    if (resync) this.emit({ kind: "resync", from });
    // Every connection, not only a resync: the menu may have moved while
    // this device was away, with no event to show for it.
    this.menuAt(welcome.menu_version);
    // The print queue likewise: jobs may have printed or failed meanwhile.
    this.printAt(welcome.print_version);
    // And the floor: a table cleared, or the plan edited.
    this.floorAt(welcome.floor_version);
    this.setStatus(from >= welcome.latest_event_id ? this.liveStatus() : { state: "syncing", target: welcome.latest_event_id });
  }

  /** A version that is not a positive integer is not one to act on. */
  private menuAt(version: unknown): void {
    if (typeof version === "number" && Number.isInteger(version) && version > 0) this.emit({ kind: "menu", version });
  }

  /** Zero is a version: a venue that has queued nothing yet. */
  private printAt(version: unknown): void {
    if (typeof version === "number" && Number.isInteger(version) && version >= 0) this.emit({ kind: "print", version });
  }

  /** Zero is a version: a venue whose floor has never changed. */
  private floorAt(version: unknown): void {
    if (typeof version === "number" && Number.isInteger(version) && version >= 0) this.emit({ kind: "floor", version });
  }

  private advance(id: number): void {
    if (id > this.lastId) this.lastId = id;
    if (this.status.state === "syncing" && this.lastId >= this.status.target) {
      this.setStatus(this.liveStatus());
    }
  }

  private liveStatus(): RealtimeStatus {
    return { state: "live", since: this.opts.now() };
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.lastHeardAt = this.opts.now();
    const every = this.opts.heartbeatMs;
    this.heartbeatTimer = setInterval(() => {
      // Two intervals of silence: the last question went unanswered.
      if (this.opts.now() - this.lastHeardAt > 2 * every) {
        this.lost();
        return;
      }
      this.send({ type: "subscribe", ...(this.opts.topics.length > 0 ? { topics: [...this.opts.topics] } : {}) });
    }, every);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer !== null) clearInterval(this.heartbeatTimer);
    this.heartbeatTimer = null;
  }

  private send(message: ClientMessage): void {
    try {
      this.socket?.send(JSON.stringify(message));
    } catch {
      this.lost();
    }
  }

  // Close, error and a silent heartbeat all land here, often more than once
  // for one failure; only the first does anything.
  private lost(): void {
    if (!this.socket && this.status.state !== "connecting") return;
    this.teardown();
    if (!this.running) return;

    this.attempt += 1;
    this.scheduleRetry(this.opts.isOnline() ? "checking" : "offline");
    if (this.opts.isOnline()) void this.diagnose();
  }

  private scheduleRetry(reason: DownReason): void {
    this.clearRetry();
    const delay = backoffDelay(this.attempt, this.opts.backoff, this.opts.random);
    this.retryTimer = setTimeout(() => {
      this.retryTimer = null;
      this.connect();
    }, delay);
    this.setStatus({ state: "reconnecting", attempt: this.attempt, retryAt: this.opts.now() + delay, reason });
  }

  private async diagnose(): Promise<void> {
    const attempt = this.attempt;
    const result = await this.opts.probe().catch((): ProbeResult => "unreachable");
    // A newer failure, a success or a stop has happened since: this answer is stale.
    if (this.status.state !== "reconnecting" || this.attempt !== attempt) return;

    if (result === "unauthenticated" || result === "device_revoked") {
      this.clearRetry();
      this.setStatus({ state: "unpaired", code: result });
      return;
    }
    if (result === "away") {
      // No retry: the next attempt would fail the same way, and the one
      // after, each a request through the relay that the venue pays for.
      this.clearRetry();
      this.setStatus({ state: "away" });
      return;
    }
    this.setStatus({ ...this.status, reason: result === "ok" ? "stream_lost" : "unreachable" });
  }

  private clearSettle(): void {
    if (this.settleTimer !== null) clearTimeout(this.settleTimer);
    this.settleTimer = null;
  }

  private teardown(): void {
    this.stopHeartbeat();
    this.clearHandshake();
    this.clearSettle();
    const socket = this.socket;
    this.socket = null;
    this.handshaken = false;
    if (!socket) return;
    socket.onopen = socket.onmessage = socket.onclose = socket.onerror = null;
    try {
      socket.close(1000, "reconnecting");
    } catch {
      // already closed
    }
  }

  private clearHandshake(): void {
    if (this.handshakeTimer !== null) clearTimeout(this.handshakeTimer);
    this.handshakeTimer = null;
  }

  private clearRetry(): void {
    if (this.retryTimer !== null) clearTimeout(this.retryTimer);
    this.retryTimer = null;
  }

  private setStatus(next: RealtimeStatus): void {
    this.status = next;
    for (const listener of this.listeners) {
      try {
        listener();
      } catch (err) {
        reportListener(err);
      }
    }
  }

  // One listener that throws must not keep the rest from hearing, nor the
  // welcome from finishing: the status would stay "connecting" over a stream
  // that is up.
  private emit(notice: RealtimeNotice): void {
    for (const listener of this.noticeListeners) {
      try {
        listener(notice);
      } catch (err) {
        reportListener(err);
      }
    }
  }
}

interface WireMessage {
  type: string;
  id?: number;
}

function parse(data: unknown): WireMessage | null {
  if (typeof data !== "string") return null;
  try {
    const value: unknown = JSON.parse(data);
    if (typeof value !== "object" || value === null || !("type" in value) || typeof value.type !== "string") return null;
    return value as WireMessage;
  } catch {
    return null;
  }
}
