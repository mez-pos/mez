// What a guest's phone reads from the venue (ADR-206): the table its code
// leads to, and the menu. Both are under the guest app's own mount, by the
// same absolute path on the venue's wifi and through the relay.

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { createApiClient, noCredentials } from "../../shared/api";
import { BILL_REFRESH_MS } from "./bill-logic";
import type {
  GuestBillResponse,
  GuestChargeResponse,
  GuestMenuResponse,
  GuestOrderRequest,
  GuestPaidResponse,
  GuestPickupResponse,
  GuestReceiptResponse,
  GuestRoundResponse,
  GuestShareRequest,
  GuestTableResponse,
} from "../../shared/api-types";
import { recoveryOf, needsSession, SESSION_HEADER, sessionOf, sessionStorageKey, tableCodeOfPath } from "./session-logic";
import { newPhoneName, PHONE_STORAGE_KEY, phoneNameOf } from "./split-logic";

// A guest is nobody: no device and no staff session. The code in the path
// says which table, and the phone's session for the sitting (ADR-262) goes
// with every call under that table's code.

/** Sessions for the life of the page, when storage will not keep one. */
const unkeptSessions = new Map<string, string>();

/** The session this phone holds for a table, if it has one. */
export function sessionFor(code: string): string | null {
  const unkept = unkeptSessions.get(code);
  if (unkept !== undefined) return unkept;
  try {
    return sessionOf(window.localStorage.getItem(sessionStorageKey(code)));
  } catch {
    return null;
  }
}

/** Keeps a session the venue handed out, or with null drops the one held. Anything that is not a session is ignored. */
function keepSession(code: string, value: unknown): void {
  const session = value === null ? null : sessionOf(value);
  if (value !== null && session === null) return;
  if (session === null) unkeptSessions.delete(code);
  else unkeptSessions.set(code, session);
  try {
    if (session === null) window.localStorage.removeItem(sessionStorageKey(code));
    else window.localStorage.setItem(sessionStorageKey(code), session);
  } catch {
    // Kept for as long as the page, then.
  }
}

/** fetch, with the session of the table whose code is in the path. */
const withSessionHeader: typeof fetch = (input, init) => {
  const path = typeof input === "string" ? input : input instanceof URL ? input.pathname : input.url;
  const code = tableCodeOfPath(path);
  const session = code === null ? null : sessionFor(code);
  if (session === null) return globalThis.fetch(input, init);
  const headers = new Headers(init?.headers);
  headers.set(SESSION_HEADER, session);
  return globalThis.fetch(input, { ...init, headers });
};

const api = createApiClient({ credentials: noCredentials, fetch: withSessionHeader });

/** One reading of the landing at a time for each table, however many calls were refused at once. */
const relanding = new Map<string, Promise<string | null>>();

/** Reads the landing again, for a new session; null when it gave none, the table having a ticket. */
function reland(code: string): Promise<string | null> {
  const going = relanding.get(code);
  if (going) return going;
  const read = api
    .get<GuestTableResponse>(base(code))
    .then((landing) => {
      const fresh = sessionOf(landing.session);
      if (fresh !== null) keepSession(code, fresh);
      return fresh;
    })
    .finally(() => relanding.delete(code));
  relanding.set(code, read);
  return read;
}

/**
 * Makes a call that needs the phone's session. Refused for want of one, the
 * session held is dropped and the landing read again; the call is made once
 * more with what it gave, or for an order with none, which asks staff to let
 * the phone join. Otherwise the refusal stands, and the screen says to ask
 * staff to accept the phone's first order.
 */
async function withSession<T>(code: string, call: () => Promise<T>, ordering = false): Promise<T> {
  try {
    return await call();
  } catch (err) {
    if (!needsSession(err)) throw err;
    keepSession(code, null);
    if (recoveryOf(await reland(code), ordering) === "ask_staff") throw err;
    return call();
  }
}

/** Keeps a session an order's answer handed over (ADR-262), and passes the answer on. */
function handedOver(code: string, round: GuestRoundResponse): GuestRoundResponse {
  if (round.session !== undefined) keepSession(code, round.session);
  return round;
}

// The venue's pickup code, when that is what the page was opened with
// (ADR-250). A page has one code for its life, and a pickup code is never
// the same string as a table's (ADR-247), so the code itself says which API
// it is asked under: "p" for this one, "t" for any other.
let collecting: string | null = null;

/** Says the page's code is the venue's pickup code. Called once, before anything is asked. */
export function enterCollect(code: string): void {
  collecting = code;
}

const base = (code: string) => `/order/api/${code === collecting ? "p" : "t"}/${encodeURIComponent(code)}`;

/** How often the menu is read again: what is sold out turns on the kitchen's clock. */
const MENU_REFRESH_MS = 60_000;

export function menuKey(code: string): readonly string[] {
  return ["guest", "menu", code];
}

/**
 * Places an order, or asks how one placed with this key stands: the server
 * answers the same key with the same order, as it is now (ADR-210).
 */
export function sendOrder(code: string, request: GuestOrderRequest, key: string): Promise<GuestRoundResponse> {
  const send = () => api.post<GuestRoundResponse>(`${base(code)}/orders`, request, { idempotencyKey: key });
  return code === collecting ? send() : withSession(code, send, true).then((round) => handedOver(code, round));
}

/**
 * How an order placed at this table stands, and once sent how far the
 * kitchen has come (ADR-238). Accepted with no session of the phone's own,
 * it hands one over (ADR-262), which is kept.
 */
export function trackOrder(code: string, round: string): Promise<GuestRoundResponse> {
  return api.get<GuestRoundResponse>(`${base(code)}/orders/${encodeURIComponent(round)}`).then((r) => handedOver(code, r));
}

/** A settled bill's receipt: the counter's own invoice, by the table's code and the bill's id (ADR-240). */
export function readReceipt(code: string, bill: string, signal: AbortSignal): Promise<GuestReceiptResponse> {
  return api.get<GuestReceiptResponse>(`${base(code)}/receipts/${encodeURIComponent(bill)}`, { signal });
}

/**
 * Where the page's code leads: a table at a venue, or for a pickup code the
 * venue alone. With no table its name is empty, which the screens read as
 * "to collect" (tableLabel).
 */
export interface Where extends GuestTableResponse {
  readonly collect: boolean;
  /** False where the venue takes no orders to collect: guests cannot pay online there. */
  readonly ordering: boolean;
}

export function useTable(code: string): UseQueryResult<Where> {
  return useQuery({
    queryKey: ["guest", "table", code],
    queryFn: async ({ signal }): Promise<Where> => {
      if (code !== collecting) {
        const landing = await api.get<GuestTableResponse>(base(code), { signal });
        // A session handed out here is the phone's for the sitting (ADR-262).
        if (landing.session !== undefined) keepSession(code, landing.session);
        return { ...landing, collect: false, ordering: true };
      }
      const venue = await api.get<GuestPickupResponse>(base(code), { signal });
      return { venue: venue.venue, table: { name: "" }, collect: true, ordering: venue.ordering };
    },
  });
}

export function useGuestMenu(code: string, enabled: boolean): UseQueryResult<GuestMenuResponse> {
  return useQuery({
    queryKey: menuKey(code),
    queryFn: ({ signal }) => api.get<GuestMenuResponse>(`${base(code)}/menu`, { signal }),
    enabled,
    refetchInterval: MENU_REFRESH_MS,
    // A phone comes back from the lock screen with a menu that may be an
    // hour old.
    refetchOnWindowFocus: true,
  });
}

/**
 * The table's bill, read while its screen is shown and again every few
 * seconds: a round accepted or a payment taken moves it.
 */
export function billKey(code: string): readonly string[] {
  return ["guest", "bill", code];
}

/** Tells staff a guest at the table says they have paid (ADR-218). It pays nothing. */
export function tellPaid(code: string): Promise<GuestPaidResponse> {
  return withSession(code, () => api.post<GuestPaidResponse>(`${base(code)}/bill/paid`));
}

/** The name for the life of the page, when storage will not keep one. */
let unkept: string | null = null;

/**
 * The name this phone gave itself (ADR-233), made once and kept: how the
 * venue tells one guest's share from another's of the same amount. Null
 * where the browser can make no random name; the phone then pays the rest
 * of a bill and no share of it.
 */
export function phoneName(): string | null {
  if (unkept !== null) return unkept;
  try {
    const kept = phoneNameOf(window.localStorage.getItem(PHONE_STORAGE_KEY));
    if (kept !== null) return kept;
  } catch {
    // Storage refused: a name is made below and lasts until the page goes.
  }
  let made: string;
  try {
    made = newPhoneName(crypto.getRandomValues(new Uint8Array(16)));
  } catch {
    return null;
  }
  try {
    window.localStorage.setItem(PHONE_STORAGE_KEY, made);
  } catch {
    unkept = made;
  }
  return made;
}

export function useBill(code: string): UseQueryResult<GuestBillResponse> {
  const phone = phoneName();
  const who = phone === null ? "" : `?phone=${encodeURIComponent(phone)}`;
  return useQuery({
    queryKey: billKey(code),
    queryFn: ({ signal }) => withSession(code, () => api.get<GuestBillResponse>(`${base(code)}/bill${who}`, { signal })),
    refetchInterval: BILL_REFRESH_MS,
    refetchOnWindowFocus: true,
    // Never shown from an earlier look without asking again.
    staleTime: 0,
    gcTime: 0,
  });
}

/**
 * The bill's UPI payment drawn as a QR code (ADR-217), as a picture to show.
 * It is read with the phone's session (ADR-262), which a plain image cannot
 * send. The amount is in the address only so the picture is asked for again
 * when it changes: the server draws the balance as it stands and reads no
 * amount from a phone.
 */
export async function readUpiQr(code: string, amountMinor: number, signal: AbortSignal): Promise<Blob> {
  const response = await withSessionHeader(`${base(code)}/bill/upi.svg?am=${amountMinor}`, { signal });
  if (!response.ok) throw new Error(`the QR code was not drawn: ${response.status}`);
  return response.blob();
}

/**
 * Starts paying through the venue's gateway (ADR-222): with no share, the
 * rest of what the table owes; with one, this phone's part of it (ADR-233).
 */
export function startCharge(code: string, share: GuestShareRequest | null = null): Promise<GuestChargeResponse> {
  return withSession(code, () => api.post<GuestChargeResponse>(`${base(code)}/bill/charge`, share ?? undefined));
}

/** Gives up a payment page this table opened, so it holds no part of the bill (ADR-234). */
export function letGoCharge(code: string, id: string): Promise<GuestChargeResponse> {
  return withSession(code, () => api.post<GuestChargeResponse>(`${base(code)}/bill/charge/${encodeURIComponent(id)}/let-go`));
}

/** Starts paying for an order the venue is holding until it is paid (ADR-227). */
export function startRoundCharge(code: string, round: string): Promise<GuestChargeResponse> {
  return api.post<GuestChargeResponse>(`${base(code)}/orders/${encodeURIComponent(round)}/charge`);
}

/** How a charge this table started stands, as the venue's server found out from the gateway. */
export function readCharge(code: string, id: string, signal: AbortSignal): Promise<GuestChargeResponse> {
  // An order to collect has no bill: its payment is asked about beside it.
  const read = () => api.get<GuestChargeResponse>(`${base(code)}/${code === collecting ? "" : "bill/"}charge/${encodeURIComponent(id)}`, { signal });
  return code === collecting ? read() : withSession(code, read);
}
