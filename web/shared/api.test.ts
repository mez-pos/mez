import { describe, expect, it, vi } from "vitest";
import { ApiError, createApiClient, noCredentials, retryAfter, type Credentials } from "./api";

interface Call {
  path: string;
  init: RequestInit;
}

function fakeFetch(respond: (call: Call) => Response | Promise<Response>) {
  const calls: Call[] = [];
  const fn = vi.fn(async (path: RequestInfo | URL, init?: RequestInit) => {
    const call = { path: String(path), init: init ?? {} };
    calls.push(call);
    return respond(call);
  });
  return { fetch: fn as unknown as typeof fetch, calls };
}

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

const signedIn: Credentials = { deviceToken: () => "nfd_device", staffToken: () => "nfs_staff" };

describe("createApiClient", () => {
  it("sends both tokens and returns the parsed body", async () => {
    const { fetch, calls } = fakeFetch(() => json(200, { id: "d1" }));
    const api = createApiClient({ credentials: signedIn, fetch });

    await expect(api.get<{ id: string }>("/api/v1/devices/me")).resolves.toEqual({ id: "d1" });

    const headers = new Headers(calls[0]?.init.headers);
    expect(calls[0]?.path).toBe("/api/v1/devices/me");
    expect(calls[0]?.init.method).toBe("GET");
    expect(headers.get("Authorization")).toBe("Bearer nfd_device");
    expect(headers.get("X-Staff-Token")).toBe("nfs_staff");
    expect(headers.has("Content-Type")).toBe(false);
  });

  it("sends no auth headers without credentials", async () => {
    const { fetch, calls } = fakeFetch(() => json(200, {}));
    await createApiClient({ credentials: noCredentials, fetch }).get("/api/v1/health");

    const headers = new Headers(calls[0]?.init.headers);
    expect(headers.has("Authorization")).toBe(false);
    expect(headers.has("X-Staff-Token")).toBe(false);
  });

  it("posts JSON with the idempotency key", async () => {
    const { fetch, calls } = fakeFetch(() => json(201, { ok: true }));
    const api = createApiClient({ credentials: signedIn, fetch });

    await api.post("/api/v1/orders", { table: 4 }, { idempotencyKey: "k-1" });

    const headers = new Headers(calls[0]?.init.headers);
    expect(calls[0]?.init.body).toBe('{"table":4}');
    expect(headers.get("Content-Type")).toBe("application/json");
    expect(headers.get("Idempotency-Key")).toBe("k-1");
  });

  it("sends PUT with a JSON body", async () => {
    const { fetch, calls } = fakeFetch(() => Response.json({ ok: true }));
    await createApiClient({ credentials: signedIn, fetch }).put("/api/v1/printers/receipt", { printer_id: 3 });
    expect(calls[0]?.init.method).toBe("PUT");
    expect(calls[0]?.init.body).toBe('{"printer_id":3}');
  });

  it("resolves 204 with nothing", async () => {
    const { fetch } = fakeFetch(() => new Response(null, { status: 204 }));
    await expect(createApiClient({ credentials: signedIn, fetch }).del("/api/v1/sessions/current")).resolves.toBeUndefined();
  });

  const failures = [
    {
      name: "the server's refusal keeps its code, status and field",
      respond: () => json(422, { error: { code: "invalid_request", message: "name is required", field: "name" } }),
      want: { code: "invalid_request", status: 422, field: "name", unreachable: false },
    },
    {
      name: "a revoked device",
      respond: () => json(401, { error: { code: "device_revoked", message: "unpaired" } }),
      want: { code: "device_revoked", status: 401, field: null, unreachable: false },
    },
    {
      name: "a body that is not JSON",
      respond: () => new Response("<html>bad gateway</html>", { status: 502 }),
      want: { code: "bad_response", status: 502, field: null, unreachable: false },
    },
    {
      name: "JSON that is not an error body",
      respond: () => json(500, { oops: true }),
      want: { code: "bad_response", status: 500, field: null, unreachable: false },
    },
    {
      name: "a server that is not there",
      respond: () => Promise.reject(new TypeError("Failed to fetch")),
      want: { code: "network", status: null, field: null, unreachable: true },
    },
  ];
  for (const tt of failures) {
    it(tt.name, async () => {
      const { fetch } = fakeFetch(tt.respond);
      const err = await createApiClient({ credentials: signedIn, fetch }).get("/x").catch((e: unknown) => e);

      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect({ code: apiErr.code, status: apiErr.status, field: apiErr.field, unreachable: apiErr.unreachable }).toEqual(tt.want);
    });
  }

  it("times out a server that never answers", async () => {
    const { fetch } = fakeFetch(
      ({ init }) =>
        new Promise((_, reject) => init.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")))),
    );
    const err = await createApiClient({ credentials: signedIn, fetch })
      .get("/x", { timeoutMs: 20 })
      .catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).code).toBe("timeout");
  });

  it("passes a caller's cancel through untouched", async () => {
    const controller = new AbortController();
    const { fetch } = fakeFetch(
      ({ init }) =>
        new Promise((_, reject) => init.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")))),
    );
    const pending = createApiClient({ credentials: signedIn, fetch }).get("/x", { signal: controller.signal }).catch((e: unknown) => e);
    controller.abort();

    const err = await pending;
    expect(err).not.toBeInstanceOf(ApiError);
    expect((err as DOMException).name).toBe("AbortError");
  });
});

describe("lockouts and credential failures", () => {
  it("reads Retry-After on a lockout", async () => {
    const { fetch } = fakeFetch(
      () => new Response(JSON.stringify({ error: { code: "locked_out", message: "locked" } }), { status: 429, headers: { "Retry-After": "240" } }),
    );
    const err = (await createApiClient({ credentials: signedIn, fetch }).post("/api/v1/sessions", {}).catch((e: unknown) => e)) as ApiError;
    expect([err.code, err.retryAfterMs]).toEqual(["locked_out", 240_000]);
  });

  const failures = [
    { code: "device_revoked", reported: true },
    { code: "unauthenticated", reported: true },
    { code: "sign_in_required", reported: true },
    { code: "invalid_pin", reported: false },
    { code: "invalid_pairing_code", reported: false },
  ];
  for (const tt of failures) {
    it(`${tt.reported ? "reports" : "does not report"} ${tt.code}`, async () => {
      const { fetch } = fakeFetch(() => json(401, { error: { code: tt.code, message: "no" } }));
      const seen: string[] = [];
      await createApiClient({ credentials: signedIn, fetch, onCredentialFailure: (c) => seen.push(c) })
        .get("/x")
        .catch(() => undefined);
      expect(seen).toEqual(tt.reported ? [tt.code] : []);
    });
  }

  it("does not report a 403, whatever its code", async () => {
    const { fetch } = fakeFetch(() => json(403, { error: { code: "unauthenticated", message: "no" } }));
    const seen: string[] = [];
    await createApiClient({ credentials: signedIn, fetch, onCredentialFailure: (c) => seen.push(c) }).get("/x").catch(() => undefined);
    expect(seen).toEqual([]);
  });
});

describe("retryAfter", () => {
  const now = Date.parse("2026-09-21T10:00:00Z");
  const tests = [
    { name: "seconds", header: "120", want: 120_000 },
    { name: "zero", header: "0", want: 0 },
    { name: "an HTTP date", header: "Mon, 21 Sep 2026 10:05:00 GMT", want: 300_000 },
    { name: "a date already past", header: "Mon, 21 Sep 2026 09:00:00 GMT", want: 0 },
    { name: "absent", header: null, want: null },
    { name: "nonsense", header: "soon", want: null },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(retryAfter(tt.header, now)).toBe(tt.want));
  }
});
