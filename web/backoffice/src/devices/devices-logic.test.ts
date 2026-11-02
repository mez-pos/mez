import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { DeviceRecordResponse, PairingPlaceResponse } from "../../../shared/api-types";
import {
  MAX_DEVICE_NAME,
  PAIRABLE,
  actionProblemOf,
  appOffer,
  deviceActions,
  kindText,
  revokeWords,
  revokedToast,
  dateText,
  devicesSummary,
  isStale,
  kindWords,
  listed,
  lastSeenText,
  nameProblem,
  pairProblemOf,
  pairingPlaces,
  typedAddress,
  revokedText,
  runsOutText,
  split,
  stateOf,
} from "./devices-logic";

const now = new Date("2026-10-03T12:00:00Z");
const ago = (ms: number) => new Date(now.getTime() - ms).toISOString();
const MIN = 60_000;
const HOUR = 60 * MIN;

const device = (patch: Partial<DeviceRecordResponse> = {}): DeviceRecordResponse => ({
  id: "dev_AAAA0001",
  name: "Counter 1",
  kind: "terminal",
  paired_at: "2026-09-28T09:00:00Z",
  this: false,
  cloud: false,
  ...patch,
});

describe("a device's kind", () => {
  it("is said in the words staff use", () => {
    expect(kindWords.terminal).toBe("Terminal");
    expect(kindWords.kitchen).toBe("Kitchen display");
    expect(kindWords.back_office).toBe("Back office");
  });
  it("is offered to pair for the three that can be", () => {
    expect(PAIRABLE.map((k) => k.value)).toEqual(["terminal", "kitchen", "back_office"]);
    expect(PAIRABLE.map((k) => k.label)).toEqual(["Terminal", "Kitchen display", "Back office"]);
  });
});

describe("when a device was last heard from", () => {
  it("is in minutes, then hours, then a date", () => {
    expect(lastSeenText(device({ last_seen_at: ago(20_000) }), now, "Asia/Kolkata")).toBe("Seen just now");
    expect(lastSeenText(device({ last_seen_at: ago(12 * MIN) }), now, "Asia/Kolkata")).toBe("Seen 12 min ago");
    expect(lastSeenText(device({ last_seen_at: ago(3 * HOUR) }), now, "Asia/Kolkata")).toBe("Seen 3 h ago");
    expect(lastSeenText(device({ last_seen_at: ago(40 * HOUR) }), now, "Asia/Kolkata")).toBe("Seen 2 Oct 2026");
    expect(lastSeenText(device({ last_seen_at: ago(24 * HOUR) }), now, "Asia/Kolkata")).toBe("Seen 2 Oct 2026");
    expect(lastSeenText(device({ last_seen_at: ago(23 * HOUR) }), now, "Asia/Kolkata")).toBe("Seen 23 h ago");
  });
  it("says so when it never was", () => {
    expect(lastSeenText(device(), now, "Asia/Kolkata")).toBe("Not heard from since it was paired");
  });
  it("is stale at a day, and not before, and never for one not yet heard from", () => {
    expect(isStale(device({ last_seen_at: ago(23 * HOUR) }), now)).toBe(false);
    expect(isStale(device({ last_seen_at: ago(24 * HOUR) }), now)).toBe(true);
    expect(isStale(device({ last_seen_at: ago(5 * 24 * HOUR) }), now)).toBe(true);
    expect(isStale(device(), now)).toBe(false);
  });
});

describe("dates", () => {
  it("are written as people write them", () => {
    expect(dateText("2026-10-03T12:00:00Z", "Asia/Kolkata")).toBe("3 Oct 2026");
    expect(dateText("2026-01-05T12:00:00Z", "Asia/Kolkata")).toBe("5 Jan 2026");
    expect(dateText("2026-12-31T12:00:00Z", "Asia/Kolkata")).toBe("31 Dec 2026");
    expect(revokedText(device({ revoked_at: "2026-10-02T12:00:00Z" }), "Asia/Kolkata")).toBe("Revoked 2 Oct 2026");
    expect(revokedText(device(), "Asia/Kolkata")).toBe("Revoked");
  });
});

describe("who is in service", () => {
  it("is every device that is not revoked, and the revoked are apart, each in the server's order", () => {
    const a = device({ id: "a" });
    const b = device({ id: "b", revoked_at: "2026-10-01T00:00:00Z" });
    const c = device({ id: "c" });
    const d = device({ id: "d", revoked_at: "2026-10-02T00:00:00Z" });
    const { inService, revoked } = split([a, c, b, d]);
    expect(inService.map((x) => x.id)).toEqual(["a", "c"]);
    expect(revoked.map((x) => x.id)).toEqual(["b", "d"]);
    expect(split([])).toEqual({ inService: [], revoked: [] });
  });
});

describe("when a pairing code runs out", () => {
  it("is said in minutes, rounded up, or that it has", () => {
    const at = (ms: number) => new Date(now.getTime() + ms).toISOString();
    expect(runsOutText(at(7 * MIN), now)).toBe("runs out in 7 min");
    expect(runsOutText(at(6 * MIN + 1), now)).toBe("runs out in 7 min");
    expect(runsOutText(at(MIN), now)).toBe("runs out in 1 min");
    expect(runsOutText(at(30_000), now)).toBe("runs out in under a minute");
    expect(runsOutText(at(0), now)).toBe("has run out");
    expect(runsOutText(at(-MIN), now)).toBe("has run out");
  });
});

describe("a name", () => {
  it("is refused when empty or too long, counting characters", () => {
    expect(nameProblem("")?.field).toBe("name");
    expect(nameProblem("   ")?.field).toBe("name");
    expect(nameProblem("a".repeat(MAX_DEVICE_NAME + 1))?.field).toBe("name");
    expect(nameProblem("a".repeat(MAX_DEVICE_NAME))).toBeNull();
    expect(nameProblem("स".repeat(MAX_DEVICE_NAME))).toBeNull(); // 64 characters, 192 bytes
    expect(nameProblem("Counter 1")).toBeNull();
  });
});

describe("a failed request", () => {
  it("puts the server's words under the name or the kind", () => {
    expect(pairProblemOf(new ApiError("invalid_request", "a name of 1 to 64 characters is needed", 400, "name"))).toEqual({
      field: "name",
      message: "a name of 1 to 64 characters is needed",
    });
    expect(pairProblemOf(new ApiError("invalid_request", "kind is not a device that can be paired", 400, "kind")).field).toBe("kind");
    expect(pairProblemOf(new ApiError("invalid_request", "x", 400, "other")).field).toBeNull();
    expect(pairProblemOf(new ApiError("forbidden", "x", 403)).message).toMatch(/change the venue's settings/);
  });
  it("is the server's words for a line under the heading", () => {
    expect(actionProblemOf(new ApiError("cannot_revoke_self", "You are using this device.", 409))).toBe("You are using this device.");
    expect(actionProblemOf(new ApiError("device_unpaired", "That device has been revoked.", 409))).toBe("That device has been revoked.");
    expect(actionProblemOf(new ApiError("network", "down"))).toMatch(/Can't reach/);
  });
});

describe("where a device stands", () => {
  it("is in service when heard from within a day, or not yet heard from", () => {
    expect(stateOf(device({ last_seen_at: ago(23 * HOUR) }), now)).toBe("in_service");
    expect(stateOf(device(), now)).toBe("in_service");
  });
  it("is quiet once a day has passed without a word", () => {
    expect(stateOf(device({ last_seen_at: ago(24 * HOUR) }), now)).toBe("quiet");
  });
  it("is revoked whatever it was last heard", () => {
    expect(stateOf(device({ last_seen_at: ago(40 * HOUR), revoked_at: ago(HOUR) }), now)).toBe("revoked");
    expect(stateOf(device({ last_seen_at: ago(MIN), revoked_at: ago(20_000) }), now)).toBe("revoked");
  });
});

describe("the list as the table shows it", () => {
  it("has the devices in service first, then the revoked", () => {
    const rows = listed([device({ id: "a", revoked_at: ago(HOUR) }), device({ id: "b" }), device({ id: "c", revoked_at: ago(HOUR) }), device({ id: "d" })]);
    expect(rows.map((d) => d.id)).toEqual(["b", "d", "a", "c"]);
  });
});

describe("the line under the title", () => {
  it("counts the devices in service, the quiet among them, and the revoked", () => {
    expect(devicesSummary([device(), device({ last_seen_at: ago(MIN) })], now)).toBe("2 in service");
    expect(devicesSummary([device(), device({ last_seen_at: ago(30 * HOUR) }), device({ revoked_at: ago(HOUR) })], now)).toBe(
      "2 in service, 1 quiet for a day or more, 1 revoked",
    );
    expect(devicesSummary([], now)).toBe("0 in service");
  });
});

describe("the device mez Cloud signs the owner in on (ADR-323)", () => {
  const cloud = device({ id: "dev_CLOUD001", name: "mez Cloud", kind: "back_office", cloud: true });

  it("is not called a back office, which would send an owner looking for a laptop", () => {
    expect(kindText(cloud)).toBe("Your mez Cloud account's way in. Nobody holds it");
    expect(kindText(device({ kind: "back_office" }))).toBe("Back office");
    expect(kindText(device())).toBe("Terminal");
  });

  it("can be revoked and not renamed", () => {
    expect(deviceActions(cloud, false)).toEqual(["revoke"]);
    expect(deviceActions(cloud, true)).toEqual(["revoke"]);
    // Opened from mez Cloud, it is the device in use: nothing is offered.
    expect(deviceActions({ ...cloud, this: true }, true)).toEqual([]);
    expect(deviceActions({ ...cloud, revoked_at: ago(HOUR) }, false)).toEqual([]);
  });

  it("offers nothing on a mirror, which changes nothing", () => {
    expect(deviceActions(cloud, true, true)).toEqual([]);
    expect(deviceActions(device(), true, true)).toEqual([]);
    expect(deviceActions(device(), false, true)).toEqual([]);
  });

  it("leaves every other device's menu as it was", () => {
    expect(deviceActions(device(), false)).toEqual(["rename", "revoke"]);
    expect(deviceActions(device(), true)).toEqual(["revoke"]);
    expect(deviceActions(device({ this: true }), false)).toEqual(["rename"]);
    expect(deviceActions(device({ this: true }), true)).toEqual([]);
    expect(deviceActions(device({ revoked_at: ago(HOUR) }), false)).toEqual([]);
  });

  it("says what revoking it shuts, and what it does not", () => {
    const words = revokeWords(cloud);
    expect(words.confirm).toBe("Shut mez Cloud's sign-in");
    expect(words.body).toContain("signed out at once");
    expect(words.body).toContain("Guests' orders and the backup carry on");
    expect(words.body).toContain("Pairing to mez Cloud again");
    expect(revokeWords(device())).toEqual({
      body: "It stops working at once and shows the pairing screen. Orders it took stay as they are. To use it again it needs a new code.",
      confirm: "Revoke device",
    });
    expect(revokedToast(cloud)).toBe("mez Cloud's sign-in is shut.");
    expect(revokedToast(device())).toBe("Counter 1 revoked.");
  });
});

describe("pairingPlaces", () => {
  const wifi = { adapter: "Wi-Fi", virtual: false, url: "http://192.168.1.10:7777", svg: "<svg/>" };
  const wsl = { adapter: "vEthernet (WSL)", virtual: true, url: "http://172.25.224.1:7777", svg: "<svg/>" };

  it("shows the real adapters large and the virtual ones small", () => {
    expect(pairingPlaces([wsl, wifi])).toEqual({ main: [wifi], others: [wsl] });
  });

  it("shows the virtual ones large when they are all there is", () => {
    expect(pairingPlaces([wsl])).toEqual({ main: [wsl], others: [] });
  });

  it("has nothing to show for a code that came with no address", () => {
    expect(pairingPlaces([])).toEqual({ main: [], others: [] });
    expect(pairingPlaces(undefined)).toEqual({ main: [], others: [] });
  });
});

describe("typedAddress", () => {
  it("drops the scheme and keeps the rest", () => {
    expect(typedAddress("http://192.168.1.10:7777/kitchen")).toBe("192.168.1.10:7777/kitchen");
    expect(typedAddress("https://mez.example/backoffice/")).toBe("mez.example/backoffice/");
  });
});

describe("the tablet app beside a pairing code (ADR-421)", () => {
  const place = (patch: Partial<PairingPlaceResponse> = {}): PairingPlaceResponse => ({
    adapter: "Wi-Fi",
    virtual: false,
    url: "http://192.168.1.10:7777",
    svg: "<svg>pair</svg>",
    ...patch,
  });
  const withApp = place({ app_url: "http://192.168.1.10:7777/app/mez.apk", app_svg: "<svg>app</svg>" });

  it("is offered as a QR for a till or a kitchen display, where this build carries it", () => {
    expect(appOffer("terminal", "v0.2.0", [withApp])).toBe("offered");
    expect(appOffer("kitchen", "v0.2.0", [place(), withApp])).toBe("offered");
  });

  it("is named in words when this computer's address is not known", () => {
    expect(appOffer("terminal", "v0.2.0", [])).toBe("typed");
    expect(appOffer("kitchen", "v0.2.0", [place()])).toBe("typed");
    expect(appOffer("terminal", "v0.2.0", [place({ app_url: "http://192.168.1.10:7777/app/mez.apk" })])).toBe("typed");
  });

  it("is said to be missing from a build without it", () => {
    expect(appOffer("terminal", "", [place()])).toBe("not_built");
    expect(appOffer("kitchen", undefined, [])).toBe("not_built");
  });

  it("is not offered to a back office, which is a browser", () => {
    expect(appOffer("back_office", "v0.2.0", [withApp])).toBe("none");
    expect(appOffer("back_office", "", [])).toBe("none");
    expect(appOffer("back_office", undefined, [])).toBe("none");
  });
});
