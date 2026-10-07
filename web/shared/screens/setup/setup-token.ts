// The setup token in the browser (ADR-270). It arrives in the address's
// fragment (`/setup#token=...`), which a browser keeps to itself: it is not
// sent to the server and reaches no log. It is taken out of the address at
// once, so it is not left in the bar or the history, and kept for the tab's
// life so that a reload of the wizard still holds it.

import type { KeyValue } from "../../auth/storage";

/** The header the server reads the token from. Mirrors internal/api's HeaderSetupToken. */
export const setupTokenHeader = "X-Setup-Token";

const storageKey = "mez.setup.token";
const shape = /^[0-9a-f]{32}$/;

/** The token in a fragment such as "#token=0123...", or null. Only a token's own shape is one. */
export function tokenIn(hash: string): string | null {
  const value = new URLSearchParams(hash.replace(/^#/, "")).get("token");
  return value !== null && shape.test(value) ? value : null;
}

/** What `takeSetupToken` needs of the page, so a test can stand in for it. */
export interface TokenPage {
  readonly hash: string;
  /** Removes the fragment from the address without loading anything. */
  clearHash(): void;
  readonly storage: KeyValue | null;
}

/**
 * The setup token this tab holds: the one in the address if there is one,
 * else the one kept from before. An address carrying one is cleaned either
 * way, and a newer token replaces an older.
 */
export function takeSetupToken(page: TokenPage): string | null {
  const arrived = tokenIn(page.hash);
  if (arrived !== null) {
    try {
      page.storage?.setItem(storageKey, arrived);
    } catch {
      // Storage blocked: the token lasts until the page is reloaded.
    }
    page.clearHash();
    return arrived;
  }
  try {
    const kept = page.storage?.getItem(storageKey) ?? null;
    return kept !== null && shape.test(kept) ? kept : null;
  } catch {
    return null;
  }
}

/** Forgets the token: the venue exists and it opens nothing now. */
export function forgetSetupToken(storage: KeyValue | null): void {
  try {
    storage?.removeItem(storageKey);
  } catch {
    // Nothing to forget where nothing could be kept.
  }
}

/** The headers a setup request carries: the token, when this tab holds one. */
export function setupHeaders(token: string | null): Record<string, string> {
  return token === null ? {} : { [setupTokenHeader]: token };
}

/** This tab, as a TokenPage. */
export function browserPage(): TokenPage {
  let storage: KeyValue | null = null;
  try {
    storage = window.sessionStorage;
  } catch {
    storage = null;
  }
  return {
    hash: window.location.hash,
    clearHash: () => window.history.replaceState(window.history.state, "", window.location.pathname + window.location.search),
    storage,
  };
}
