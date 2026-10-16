// One AuthStore and one realtime connection for an app, wired together, and
// a watch on whether the server answering them is a mirror.

import { MirrorWatch } from "../mirror";
import { createRealtime } from "../use-realtime";
import { AuthStore } from "./auth-store";
import { readHandoff } from "./cloud-handoff";
import { browserStorage, credentialStore, type AppKind, type CredentialStore } from "./storage";

export function createAppAuth(app: AppKind, topics: readonly string[] = []) {
  // The back office shares the terminal's pairing: one pairing per browser (ADR-148).
  const mirror = new MirrorWatch();
  const credentials = credentialStore(app, browserStorage("local"), browserStorage("session"), app === "backoffice" ? "terminal" : undefined);
  // Only the back office is opened from mez Cloud (ADR-355).
  if (app === "backoffice") takeHandoff(window.location, window.history, credentials);
  const store = new AuthStore(app, credentials, undefined, mirror.heard);
  const realtime = createRealtime(store.credentials, topics, () => store.venueMissing());
  return { store, realtime, mirror };
}

/**
 * Keeps a session mez Cloud handed in the address's fragment, and takes it
 * out of the address at once, so that it is not in the history, a bookmark
 * or a screenshot of the bar. Reports whether there was one.
 */
export function takeHandoff(
  location: Pick<Location, "hash" | "pathname" | "search" | "protocol" | "hostname">,
  history: Pick<History, "replaceState" | "state">,
  credentials: Pick<CredentialStore, "takeCloud">,
): boolean {
  if (!new URLSearchParams(location.hash.replace(/^#/, "")).has("cloud")) return false;
  // Out of the address whether or not it is one: what looks like a token
  // is not left in the bar.
  history.replaceState(history.state, "", location.pathname + location.search);
  const handed = readHandoff(location.hash, location);
  if (!handed) return false;
  credentials.takeCloud(handed);
  return true;
}
