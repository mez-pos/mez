// A session mez Cloud hands the back office (ADR-355): kept for the tab,
// confirmed by the server, never in place of a pairing, and ended with the
// way back.

import { describe, expect, it } from "vitest";
import type { DeviceResponse } from "../api-types";
import { takeHandoff } from "./app-auth";
import { AuthStore } from "./auth-store";
import { credentialStore, type KeyValue } from "./storage";

class MapStorage implements KeyValue {
  readonly map = new Map<string, string>();
  getItem = (k: string) => this.map.get(k) ?? null;
  setItem = (k: string, v: string) => void this.map.set(k, v);
  removeItem = (k: string) => void this.map.delete(k);
}

type Route = (method: string, path: string, headers: Headers) => Response | Promise<Response>;

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
const refuse = (code: string) => json(401, { error: { code, message: code } });

const token = `nfs_${"C".repeat(43)}`;
const back = "https://app.mezpos.com/v/corner-cafe/";
const cloudDevice: DeviceResponse = { id: "dev_cloud", name: "mez Cloud", kind: "back_office" };
const laptop: DeviceResponse = { id: "dev_9", name: "Priya's laptop", kind: "back_office" };
const owner = { id: "stf_cloud", name: "Priya's account", role: "owner" };
const session = { staff: owner, permissions: ["reports.view"], expires_at: "2026-10-16T22:00:00Z" };

function setup(route: Route) {
  const local = new MapStorage();
  const tab = new MapStorage();
  const store = credentialStore("backoffice", local, tab, "terminal");
  const seen: { path: string; headers: Headers }[] = [];
  const fetchFn = (async (path: RequestInfo | URL, init?: RequestInit) => {
    const headers = new Headers(init?.headers);
    seen.push({ path: String(path), headers });
    return route(init?.method ?? "GET", String(path), headers);
  }) as typeof fetch;
  const auth = new AuthStore("backoffice", store, fetchFn);
  return { auth, store, local, tab, seen };
}

const venue: Route = (method, path) => {
  if (path === "/api/v1/devices/me") return json(200, cloudDevice);
  if (path === "/api/v1/sessions/current" && method === "GET") return json(200, session);
  if (path === "/api/v1/sessions/current" && method === "DELETE") return new Response(null, { status: 204 });
  return refuse("unknown");
};

describe("a session mez Cloud handed", () => {
  it("is presented as both, confirmed, and signs the owner in", async () => {
    const { auth, store, seen } = setup(venue);
    store.takeCloud({ token, back });
    await auth.boot();
    expect(auth.current()).toEqual({ kind: "signed_in", device: cloudDevice, staff: owner, permissions: ["reports.view"], expiresAt: session.expires_at });
    for (const { headers } of seen) {
      expect(headers.get("Authorization")).toBe(`Bearer ${token}`);
      expect(headers.get("X-Staff-Token")).toBe(token);
    }
    expect(store.device()).toEqual({ token, device: cloudDevice });
    expect(store.staff()).toMatchObject({ token, staff: owner });
  });

  it("is the tab's alone, and never takes the place of the browser's own pairing", async () => {
    const { auth, store, local } = setup(venue);
    store.saveDevice({ token: "nfd_laptop", device: laptop });
    const pairing = local.map.get("mez.backoffice.device");
    store.takeCloud({ token, back });
    await auth.boot();
    expect(auth.current().kind).toBe("signed_in");
    expect(local.map.get("mez.backoffice.device")).toBe(pairing);

    await auth.signOut();
    expect(store.cloud()).toBeNull();
    expect(store.device()).toEqual({ token: "nfd_laptop", device: laptop });
  });

  it("ends when the server never takes it, with no way back to offer", async () => {
    const { auth, store } = setup(() => refuse("device_unrecognised"));
    store.takeCloud({ token, back });
    await auth.boot();
    expect(auth.current()).toEqual({ kind: "unpaired", notice: "cloud_ended", back: null });
    expect(store.cloud()).toBeNull();
  });

  it("ends when a later request is refused, as a session that expired would", async () => {
    const { auth, store } = setup(venue);
    store.takeCloud({ token, back });
    await auth.boot();
    auth.credentialFailed("sign_in_required");
    expect(auth.current()).toEqual({ kind: "unpaired", notice: "cloud_ended", back });
    expect(store.device()).toBeNull();
  });

  it("is not trusted unconfirmed when the server cannot be reached", async () => {
    const { auth, store } = setup(() => Promise.reject(new TypeError("Failed to fetch")));
    store.takeCloud({ token, back: null });
    await auth.boot();
    expect(auth.current()).toEqual({ kind: "unpaired", notice: "cloud_ended", back: null });
  });

  it("signed out of, says so, ends on the server, and has no lock screen", async () => {
    const { auth, store, seen } = setup(venue);
    store.takeCloud({ token, back });
    await auth.boot();
    await auth.signOut();
    expect(seen.some(({ path }) => path === "/api/v1/sessions/current")).toBe(true);
    expect(auth.current()).toEqual({ kind: "unpaired", notice: "cloud_signed_out", back });
    expect(store.cloud()).toBeNull();
  });

  it("ended, leaves the pairing alone whatever else is refused after", async () => {
    const { auth, store, local } = setup(venue);
    store.saveDevice({ token: "nfd_laptop", device: laptop });
    const pairing = local.map.get("mez.backoffice.device");
    store.takeCloud({ token, back });
    await auth.boot();
    // The session ends, and a request and the stream still in flight say so too.
    auth.credentialFailed("sign_in_required");
    auth.credentialFailed("unauthenticated");
    auth.credentialFailed("device_revoked");
    expect(auth.current()).toEqual({ kind: "unpaired", notice: "cloud_ended", back });
    expect(local.map.get("mez.backoffice.device")).toBe(pairing);
    // Nothing falls back to the pairing in this tab until it pairs or reloads.
    expect(auth.credentials.deviceToken()).toBeNull();
    expect(auth.credentials.staffToken()).toBeNull();
  });

  it("offers no way back for a token the server never confirmed", async () => {
    const { auth, store } = setup(() => refuse("unauthenticated"));
    store.takeCloud({ token, back: "https://evil.mezpos.com/login" });
    await auth.boot();
    expect(auth.current()).toEqual({ kind: "unpaired", notice: "cloud_ended", back: null });
  });

  it("signed out with the session already gone, keeps its first word and the way back", async () => {
    const { auth, store } = setup((method, path, headers) => (method === "DELETE" ? refuse("unauthenticated") : venue(method, path, headers)));
    store.takeCloud({ token, back });
    await auth.boot();
    await auth.signOut();
    expect(auth.current()).toEqual({ kind: "unpaired", notice: "cloud_ended", back });
  });

  it("ends when this tab pairs with a code", async () => {
    const { store } = setup(venue);
    store.takeCloud({ token, back });
    store.saveDevice({ token: "nfd_laptop", device: laptop });
    expect(store.cloud()).toBeNull();
    expect(store.device()?.device).toEqual(laptop);
  });
});

describe("takeHandoff", () => {
  // A location and a history as a browser's are: one live object, whose hash
  // is gone once the address is replaced.
  let live = { hash: "", pathname: "/backoffice/reports", search: "?day=today", protocol: "https:", hostname: "corner-cafe.mezpos.com" };
  const at = (hash: string) => {
    live = { ...live, hash };
    return live;
  };
  const history = () => {
    const replaced: string[] = [];
    const replaceState = (_s: unknown, _t: string, url?: string | URL | null) => {
      replaced.push(String(url));
      live.hash = "";
    };
    return { replaced, h: { state: null, replaceState } };
  };

  it("keeps the session and takes it out of the address", () => {
    const { replaced, h } = history();
    const kept: unknown[] = [];
    const took = takeHandoff(at(`#cloud=${token}&back=${encodeURIComponent(back)}`), h, { takeCloud: (v) => void kept.push(v) });
    expect(took).toBe(true);
    expect(kept).toEqual([{ token, back }]);
    expect(replaced).toEqual(["/backoffice/reports?day=today"]);
  });

  it("takes a token that is not one out of the address, and keeps nothing", () => {
    const { replaced, h } = history();
    const kept: unknown[] = [];
    expect(takeHandoff(at("#cloud=not-a-token"), h, { takeCloud: (v) => void kept.push(v) })).toBe(false);
    expect(kept).toEqual([]);
    expect(replaced).toEqual(["/backoffice/reports?day=today"]);
  });

  it("leaves any other fragment alone", () => {
    const { replaced, h } = history();
    expect(takeHandoff(at("#section-2"), h, { takeCloud: () => undefined })).toBe(false);
    expect(replaced).toEqual([]);
  });
});
