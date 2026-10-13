// One AuthStore and one realtime connection for an app, wired together, and
// a watch on whether the server answering them is a mirror.

import { MirrorWatch } from "../mirror";
import { createRealtime } from "../use-realtime";
import { AuthStore } from "./auth-store";
import { browserStorage, credentialStore, type AppKind } from "./storage";

export function createAppAuth(app: AppKind, topics: readonly string[] = []) {
  // The back office shares the terminal's pairing: one pairing per browser (ADR-148).
  const mirror = new MirrorWatch();
  const store = new AuthStore(
    app,
    credentialStore(app, browserStorage("local"), browserStorage("session"), app === "backoffice" ? "terminal" : undefined),
    undefined,
    mirror.heard,
  );
  const realtime = createRealtime(store.credentials, topics, () => store.venueMissing());
  return { store, realtime, mirror };
}
