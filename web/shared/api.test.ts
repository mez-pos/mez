import { describe, expect, it, vi } from "vitest";
import { ApiError, apiErrorOf, cameThroughRelay, createApiClient, isUnreachable, noCredentials, retryAfter, wordsOf, type Credentials } from "./api";

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

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

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

  it("sends a Blob as it is, with its own type", async () => {
    const { fetch, calls } = fakeFetch(() => json(200, { photo: "p.jpg" }));
    const api = createApiClient({ credentials: signedIn, fetch });
    const photo = new Blob([new Uint8Array([0xff, 0xd8, 0xff])], { type: "image/jpeg" });
    await api.put("/api/v1/menu/items/1/photo", photo);
    expect(new Headers(calls[0]?.init.headers).get("Content-Type")).toBe("image/jpeg");
    expect(calls[0]?.init.body).toBe(photo);
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
      const err = await createApiClient({ credentials: signedIn, fetch })
        .get("/x")
        .catch((e: unknown) => e);

      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect({ code: apiErr.code, status: apiErr.status, field: apiErr.field, unreachable: apiErr.unreachable }).toEqual(tt.want);
    });
  }

  it("times out a server that never answers", async () => {
    const { fetch } = fakeFetch(
      ({ init }) => new Promise((_, reject) => init.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")))),
    );
    const err = await createApiClient({ credentials: signedIn, fetch })
      .get("/x", { timeoutMs: 20 })
      .catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).code).toBe("timeout");
  });

  // Headers at once, then a body that never ends.
  const stalled = () => new Response(new ReadableStream({ start: () => undefined }), { status: 200, headers: { "Content-Type": "application/json" } });

  it("times out a server whose body never finishes", async () => {
    const { fetch } = fakeFetch(stalled);
    const err = await createApiClient({ credentials: signedIn, fetch })
      .get("/x", { timeoutMs: 20 })
      .catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).code).toBe("timeout");
  });

  it("lets a caller cancel while the body is still being read", async () => {
    const controller = new AbortController();
    const { fetch } = fakeFetch(stalled);
    const pending = createApiClient({ credentials: signedIn, fetch })
      .get("/x", { signal: controller.signal })
      .catch((e: unknown) => e);
    await Promise.resolve();
    controller.abort();

    const err = await pending;
    expect(err).not.toBeInstanceOf(ApiError);
    expect((err as DOMException).name).toBe("AbortError");
  });

  it("passes a caller's cancel through untouched", async () => {
    const controller = new AbortController();
    const { fetch } = fakeFetch(
      ({ init }) => new Promise((_, reject) => init.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")))),
    );
    const pending = createApiClient({ credentials: signedIn, fetch })
      .get("/x", { signal: controller.signal })
      .catch((e: unknown) => e);
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
    const err = (await createApiClient({ credentials: signedIn, fetch })
      .post("/api/v1/sessions", {})
      .catch((e: unknown) => e)) as ApiError;
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

  it("says so when the computer has no venue, and does not call it a credential failure", async () => {
    const { fetch } = fakeFetch(() => json(503, { error: { code: "no_venue", message: "no venue yet" } }));
    const seen: string[] = [];
    await createApiClient({ credentials: signedIn, fetch, onCredentialFailure: (c) => seen.push(c), onNoVenue: () => seen.push("no venue") })
      .get("/x")
      .catch(() => undefined);
    expect(seen).toEqual(["no venue"]);
  });

  it("does not report a 403, whatever its code", async () => {
    const { fetch } = fakeFetch(() => json(403, { error: { code: "unauthenticated", message: "no" } }));
    const seen: string[] = [];
    await createApiClient({ credentials: signedIn, fetch, onCredentialFailure: (c) => seen.push(c) })
      .get("/x")
      .catch(() => undefined);
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

describe("isUnreachable", () => {
  const cases: [string, unknown, boolean][] = [
    ["no network", new ApiError("network", "x"), true],
    ["a timeout", new ApiError("timeout", "x"), true],
    ["a relay with no venue behind it: 502", new ApiError("bad_response", "x", 502), true],
    ["503", new ApiError("internal", "x", 503), true],
    ["504", new ApiError("bad_response", "x", 504), true],
    ["a refusal", new ApiError("over_tender", "x", 409), false],
    ["a 500 is an answer", new ApiError("internal", "x", 500), false],
    ["the cause of a wrapped error", new Error("Not sent", { cause: new ApiError("network", "x") }), true],
    ["something that is not an API error", new Error("x"), false],
    ["not an error at all", "x", false],
  ];
  it.each(cases)("%s", (_name, err, want) => {
    expect(isUnreachable(err)).toBe(want);
  });
});

describe("apiErrorOf", () => {
  it("finds the server's error itself or as a cause", () => {
    const api = new ApiError("forbidden", "no", 403);
    expect(apiErrorOf(api)).toBe(api);
    expect(apiErrorOf(new Error("words", { cause: api }))).toBe(api);
    expect(apiErrorOf(new Error("words"))).toBeNull();
    expect(apiErrorOf(null)).toBeNull();
  });
});

describe("wordsOf", () => {
  const fallback = "Can't reach the mez server.";
  it("is the server's message when it answered", () => {
    expect(wordsOf(new ApiError("forbidden", "Not allowed", 403), fallback)).toBe("Not allowed");
  });
  it("is the fallback when it did not", () => {
    expect(wordsOf(new ApiError("network", "the server could not be reached"), fallback)).toBe(fallback);
    expect(wordsOf(new ApiError("bad_response", "bad gateway", 502), fallback)).toBe(fallback);
    expect(wordsOf(new Error("boom"), fallback)).toBe(fallback);
    expect(wordsOf(undefined, fallback)).toBe(fallback);
  });
});

describe("answers through the relay", () => {
  const relayed = (status: number, body: unknown, via: string | null) =>
    new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", ...(via === null ? {} : { "X-Mez-Via": via }) } });

  it("reads the mark off an answer, and only the relay's own word for it", () => {
    expect(cameThroughRelay(relayed(200, {}, "relay"))).toBe(true);
    expect(cameThroughRelay(relayed(200, {}, null))).toBe(false);
    expect(cameThroughRelay(relayed(200, {}, "lan"))).toBe(false);
    expect(cameThroughRelay(relayed(200, {}, ""))).toBe(false);
  });

  it("shows every answer to onResponse, a refusal as much as a success", async () => {
    const seen: boolean[] = [];
    const answers = [
      relayed(200, { id: "d1" }, "relay"),
      relayed(403, { error: { code: "remote_not_allowed", message: "This can only be done at the venue." } }, "relay"),
      relayed(200, {}, null),
    ];
    const { fetch } = fakeFetch(() => answers.shift() ?? relayed(500, {}, null));
    const api = createApiClient({ credentials: signedIn, fetch, onResponse: (r) => seen.push(cameThroughRelay(r)) });

    await api.get("/api/v1/devices/me");
    const refused = await api.post("/api/v1/orders", {}).catch((err: unknown) => err);
    await api.get("/api/v1/devices/me");

    expect(seen).toEqual([true, true, false]);
    expect(refused).toBeInstanceOf(ApiError);
    expect((refused as ApiError).code).toBe("remote_not_allowed");
    // The server's sentence is what a screen shows: it needs none of its own.
    expect(wordsOf(refused, "Could not save.")).toBe("This can only be done at the venue.");
  });

  it("is not shown an answer that never came", async () => {
    const seen: Response[] = [];
    const { fetch } = fakeFetch(() => Promise.reject(new TypeError("failed to fetch")));
    const api = createApiClient({ credentials: signedIn, fetch, onResponse: (r) => seen.push(r) });
    await expect(api.get("/api/v1/devices/me")).rejects.toMatchObject({ code: "network" });
    expect(seen).toEqual([]);
  });
});
