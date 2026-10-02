import { describe, expect, it } from "vitest";
import type { DeviceResponse, Permission } from "../api-types";
import { AuthStore, WrongDeviceKind } from "./auth-store";
import { credentialStore, type KeyValue } from "./storage";

class MapStorage implements KeyValue {
  readonly map = new Map<string, string>();
  getItem = (k: string) => this.map.get(k) ?? null;
  setItem = (k: string, v: string) => void this.map.set(k, v);
  removeItem = (k: string) => void this.map.delete(k);
}

type Route = (method: string, path: string, init: RequestInit) => Response | Promise<Response>;

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", ...headers } });
const refuse = (code: string) => json(401, { error: { code, message: code } });

const till: DeviceResponse = { id: "dev_1", name: "Counter 1", kind: "terminal" };
const asha = { id: "stf_1", name: "Asha" };

function setup(route: Route, seed: { device?: boolean; staff?: boolean; permissions?: readonly Permission[] } = {}) {
  const local = new MapStorage();
  const session = new MapStorage();
  const store = credentialStore("terminal", local, session);
  if (seed.device) store.saveDevice({ token: "nfd_old", device: { ...till, name: "Old name" } });
  if (seed.staff) {
    const stored = { token: "nfs_old", staff: asha, expiresAt: "2026-09-21T20:00:00Z" };
    store.saveStaff(seed.permissions ? { ...stored, permissions: seed.permissions } : stored);
  }
  const calls: string[] = [];
  const fetchFn = (async (path: RequestInfo | URL, init?: RequestInit) => {
    const method = init?.method ?? "GET";
    calls.push(`${method} ${String(path)}`);
    return route(method, String(path), init ?? {});
  }) as typeof fetch;
  const auth = new AuthStore("terminal", store, fetchFn);
  return { auth, store, local, session, calls };
}

const unreachable: Route = () => Promise.reject(new TypeError("Failed to fetch"));

describe("AuthStore.boot", () => {
  it("is unpaired with nothing stored, and asks the server nothing", async () => {
    const { auth, calls } = setup(unreachable);
    await auth.boot();
    expect(auth.current()).toEqual({ kind: "unpaired", notice: null });
    expect(calls).toEqual([]);
  });

  it("is locked with a device the server still knows, and takes its current name", async () => {
    const { auth, store } = setup((_, path) => (path === "/api/v1/devices/me" ? json(200, till) : refuse("x")), { device: true });
    await auth.boot();
    expect(auth.current()).toEqual({ kind: "locked", device: till, notice: null });
    expect(store.device()?.device.name).toBe("Counter 1");
  });

  it("is signed in when the stored session still stands", async () => {
    const { auth } = setup(
      (_, path) =>
        path === "/api/v1/devices/me" ? json(200, till) : json(200, { staff: asha, permissions: ["order.open"], expires_at: "2026-09-21T21:00:00Z" }),
      { device: true, staff: true },
    );
    await auth.boot();
    expect(auth.current()).toEqual({ kind: "signed_in", device: till, staff: asha, permissions: ["order.open"], expiresAt: "2026-09-21T21:00:00Z" });
  });

  it("takes the server's permissions over the stored ones, and stores them", async () => {
    const { auth, store } = setup(
      (_, path) => (path === "/api/v1/devices/me" ? json(200, till) : json(200, { staff: asha, permissions: [], expires_at: "x" })),
      { device: true, staff: true, permissions: ["reports.view"] },
    );
    await auth.boot();
    expect(auth.current()).toMatchObject({ kind: "signed_in", permissions: [] });
    expect(store.staff()?.permissions).toEqual([]);
  });

  it("reads a session stored before sessions carried permissions as holding none", async () => {
    const { auth, store } = setup(unreachable, { device: true, staff: true });
    expect(store.staff()?.permissions).toBeUndefined();
    await auth.boot();
    expect(auth.current()).toMatchObject({ kind: "signed_in", permissions: [] });
  });

  it("is locked, saying so, when the stored session has expired", async () => {
    const { auth, store } = setup((_, path) => (path === "/api/v1/devices/me" ? json(200, till) : refuse("sign_in_required")), {
      device: true,
      staff: true,
    });
    await auth.boot();
    expect(auth.current()).toEqual({ kind: "locked", device: till, notice: "expired" });
    expect(store.staff()).toBeNull();
  });

  it("is unpaired, saying why, when the device was revoked, and forgets everything", async () => {
    const { auth, store } = setup(() => refuse("device_revoked"), { device: true, staff: true });
    await auth.boot();
    expect(auth.current()).toEqual({ kind: "unpaired", notice: "revoked" });
    expect([store.device(), store.staff()]).toEqual([null, null]);
  });

  it("boots once, so a second call cannot erase why it was unpaired", async () => {
    const { auth, calls } = setup(() => refuse("unauthenticated"), { device: true });
    await Promise.all([auth.boot(), auth.boot()]);
    await auth.boot();
    expect(auth.current()).toEqual({ kind: "unpaired", notice: "unrecognised" });
    expect(calls).toEqual(["GET /api/v1/devices/me"]);
  });

  it("trusts what is stored when the server cannot be reached", async () => {
    const { auth } = setup(unreachable, { device: true, staff: true, permissions: ["reports.view"] });
    await auth.boot();
    expect(auth.current()).toMatchObject({ kind: "signed_in", staff: asha, permissions: ["reports.view"], device: { name: "Old name" } });
  });
});

describe("AuthStore.pair", () => {
  it("stores the device and locks, ready for a PIN", async () => {
    const { auth, store, calls } = setup(() => json(201, { device: till, token: "nfd_new" }));
    await auth.pair("MW4C-HHWY");
    expect(auth.current()).toEqual({ kind: "locked", device: till, notice: null });
    expect(store.device()).toEqual({ token: "nfd_new", device: till });
    expect(calls).toEqual(["POST /api/v1/devices/pair"]);
  });

  it("refuses a code made for another kind of device, and stores nothing", async () => {
    const { auth, store } = setup(() => json(201, { device: { ...till, kind: "back_office" }, token: "nfd_new" }));
    await auth.boot();
    const err = await auth.pair("MW4C-HHWY").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(WrongDeviceKind);
    expect(store.device()).toBeNull();
    expect(auth.current().kind).toBe("unpaired");
  });

  it("passes a bad code through for the screen to explain", async () => {
    const { auth } = setup(() => refuse("invalid_pairing_code"));
    await auth.boot();
    await expect(auth.pair("AAAA-AAAA")).rejects.toMatchObject({ code: "invalid_pairing_code" });
    expect(auth.current()).toEqual({ kind: "unpaired", notice: null });
  });
});

describe("AuthStore sign-in and sign-out", () => {
  const paired = (route: Route) => {
    const s = setup((m, p, i) => (p === "/api/v1/devices/me" ? json(200, till) : route(m, p, i)), { device: true });
    return s;
  };

  it("signs in with a PIN, sending the device token and the staff member", async () => {
    let sent: unknown = null;
    const { auth, store } = paired((_, __, init) => {
      sent = JSON.parse(String(init.body));
      return json(201, { staff: asha, permissions: ["menu.edit"], expires_at: "2026-09-21T21:00:00Z", token: "nfs_new" });
    });
    await auth.boot();
    await auth.signIn("stf_1", "2468");
    expect(sent).toEqual({ staff_id: "stf_1", pin: "2468" });
    expect(auth.current()).toMatchObject({ kind: "signed_in", staff: asha, permissions: ["menu.edit"] });
    expect(store.staff()).toMatchObject({ token: "nfs_new", permissions: ["menu.edit"] });
  });

  it("stays locked on a wrong PIN, and does not treat it as a lost device", async () => {
    const { auth, store } = paired(() => refuse("invalid_pin"));
    await auth.boot();
    await expect(auth.signIn("stf_1", "0000")).rejects.toMatchObject({ code: "invalid_pin" });
    expect(auth.current().kind).toBe("locked");
    expect(store.device()).not.toBeNull();
  });

  it("carries the wait on a lockout", async () => {
    const { auth } = paired(() => json(429, { error: { code: "locked_out", message: "locked" } }, { "Retry-After": "300" }));
    await auth.boot();
    await expect(auth.signIn("stf_1", "0000")).rejects.toMatchObject({ code: "locked_out", retryAfterMs: 300_000 });
  });

  it("locks on sign-out even when the server cannot be told", async () => {
    const { auth, store } = setup(
      (m, p) => (p === "/api/v1/devices/me" ? json(200, till) : p === "/api/v1/sessions/current" && m === "GET" ? json(200, { staff: asha, expires_at: "x" }) : Promise.reject(new TypeError("offline"))),
      { device: true, staff: true },
    );
    await auth.boot();
    await auth.signOut();
    expect(auth.current()).toEqual({ kind: "locked", device: till, notice: "signed_out" });
    expect(store.staff()).toBeNull();
  });

  it("locks, saying so, when any later request finds the session expired", async () => {
    const { auth } = setup(
      (m, p) => (p === "/api/v1/devices/me" ? json(200, till) : p === "/api/v1/sessions/current" && m === "GET" ? json(200, { staff: asha, expires_at: "x" }) : refuse("sign_in_required")),
      { device: true, staff: true },
    );
    await auth.boot();
    await auth.api.get("/api/v1/orders/ord_1").catch(() => undefined);
    expect(auth.current()).toEqual({ kind: "locked", device: till, notice: "expired" });
  });

  it("notifies subscribers on every change", async () => {
    const { auth } = setup(unreachable);
    const seen: string[] = [];
    auth.subscribe(() => seen.push(auth.current().kind));
    await auth.boot();
    auth.credentialFailed("device_revoked");
    expect(seen).toEqual(["unpaired", "unpaired"]);
  });
});

describe("credentialStore", () => {
  it("keeps apps apart on one origin", () => {
    const local = new MapStorage();
    const session = new MapStorage();
    credentialStore("terminal", local, session).saveDevice({ token: "t", device: till });
    expect(credentialStore("backoffice", local, session).device()).toBeNull();
  });

  it("treats a malformed entry as absent", () => {
    const local = new MapStorage();
    local.setItem("mez.terminal.device", "{not json");
    expect(credentialStore("terminal", local, null).device()).toBeNull();
    local.setItem("mez.terminal.device", JSON.stringify({ token: 5 }));
    expect(credentialStore("terminal", local, null).device()).toBeNull();
  });

  it("works with no storage at all", () => {
    const store = credentialStore("terminal", null, null);
    store.saveDevice({ token: "t", device: till });
    expect(store.device()).toBeNull();
  });
});
