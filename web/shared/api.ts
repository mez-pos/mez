// The HTTP client every app uses. Request and response shapes come from
// api-types.ts, which is generated from the Go structs; never restate one here.
//
// Paths are origin-relative (`/api/v1/...`). In production the Go server
// serves the app and the API from one origin; in development the Vite proxy
// makes that true too. So nothing here knows which one it is in.

import type { ApiErrorBody, ErrorCode } from "./api-types";

/** Where the two tokens come from: auth/session-store.ts keeps them. */
export interface Credentials {
  deviceToken(): string | null;
  staffToken(): string | null;
}

export const noCredentials: Credentials = {
  deviceToken: () => null,
  staffToken: () => null,
};

/**
 * Why a request failed. `code` is the server's error code when it answered,
 * `network` when it could not be reached, `timeout` when it did not answer in
 * time, and `bad_response` when it answered with something that was not JSON.
 * A screen switches on `code`, never on `message`, which is for humans.
 */
export type FailureCode = ErrorCode | "network" | "timeout" | "bad_response";

export class ApiError extends Error {
  override readonly name = "ApiError";

  constructor(
    readonly code: FailureCode,
    message: string,
    readonly status: number | null = null,
    readonly field: string | null = null,
    /** From Retry-After, on a lockout or a rate limit: how long to wait. */
    readonly retryAfterMs: number | null = null,
  ) {
    super(message);
  }

  /** The server was not reached at all, so retrying may help. */
  get unreachable(): boolean {
    return this.code === "network" || this.code === "timeout";
  }
}

/**
 * The server's error, whether thrown as it is or as the cause of an error a
 * screen put into its own words (TicketQueue does). Null for anything else.
 */
export function apiErrorOf(err: unknown): ApiError | null {
  if (err instanceof ApiError) return err;
  if (err instanceof Error && err.cause instanceof ApiError) return err.cause;
  return null;
}

/**
 * The server did not answer for itself: no network, no answer in time, or a
 * relay or proxy saying 502, 503 or 504 because it could not reach the venue.
 * Whatever was sent may or may not have been done.
 */
export function isUnreachable(err: unknown): boolean {
  const api = apiErrorOf(err);
  if (!api) return false;
  return api.unreachable || (api.status !== null && api.status >= 502 && api.status <= 504);
}

/** The server's own words when it answered, or the fallback when it was not reached or the failure is not the server's. */
export function wordsOf(err: unknown, fallback: string): string {
  const api = apiErrorOf(err);
  return api && !isUnreachable(api) ? api.message : fallback;
}

// Mirrors of internal/api's header constants.
const headerStaffToken = "X-Staff-Token";
const headerIdempotencyKey = "Idempotency-Key";

/**
 * An answer that came back through the relay says so in this header
 * (internal/api's HeaderVia and ViaRelay, ADR-308). It is how the back office
 * learns it is being used away from the venue.
 */
export const headerVia = "X-Mez-Via";
export const viaRelay = "relay";

/** Whether an answer came through the relay and not from the venue's own network. */
export function cameThroughRelay(response: Pick<Response, "headers">): boolean {
  return response.headers.get(headerVia) === viaRelay;
}

// A LAN server answers in milliseconds. Ten seconds is long enough for a slow
// SQLite checkpoint and short enough that a till does not hang on a dead one.
const defaultTimeoutMs = 10_000;

export interface RequestOptions {
  signal?: AbortSignal;
  /** Reuse the same key when retrying the same action. See ADR-053. */
  idempotencyKey?: string;
  timeoutMs?: number;
  /** Headers of this request's own, beyond the credentials: the setup token (ADR-270). */
  headers?: Readonly<Record<string, string>>;
}

export interface ApiClient {
  get<T>(path: string, options?: RequestOptions): Promise<T>;
  post<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T>;
  /** A Blob body is sent as it is, with its own type: a photo (ADR-110). Anything else is JSON. */
  put<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T>;
  del<T>(path: string, options?: RequestOptions): Promise<T>;
}

/**
 * The refusals that mean the stored credentials are no longer good, from any
 * request at all. The auth store listens, so a session that expired while a
 * screen was open sends that screen back to the lock, and a device unpaired
 * from the back office goes back to pairing.
 */
export type CredentialFailure = Extract<ErrorCode, "unauthenticated" | "device_revoked" | "sign_in_required">;

const credentialFailures: readonly string[] = ["unauthenticated", "device_revoked", "sign_in_required"] satisfies CredentialFailure[];

export interface ApiClientConfig {
  credentials: Credentials;
  fetch?: typeof fetch;
  onCredentialFailure?: (code: CredentialFailure) => void;
  /** Shown every answer the server gave, good or bad, before its body is read: for its headers. */
  onResponse?: (response: Response) => void;
}

export function createApiClient({
  credentials,
  fetch: fetchFn = globalThis.fetch.bind(globalThis),
  onCredentialFailure,
  onResponse,
}: ApiClientConfig): ApiClient {
  const request = async <T>(method: string, path: string, body: unknown, options: RequestOptions = {}): Promise<T> => {
    try {
      return await send<T>(fetchFn, method, path, body, headersFor(credentials, body, options), options, onResponse);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401 && credentialFailures.includes(err.code)) {
        onCredentialFailure?.(err.code as CredentialFailure);
      }
      throw err;
    }
  };

  return {
    get: (path, options) => request("GET", path, undefined, options),
    post: (path, body, options) => request("POST", path, body, options),
    put: (path, body, options) => request("PUT", path, body, options),
    del: (path, options) => request("DELETE", path, undefined, options),
  };
}

function headersFor(credentials: Credentials, body: unknown, options: RequestOptions): Headers {
  const headers = new Headers({ Accept: "application/json" });
  const device = credentials.deviceToken();
  const staff = credentials.staffToken();
  if (device) headers.set("Authorization", `Bearer ${device}`);
  if (staff) headers.set(headerStaffToken, staff);
  if (body instanceof Blob) headers.set("Content-Type", body.type || "application/octet-stream");
  else if (body !== undefined) headers.set("Content-Type", "application/json");
  if (options.idempotencyKey) headers.set(headerIdempotencyKey, options.idempotencyKey);
  for (const [name, value] of Object.entries(options.headers ?? {})) headers.set(name, value);
  return headers;
}

async function send<T>(
  fetchFn: typeof fetch,
  method: string,
  path: string,
  body: unknown,
  headers: Headers,
  options: RequestOptions,
  onResponse?: (response: Response) => void,
): Promise<T> {
  const deadline = withDeadline(options.signal, options.timeoutMs ?? defaultTimeoutMs);
  try {
    let response: Response;
    try {
      response = await fetchFn(path, {
        method,
        headers,
        signal: deadline.signal,
        ...(body === undefined ? {} : { body: body instanceof Blob ? body : JSON.stringify(body) }),
      });
    } catch (err) {
      throw notAnswered(err, options.signal, deadline);
    }
    onResponse?.(response);
    return await readResponse<T>(response, options.signal, deadline);
  } finally {
    // Only once the body is read too. A server that sends the headers and
    // then stalls must still time out, and a caller's cancel must still
    // land, or a payment screen waits on it for good.
    deadline.clear();
  }
}

/** Why a request that got no answer failed: the caller's own cancel is passed through as it is. */
function notAnswered(err: unknown, outer: AbortSignal | undefined, deadline: Deadline): unknown {
  if (outer?.aborted) return err;
  if (deadline.timedOut()) return new ApiError("timeout", "the server did not answer in time");
  return new ApiError("network", "the server could not be reached");
}

async function readResponse<T>(response: Response, outer: AbortSignal | undefined, deadline: Deadline): Promise<T> {
  if (response.status === 204) return undefined as T;

  let parsed: unknown;
  try {
    parsed = await jsonWithin(response, deadline.signal);
  } catch (err) {
    if (outer?.aborted || deadline.timedOut()) throw notAnswered(err, outer, deadline);
    throw new ApiError("bad_response", `the server answered ${response.status} with something that was not JSON`, response.status);
  }
  if (response.ok) return parsed as T;

  if (isErrorBody(parsed)) {
    const { code, message, field } = parsed.error;
    throw new ApiError(code, message, response.status, field ?? null, retryAfter(response.headers.get("Retry-After")));
  }
  throw new ApiError("bad_response", `the server answered ${response.status} without an error body`, response.status);
}

/** Retry-After is seconds or an HTTP date (RFC 9110); either becomes milliseconds. */
export function retryAfter(header: string | null, now: number = Date.now()): number | null {
  if (header === null || header.trim() === "") return null;
  if (/^\d+$/.test(header.trim())) return Number(header.trim()) * 1000;
  const at = Date.parse(header);
  return Number.isNaN(at) ? null : Math.max(0, at - now);
}

function isErrorBody(value: unknown): value is ApiErrorBody {
  if (typeof value !== "object" || value === null || !("error" in value)) return false;
  const error: unknown = value.error;
  return typeof error === "object" && error !== null && "code" in error && typeof error.code === "string";
}

/**
 * The body as JSON, or a rejection the moment the signal aborts. A browser's
 * fetch stops the read itself when its signal aborts, but the read is what
 * the deadline has to cover, so this does not count on it.
 */
function jsonWithin(response: Response, signal: AbortSignal): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const abort = () => reject(new DOMException("The body was not read in time.", "AbortError"));
    if (signal.aborted) {
      abort();
      return;
    }
    signal.addEventListener("abort", abort, { once: true });
    response
      .json()
      .then(resolve, reject)
      .finally(() => signal.removeEventListener("abort", abort));
  });
}

interface Deadline {
  signal: AbortSignal;
  timedOut(): boolean;
  clear(): void;
}

// AbortSignal.any and AbortSignal.timeout would do this in one line, but
// AbortSignal.any is missing from Safari before 17.4, and a guest's phone may
// be older than that.
function withDeadline(outer: AbortSignal | undefined, timeoutMs: number): Deadline {
  const controller = new AbortController();
  let expired = false;
  const timer = setTimeout(() => {
    expired = true;
    controller.abort();
  }, timeoutMs);
  const forward = () => controller.abort();
  if (outer?.aborted) controller.abort();
  outer?.addEventListener("abort", forward, { once: true });

  return {
    signal: controller.signal,
    timedOut: () => expired,
    clear: () => {
      clearTimeout(timer);
      outer?.removeEventListener("abort", forward);
    },
  };
}
