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

// Mirrors of internal/api's header constants.
const headerStaffToken = "X-Staff-Token";
const headerIdempotencyKey = "Idempotency-Key";

// A LAN server answers in milliseconds. Ten seconds is long enough for a slow
// SQLite checkpoint and short enough that a till does not hang on a dead one.
const defaultTimeoutMs = 10_000;

export interface RequestOptions {
  signal?: AbortSignal;
  /** Reuse the same key when retrying the same action. See ADR-053. */
  idempotencyKey?: string;
  timeoutMs?: number;
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
}

export function createApiClient({
  credentials,
  fetch: fetchFn = globalThis.fetch.bind(globalThis),
  onCredentialFailure,
}: ApiClientConfig): ApiClient {
  const request = async <T>(method: string, path: string, body: unknown, options: RequestOptions = {}): Promise<T> => {
    try {
      return await send<T>(fetchFn, method, path, body, headersFor(credentials, body, options), options);
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
  return headers;
}

async function send<T>(
  fetchFn: typeof fetch,
  method: string,
  path: string,
  body: unknown,
  headers: Headers,
  options: RequestOptions,
): Promise<T> {
  const deadline = withDeadline(options.signal, options.timeoutMs ?? defaultTimeoutMs);
  let response: Response;
  try {
    response = await fetchFn(path, {
      method,
      headers,
      signal: deadline.signal,
      ...(body === undefined ? {} : { body: body instanceof Blob ? body : JSON.stringify(body) }),
    });
  } catch (err) {
    if (options.signal?.aborted) throw err; // the caller cancelled; not a failure
    if (deadline.timedOut()) throw new ApiError("timeout", "the server did not answer in time");
    throw new ApiError("network", "the server could not be reached");
  } finally {
    deadline.clear();
  }
  return readResponse<T>(response);
}

async function readResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) return undefined as T;

  let parsed: unknown;
  try {
    parsed = await response.json();
  } catch {
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
