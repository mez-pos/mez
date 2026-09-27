// One AuthStore and one realtime connection for an app, wired together.

import { createRealtime } from "../use-realtime";
import { AuthStore } from "./auth-store";
import { browserStorage, credentialStore, type AppKind } from "./storage";

export function createAppAuth(app: AppKind, topics: readonly string[] = []) {
  const store = new AuthStore(app, credentialStore(app, browserStorage("local"), browserStorage("session")));
  const realtime = createRealtime(store.credentials, topics);
  return { store, realtime };
}
