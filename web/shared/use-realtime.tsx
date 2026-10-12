// React binding for RealtimeConnection. One connection per app, created by
// the app and handed to RealtimeProvider; components read it with
// useRealtime() and listen with useRealtimeNotices().

import { createContext, useContext, useEffect, useLayoutEffect, useRef, useSyncExternalStore, type ReactNode } from "react";
import { ApiError, cameThroughRelay, createApiClient, type Credentials } from "./api";
import { useOnMirror } from "./use-mirror";
import { RealtimeConnection, streamPath, type ProbeResult, type RealtimeNotice, type RealtimeStatus } from "./realtime";

const RealtimeContext = createContext<RealtimeConnection | null>(null);

/** The stream URL for the page's own origin, ws: or wss: to match. */
export function streamURL(location: Pick<Location, "protocol" | "host"> = window.location): string {
  const scheme = location.protocol === "https:" ? "wss:" : "ws:";
  return `${scheme}//${location.host}${streamPath}`;
}

/** A connection for this page, probing `/devices/me` to explain failures. */
export function createRealtime(credentials: Credentials, topics: readonly string[] = []): RealtimeConnection {
  // Whether the probe's own answer came through the relay. Read off the
  // answer and not kept from an earlier one: the same page is never both.
  let relayed = false;
  const api = createApiClient({
    credentials,
    onResponse: (response) => {
      relayed = cameThroughRelay(response);
    },
  });
  const probe = async (): Promise<ProbeResult> => {
    relayed = false;
    try {
      await api.get("/api/v1/devices/me", { timeoutMs: 4_000 });
      return relayed ? "away" : "ok";
    } catch (err) {
      if (err instanceof ApiError && (err.code === "unauthenticated" || err.code === "device_revoked")) return err.code;
      return "unreachable";
    }
  };
  return new RealtimeConnection({ url: streamURL(), deviceToken: credentials.deviceToken, probe, topics });
}

interface RealtimeProviderProps {
  connection: RealtimeConnection;
  children: ReactNode;
}

export function RealtimeProvider({ connection, children }: RealtimeProviderProps) {
  useEffect(() => {
    connection.start();
    // The browser knowing before the backoff timer does: try at once.
    const retry = () => connection.reconnectNow();
    const visible = () => {
      if (document.visibilityState === "visible") connection.reconnectNow();
    };
    window.addEventListener("online", retry);
    document.addEventListener("visibilitychange", visible);
    return () => {
      window.removeEventListener("online", retry);
      document.removeEventListener("visibilitychange", visible);
      connection.stop();
    };
  }, [connection]);

  return <RealtimeContext.Provider value={connection}>{children}</RealtimeContext.Provider>;
}

function useConnection(): RealtimeConnection {
  const connection = useContext(RealtimeContext);
  if (!connection) throw new Error("useRealtime needs a RealtimeProvider above it");
  return connection;
}

export interface Realtime {
  status: RealtimeStatus;
  lastEventId: number;
  reconnectNow: () => void;
}

export function useRealtime(): Realtime {
  const connection = useConnection();
  const status = useSyncExternalStore(connection.subscribe, connection.current);
  return { status, lastEventId: connection.lastEventId(), reconnectNow: () => connection.reconnectNow() };
}

/**
 * Whether this page is being used away from the venue, through the relay.
 * False until the connection has found out, which takes one failed attempt
 * at the stream: a screen that hides something on this hides it a moment
 * late, and the server refuses it either way (ADR-308).
 *
 * A mirror is away too, and known to be from its first answer, before the
 * stream has been tried: what is left out from away is left out there at
 * once (ADR-330).
 */
export function useAway(): boolean {
  const connection = useConnection();
  const mirror = useOnMirror();
  const away = useSyncExternalStore(connection.subscribe, () => connection.current().state === "away");
  return away || mirror;
}

/** Hear every event and resync. The listener may change between renders. */
export function useRealtimeNotices(listener: (notice: RealtimeNotice) => void): void {
  const connection = useConnection();
  const latest = useRef(listener);
  useLayoutEffect(() => {
    latest.current = listener;
  });
  useEffect(() => connection.onNotice((notice) => latest.current(notice)), [connection]);
}
