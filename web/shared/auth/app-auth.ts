// One AuthStore and one realtime connection for an app, wired together.

import { createRealtime } from "../use-realtime";
import { AuthStore } from "./auth-store";
import { browserStorage, credentialStore, type AppKind } from "./storage";

export function createAppAuth(app: AppKind, topics: readonly string[] = []) {
  // The back office shares the terminal's pairing: one pairing per browser (ADR-148).
  const store = new AuthStore(app, credentialStore(app, browserStorage("local"), browserStorage("session"), app === "backoffice" ? "terminal" : undefined));
  const realtime = createRealtime(store.credentials, topics);
  return { store, realtime };
}
