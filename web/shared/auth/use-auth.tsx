// React binding for AuthStore, and the wiring between auth and the stream.

import { createContext, useContext, useEffect, useSyncExternalStore, type ReactNode } from "react";
import type { RealtimeConnection } from "../realtime";
import type { AuthStatus, AuthStore } from "./auth-store";

const AuthContext = createContext<AuthStore | null>(null);

interface AuthProviderProps {
  store: AuthStore;
  realtime: RealtimeConnection;
  children: ReactNode;
}

export function AuthProvider({ store, realtime, children }: AuthProviderProps) {
  useEffect(() => {
    void store.boot();
  }, [store]);

  // A new device token (paired, or forgotten) means a new stream identity.
  useEffect(() => {
    let token = store.credentials.deviceToken();
    return store.subscribe(() => {
      const next = store.credentials.deviceToken();
      if (next !== token) {
        token = next;
        realtime.restart();
      }
    });
  }, [store, realtime]);

  // The stream can learn first that this device was unpaired: a till left
  // idle overnight makes no requests, but its socket is refused at once.
  useEffect(
    () =>
      realtime.subscribe(() => {
        const s = realtime.current();
        if (s.state === "unpaired") store.credentialFailed(s.code);
      }),
    [store, realtime],
  );

  return <AuthContext.Provider value={store}>{children}</AuthContext.Provider>;
}

export function useAuthStore(): AuthStore {
  const store = useContext(AuthContext);
  if (!store) throw new Error("useAuth needs an AuthProvider above it");
  return store;
}

export function useAuth(): AuthStatus {
  const store = useAuthStore();
  return useSyncExternalStore(store.subscribe, store.current);
}
