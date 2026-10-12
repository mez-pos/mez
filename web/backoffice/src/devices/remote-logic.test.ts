import { describe, expect, it } from "vitest";
import { remoteState, remoteWords } from "./remote-logic";

const url = "https://cafe.mezpos.com/backoffice/";

describe("remoteState", () => {
  it("is off unless the venue turned it on, whatever the relay is doing", () => {
    expect(remoteState({ state: "off", back_office: false })).toBe("off");
    expect(remoteState({ state: "connected", back_office: false })).toBe("off");
    expect(remoteState({ state: "refused", back_office: false })).toBe("off");
  });

  it("is on only while the relay is reached", () => {
    expect(remoteState({ state: "connected", back_office: true })).toBe("on");
    for (const state of ["connecting", "waiting", "refused", "not_entitled", "replaced", "stopped"] as const) {
      expect(remoteState({ state, back_office: true })).toBe("unreachable");
    }
  });
});

describe("a relay that does not carry the back office", () => {
  const on = { state: "connected", back_office: true, back_office_url: url } as const;

  it("is not reachable, though the venue has it on and the relay is reached", () => {
    expect(remoteState({ ...on, cloud: { back_office: false } })).toBe("unreachable");
    expect(remoteState({ ...on, cloud: { back_office: true } })).toBe("on");
    // A relay that says nothing is taken at the venue's word.
    expect(remoteState(on)).toBe("on");
  });

  it("says so, says guests are not affected, and lists nothing to do here", () => {
    const w = remoteWords({ ...on, cloud: { back_office: false } }, false);
    expect(w.line).toMatch(/the relay does not carry the back office for this venue/);
    expect(w.line).toMatch(/Guests' orders are not affected\./);
    expect(w.address).toBe(url);
    expect(w.steps).toEqual([]);
  });

  it("says nothing of it while the relay is not reached at all", () => {
    const w = remoteWords({ ...on, state: "waiting", cloud: { back_office: false } }, false);
    expect(w.line).toMatch(/the relay is not reached right now/);
  });
});

describe("remoteWords", () => {
  it("says a relay comes first when there is none", () => {
    const w = remoteWords({ state: "off", back_office: false }, false);
    expect(w.line).toMatch(/only at the venue/);
    expect(w.line).toMatch(/needs a relay first/);
    expect(w.address).toBeNull();
    expect(w.steps).toEqual([]);
  });

  it("says how to turn it on when there is a relay and it is off", () => {
    const w = remoteWords({ state: "connected", back_office: false }, false);
    expect(w.line).toMatch(/only at the venue/);
    expect(w.line).toMatch(/nothing that takes money/);
    expect(w.address).toBeNull();
    expect(w.steps).toHaveLength(3);
    expect(w.steps.join(" ")).toMatch(/remote_backoffice = true/);
    expect(w.steps.at(-1)).toBe("Restart mez.");
  });

  it("never shows an address while it is off, even if one is on the wire", () => {
    expect(remoteWords({ state: "connected", back_office: false, back_office_url: url }, false).address).toBeNull();
  });

  it("gives the address and the three things to do the first time", () => {
    const w = remoteWords({ state: "connected", back_office: true, back_office_url: url }, false);
    expect(w.line).toBe("The back office can be opened away from the venue, at this address.");
    expect(w.address).toBe(url);
    expect(w.steps).toHaveLength(3);
    expect(w.steps[0]).toMatch(/phone, tablet or computer/);
    expect(w.steps[1]).toMatch(/Pair a device/);
    expect(w.steps[1]).toMatch(/made at the venue/);
    expect(w.steps[2]).toMatch(/manages the venue/);
  });

  it("keeps the address, and says it opens nothing, while the relay is not reached", () => {
    const w = remoteWords({ state: "waiting", back_office: true, back_office_url: url }, false);
    expect(w.line).toMatch(/relay is not reached/);
    expect(w.line).toMatch(/counter is not affected/);
    expect(w.address).toBe(url);
  });

  it("says what is missing when it is on and there is no address to give", () => {
    const w = remoteWords({ state: "connected", back_office: true }, false);
    expect(w.line).toMatch(/has not said where it is reached/);
    expect(w.address).toBeNull();
    expect(w.steps.join(" ")).toMatch(/public_url/);
  });

  it("says so, and asks for nothing, when this page is the one being used from away", () => {
    const w = remoteWords({ state: "connected", back_office: true, back_office_url: url }, true);
    expect(w.line).toMatch(/You are using it now/);
    expect(w.address).toBe(url);
    expect(w.steps).toEqual([]);
  });

  it("has a sentence for every case", () => {
    for (const back_office of [true, false]) {
      for (const state of ["off", "connecting", "connected", "waiting", "refused", "replaced", "stopped"] as const) {
        for (const away of [true, false]) {
          expect(remoteWords({ state, back_office, back_office_url: url }, away).line.length).toBeGreaterThan(20);
        }
      }
    }
  });
});
