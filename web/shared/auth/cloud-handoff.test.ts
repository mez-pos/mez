import { describe, expect, it } from "vitest";
import { readHandoff, safeBack } from "./cloud-handoff";

const token = `nfs_${"A".repeat(42)}w`;
const venue = { protocol: "https:", hostname: "corner-cafe.mezpos.com" };
const local = { protocol: "http:", hostname: "localhost" };

describe("readHandoff", () => {
  it("takes a session and the way back", () => {
    const back = encodeURIComponent("https://app.mezpos.com/v/corner-cafe/");
    expect(readHandoff(`#cloud=${token}&back=${back}`, venue)).toEqual({ token, back: "https://app.mezpos.com/v/corner-cafe/" });
  });

  it("takes a mirror's session too", () => {
    const mirror = `nfms_${"b".repeat(43)}`;
    expect(readHandoff(`#cloud=${mirror}`, venue)).toEqual({ token: mirror, back: null });
  });

  it.each([
    ["nothing", ""],
    ["another fragment", "#/menu"],
    ["no token", "#cloud="],
    ["a token too short", `#cloud=nfs_abc`],
    ["a token with no prefix", `#cloud=${"A".repeat(43)}`],
    ["a token with a stray character", `#cloud=${token.slice(0, -1)}!`],
  ])("refuses %s", (_name, hash) => {
    expect(readHandoff(hash, venue)).toBeNull();
  });

  it("keeps the session and drops a way back that is not one", () => {
    expect(readHandoff(`#cloud=${token}&back=${encodeURIComponent("https://evil.example/")}`, venue)).toEqual({ token, back: null });
  });
});

describe("safeBack", () => {
  it.each([
    ["the account area", "https://app.mezpos.com/v/corner-cafe/", venue, "https://app.mezpos.com/v/corner-cafe/"],
    ["the parent domain itself", "https://mezpos.com/", venue, "https://mezpos.com/"],
    ["another site", "https://evil.example/", venue, null],
    ["a lookalike", "https://app.mezpos.com.evil.example/", venue, null],
    ["a suffix that is not a label", "https://notmezpos.com/", venue, null],
    ["plain http from https", "http://app.mezpos.com/", venue, null],
    ["a script", "javascript:alert(1)", venue, null],
    ["credentials in it", "https://user:pw@app.mezpos.com/", venue, null],
    ["not an address", "app.mezpos.com", venue, null],
    ["nothing", null, venue, null],
    ["an address by number", "https://203.0.113.9/", venue, null],
    ["from a page with no venue label", "https://app.mezpos.com/", { protocol: "https:", hostname: "mezpos.com" }, null],
    ["a self-hoster's domain under a shared suffix", "https://app.pos.co.in/", { protocol: "https:", hostname: "corner.pos.co.in" }, "https://app.pos.co.in/"],
    ["another site under that suffix", "https://evil.co.in/", { protocol: "https:", hostname: "corner.pos.co.in" }, null],
    ["loopback on a developer's machine", "http://127.0.0.1:5180/v/corner-cafe/", local, "http://127.0.0.1:5180/v/corner-cafe/"],
    ["the world from a developer's machine", "https://app.mezpos.com/", local, null],
    [
      "a developer's venue under localhost, back to the account area on 127.0.0.1",
      "http://127.0.0.1:8088/v/blue-tokai",
      { protocol: "http:", hostname: "blue-tokai.mez.localhost" },
      "http://127.0.0.1:8088/v/blue-tokai",
    ],
    ["the world from a venue under localhost", "https://evil.example/", { protocol: "http:", hostname: "blue-tokai.mez.localhost" }, null],
    ["a name that only ends like localhost", "http://evil-localhost.example/", { protocol: "http:", hostname: "blue-tokai.mez.localhost" }, null],
  ])("%s", (_name, raw, here, want) => {
    expect(safeBack(raw, here)).toBe(want);
  });
});
