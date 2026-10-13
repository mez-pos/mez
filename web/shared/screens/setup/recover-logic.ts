// "I had mez before" (setup-wizard.md, ADR-332 to ADR-336): what the wizard
// asks the server to bring a venue back from its owner's mez Cloud account,
// and every sentence it says while doing so. Pure but for the four requests,
// so each is tested without a server, a cloud or a clock.
//
// The server cleans and checks the number and the code too. The checks here
// spare the owner a message sent to a phone, or a try spent, on a slip.

import { ApiError, type ApiClient } from "../../api";
import type {
  DeviceKind,
  RecoverCloudState,
  RecoverOutcomeRequest,
  RecoverOutcomeResponse,
  RecoverRestoreRequest,
  RecoverRestoreResponse,
  RecoverStartRequest,
  RecoverStartResponse,
  RecoverVenue,
  RecoverVerifyRequest,
  RecoverVerifyResponse,
} from "../../api-types";
import { formatWait } from "../../auth/format";
import { unreachableMessage } from "../../auth/messages";
import { backOfficeAddress } from "../../cloud-pair";
import { setupHeaders } from "./setup-token";

/** The four things the owner does, as the frame numbers them. */
export const recoverSteps = [
  { key: "phone", name: "Your number" },
  { key: "code", name: "The code" },
  { key: "venue", name: "Your venue" },
  { key: "back", name: "Bring it back" },
] as const;

export type RecoverStepKey = (typeof recoverSteps)[number]["key"];

export function recoverStepIndex(key: RecoverStepKey): number {
  return recoverSteps.findIndex((s) => s.key === key);
}

/**
 * What the number field starts with. India's code is offered and not assumed:
 * the server refuses a number with no country code, because a code sent to
 * the wrong country's number rings a stranger's phone (ADR-332).
 */
export const offeredCountryCode = "+91 ";

/** The claim's header (ADR-334): what the page shows in place of the setup token once the venue is back. */
export const recoverClaimHeader = "X-Recover-Claim";

const minPhoneDigits = 8;
const maxPhoneDigits = 15;
export const otpLength = 6;

/** A number as it is sent: a plus and its digits. Null if it could not be one. */
export function cleanPhone(typed: string): string | null {
  const trimmed = typed.trim();
  if (!trimmed.startsWith("+")) return null;
  const rest = trimmed.slice(1).replace(/[\s\-.()]/g, "");
  if (!/^[1-9][0-9]*$/.test(rest)) return null;
  if (rest.length < minPhoneDigits || rest.length > maxPhoneDigits) return null;
  return `+${rest}`;
}

/** What is wrong with a number before asking the server. Null when it can be sent. */
export function phoneRefusal(typed: string): string | null {
  const trimmed = typed.trim();
  if (trimmed === "" || trimmed === offeredCountryCode.trim()) return "Type the phone number of your mez Cloud account.";
  if (!trimmed.startsWith("+")) return "Start with + and the country code, like +91 98765 43210.";
  if (cleanPhone(typed) === null) return "That does not look like a phone number. Type it with its country code, like +91 98765 43210.";
  return null;
}

/** A one-time code as it is sent: its six digits. Null if it could not be one. */
export function cleanOtp(typed: string): string | null {
  const digits = typed.replace(/[\s-]/g, "");
  return new RegExp(`^[0-9]{${otpLength}}$`).test(digits) ? digits : null;
}

/** What is wrong with a one-time code before asking the server. Null when it can be sent. */
export function otpRefusal(typed: string): string | null {
  if (typed.trim() === "") return "Type the code that was sent to your phone.";
  if (cleanOtp(typed) === null) return `The code sent to your phone is ${otpLength} digits.`;
  return null;
}

/** A venue with a backup can be brought back. One without has nothing to bring. */
export function canBringBack(venue: RecoverVenue): boolean {
  return Boolean(venue.last_backup_at);
}

/** The venue the list starts on: the only one that can be brought back, if there is only one. */
export function onlyChoice(venues: readonly RecoverVenue[]): string | null {
  const able = venues.filter(canBringBack);
  return able.length === 1 && able[0] ? able[0].id : null;
}

/**
 * When a venue was last backed up, for its row. The time is this computer's:
 * there is no venue yet to say which zone is the venue's.
 */
export function lastBackedUp(venue: RecoverVenue, locale?: string, timeZone?: string): string {
  if (!venue.last_backup_at) return "No backup with mez Cloud. It cannot be brought back.";
  const at = new Date(venue.last_backup_at);
  if (Number.isNaN(at.getTime())) return "Backed up with mez Cloud.";
  const when = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit", timeZone }).format(at);
  return `Last backed up ${when}`;
}

/** What the venue step says above the list, by what the account has. */
export function venuesLead(owner: string, venues: readonly RecoverVenue[]): string {
  if (venues.length === 0) return `${owner}, this mez Cloud account has no venues. Set this computer up as a new venue instead.`;
  if (!venues.some(canBringBack)) {
    return `${owner}, none of this account's venues has a backup with mez Cloud, so there is nothing to bring back. Set this computer up as a new venue instead.`;
  }
  return venues.length === 1
    ? `${owner}, this is the venue on your account. Bringing it back puts its menu, its staff and every sale up to its last backup on this computer.`
    : `${owner}, choose the venue this computer is to be. Bringing it back puts its menu, its staff and every sale up to its last backup on this computer.`;
}

export interface RecoverDeps {
  api: ApiClient;
  /** The setup token, for the three requests before mez starts again. */
  token: string | null;
}

/** Asks mez Cloud to send a code to the account's phone. Clean the number first. */
export function startRecovery(deps: RecoverDeps, phone: string): Promise<RecoverStartResponse> {
  return deps.api.post<RecoverStartResponse>("/api/v1/setup/recover/start", { phone } satisfies RecoverStartRequest, { headers: setupHeaders(deps.token) });
}

/** Gives the code back and signs in to the account. */
export function verifyRecovery(deps: RecoverDeps, attempt: string, code: string): Promise<RecoverVerifyResponse> {
  return deps.api.post<RecoverVerifyResponse>("/api/v1/setup/recover/verify", { attempt, code } satisfies RecoverVerifyRequest, {
    headers: setupHeaders(deps.token),
  });
}

/**
 * The longest the page waits for a venue's backup to come down. The server
 * gives it half an hour (ADR-334), and answers before then; a minute more
 * means it is the server's answer the owner reads and not this page's guess.
 */
export const restoreTimeoutMs = 31 * 60_000;

/** Brings the venue down. The answer comes when it is staged, and mez starts again as it is read. */
export function restoreVenue(deps: RecoverDeps, venueId: string): Promise<RecoverRestoreResponse> {
  return deps.api.post<RecoverRestoreResponse>("/api/v1/setup/recover/restore", { venue_id: venueId } satisfies RecoverRestoreRequest, {
    headers: setupHeaders(deps.token),
    timeoutMs: restoreTimeoutMs,
  });
}

/** How it went, for whoever holds the claim. With a device named, the answer carries the code that pairs this browser. */
export function askOutcome(api: ApiClient, claim: string, device?: { name: string; kind: DeviceKind }): Promise<RecoverOutcomeResponse> {
  const body: RecoverOutcomeRequest = device ? { device_name: device.name, device_kind: device.kind } : {};
  return api.post<RecoverOutcomeResponse>("/api/v1/setup/recover/outcome", body, { headers: { [recoverClaimHeader]: claim } });
}

/** Why a step did not work, in words for the owner. The server's own are already an owner's (ADR-335). */
export function recoverRefusal(err: unknown, host: string): string {
  if (!(err instanceof ApiError)) return "Something went wrong. Try again.";
  if (err.code === "timeout") return "mez did not answer in time. Nothing on this computer was changed. Try again.";
  if (err.unreachable) return unreachableMessage(host);
  if (err.code === "rate_limited") {
    // This computer's own limit and mez Cloud's "busy" come under one code, and one sentence does for both.
    const wait = err.retryAfterMs !== null ? formatWait(err.retryAfterMs) : "a minute";
    return `Too many tries, or mez Cloud is busy. Wait ${wait} and try again.`;
  }
  if (err.code === "setup_locked") return "This page can no longer set this computer up. Open mez again from the installer, or from the setup link it prints.";
  return err.message;
}

/** Where a refusal belongs: under the number, under the code, or on the step. */
export function refusalField(err: unknown): "phone" | "code" | null {
  if (!(err instanceof ApiError)) return null;
  return err.field === "phone" || err.field === "code" ? err.field : null;
}

/** A sign-in that is over: the owner is sent back to ask for a new code. */
export function signedOut(err: unknown): boolean {
  return err instanceof ApiError && err.code === "recover_signed_out";
}

/** Someone made a venue here meanwhile, or one was there all along. */
export function alreadySetUp(err: unknown): boolean {
  return err instanceof ApiError && err.code === "already_set_up";
}

// How the page waits for mez to start again on the venue (ADR-334, ADR-335).
export const waiting = {
  /** Between two askings. The server allows ninety a minute. */
  everyMs: 1500,
  /**
   * How long "nothing to tell" is taken for "not yet". The start that staged
   * the venue answers so for the moment it is still up: it adopted nothing.
   */
  graceMs: 20_000,
  /** How long in all before the page says it cannot tell. A start takes a second or two. */
  giveUpMs: 90_000,
  /** How long the page goes on asking whether the venue is paired, once it is back. */
  pairingMs: 45_000,
} as const;

export type Arrival =
  | { kind: "back"; outcome: RecoverOutcomeResponse }
  /** The server says there is nothing to tell this page: the claim ran out, or the venue was not put in place. */
  | { kind: "untold" }
  /** Nothing answered for as long as the page was willing to wait. */
  | { kind: "silent" };

export interface WaitDeps {
  ask: () => Promise<RecoverOutcomeResponse>;
  sleep: (ms: number) => Promise<void>;
  now: () => number;
}

/**
 * Waits for mez to start again and say the venue is back. A server that does
 * not answer is the start itself, and is waited through. It never throws.
 */
export async function awaitVenue(deps: WaitDeps): Promise<Arrival> {
  const began = deps.now();
  for (;;) {
    const waited = deps.now() - began;
    try {
      return { kind: "back", outcome: await deps.ask() };
    } catch (err) {
      const api = err instanceof ApiError ? err : null;
      if (api?.code === "recover_nothing" && waited >= waiting.graceMs) return { kind: "untold" };
      if (waited >= waiting.giveUpMs) return { kind: "silent" };
      // Asked too fast: wait as long as the server says, and no less than usual.
      // And never past the moment the page would give up: a wait it cannot explain is a page that has stopped.
      const asked = api?.code === "rate_limited" && api.retryAfterMs !== null ? Math.max(api.retryAfterMs, waiting.everyMs) : waiting.everyMs;
      await deps.sleep(Math.min(asked, Math.max(waiting.giveUpMs - waited, waiting.everyMs)));
    }
  }
}

export interface PairDeps {
  /** The outcome, asked for with the device named: its answer carries the code. */
  askNamed: () => Promise<RecoverOutcomeResponse>;
  /** Pairs this browser with the code (AuthStore.pair). */
  pair: (code: string) => Promise<void>;
  sleep: (ms: number) => Promise<void>;
}

/** How many times the page asks for this browser's code before sending the owner to `mez pair`. */
export const pairTries = 3;

export interface PairedBrowser {
  paired: boolean;
  /** The last answer that came, which is fresher than the one the wait ended on. Null if none did. */
  outcome: RecoverOutcomeResponse | null;
}

/**
 * Pairs the browser the owner is at with the venue brought back: it is no
 * device of that venue's, whose devices were the old computer's. The code is
 * the same however often it is asked for (ADR-334), so asking again after a
 * slip costs nothing; a code that will not pair is not tried twice. It never
 * throws: the venue is back whether or not this browser is paired.
 */
export async function pairThisBrowser(deps: PairDeps): Promise<PairedBrowser> {
  let outcome: RecoverOutcomeResponse | null = null;
  for (let attempt = 1; attempt <= pairTries; attempt++) {
    let code: string | undefined;
    try {
      outcome = await deps.askNamed();
      code = outcome.pairing_code;
    } catch (err) {
      // The claim is no longer one: asking again will be answered the same.
      if (err instanceof ApiError && err.code === "recover_nothing") return { paired: false, outcome };
    }
    if (code) {
      try {
        await deps.pair(code);
        return { paired: true, outcome };
      } catch {
        return { paired: false, outcome };
      }
    }
    if (attempt < pairTries) await deps.sleep(waiting.everyMs);
  }
  return { paired: false, outcome };
}

/** Whether to ask again how the pairing went. */
export function stillPairing(state: RecoverCloudState): boolean {
  return state === "pairing";
}

/** What became of the venue's pairing to its account, for the last card. */
export function cloudLines(outcome: Pick<RecoverOutcomeResponse, "cloud" | "hostname">, gaveUp = false): string[] {
  switch (outcome.cloud) {
    case "paired":
      return outcome.hostname
        ? ["It is paired to your mez Cloud account again. From a phone or a computer elsewhere, the back office opens at:", backOfficeAddress(outcome.hostname)]
        : ["It is paired to your mez Cloud account again."];
    case "pairing":
      return gaveUp
        ? ["It is still being paired to your mez Cloud account. Look in the back office under Devices in a minute."]
        : ["Pairing it to your mez Cloud account again."];
    case "not_paired":
      return [
        "It could not be paired to your mez Cloud account again. The till works all the same. To pair it, make a code in your mez Cloud account and enter it in the back office under Devices.",
      ];
    case "none":
      return [];
  }
}

/** What the last card says of the menu's photos. Nothing, when they all came. */
export function photosLine(restored: Pick<RecoverRestoreResponse, "photos" | "photos_incomplete">): string | null {
  if (!restored.photos_incomplete) return null;
  return restored.photos === 0
    ? "The menu's photos could not be brought back. The dishes are all there; add their photos again in the back office."
    : "Not all of the menu's photos could be brought back. The dishes are all there; add the missing photos again in the back office.";
}

/** This till cannot be paired from this page: the way on is a code made at the machine. */
export const needsCode = "Your venue is back, but this till could not be paired to it from here. On this computer, run mez pair and enter the code it prints.";

/** The page cannot be told how it went. The venue may well be there. */
export const untold =
  "mez can no longer tell this page how bringing your venue back went. Open mez again on this computer: if your venue is there, sign in as you did before. If it asks to pair this device, run mez pair on this computer and enter the code it prints.";

/** Nothing answered. */
export function silent(host: string): string {
  return `mez at ${host} has not answered since it began starting again on your venue. Check that mez is running on this computer, then open it again: your venue is put in place when it starts.`;
}
