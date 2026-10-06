// The phone's session for the sitting, without React (ADR-262). A table's
// code says which table and never expires, so the venue asks a phone for a
// session as well before it lets it order to the kitchen or read the bill.
// The landing hands one out while the table has no ticket; the venue hands
// one over on the order's own address once staff let the phone's order onto
// a ticket it had no session for. The phone keeps it by table code and sends
// it with every call.

import { apiErrorOf } from "../../shared/api";

/** The header a session goes in. A mirror of internal/api's HeaderGuestSession. */
export const SESSION_HEADER = "X-Guest-Session";

export function sessionStorageKey(code: string): string {
  return `mez.session.${code}`;
}

/** A session as the server draws one: gst_ and 32 bytes, base64url. Storage and answers are not trusted for shape. */
const SESSION = /^gst_[A-Za-z0-9_-]{43}$/;

/** The session in a stored value or an answer, or null for anything that is not one. */
export function sessionOf(value: unknown): string | null {
  return typeof value === "string" && SESSION.test(value) ? value : null;
}

/** The venue says this phone has no session good for the table's sitting now. */
export function needsSession(error: unknown): boolean {
  return apiErrorOf(error)?.code === "guest_session_required";
}

/** The table's code in a path of the guest API, or null: whose session a call carries. A pickup code has none. */
export function tableCodeOfPath(path: string): string | null {
  const match = /^\/order\/api\/t\/([^/?#]+)/.exec(path);
  if (!match?.[1]) return null;
  try {
    return decodeURIComponent(match[1]);
  } catch {
    return null;
  }
}

/**
 * What to do after the venue refused the phone's session, the session was
 * dropped and the landing read again. With a new session from it, ask once
 * more. With none, the table has a ticket this phone is not part of: an
 * order is asked once more without one, which asks staff to let the phone
 * join; anything else stops, and the guest is told to ask staff to accept
 * their first order.
 */
export type Recovery = "again" | "ask_staff";

export function recoveryOf(fresh: string | null, ordering: boolean): Recovery {
  return fresh !== null || ordering ? "again" : "ask_staff";
}
