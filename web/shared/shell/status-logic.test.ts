import { describe, expect, it } from "vitest";
import { connectionText, venueSilent } from "./status-logic";

describe("connectionText", () => {
  it("says Online while the stream is live, in the success tone", () => {
    expect(connectionText({ state: "live", since: 0 }, 0)).toMatchObject({ word: "Online", tone: "success", retryable: false });
  });
  it("says what the connection indicator says when it is not", () => {
    expect(connectionText({ state: "reconnecting", reason: "unreachable", retryAt: 5_000, attempt: 2 }, 0)).toMatchObject({
      word: "Server unreachable",
      tone: "danger",
      retryable: true,
    });
    expect(connectionText({ state: "syncing", target: 10 }, 0).word).toBe("Syncing");
  });

  describe("on a mirror", () => {
    const fresh = { asOf: 1_000_000, heardAt: 1_030_000 };
    const stale = { asOf: 1_000_000, heardAt: 1_000_000 + 2 * 60_000 + 1 };

    it("says Venue offline, as a warning with nothing to retry, once the copy has stopped growing newer", () => {
      expect(connectionText({ state: "away" }, 0, stale)).toEqual({ tone: "warning", word: "Venue offline", detail: null, retryable: false });
      expect(venueSilent({ state: "away" }, stale)).toBe(true);
    });
    it("says Away from the venue while the venue is still sending", () => {
      expect(connectionText({ state: "away" }, 0, fresh)).toMatchObject({ tone: "muted", word: "Away from the venue" });
      expect(venueSilent({ state: "away" }, fresh)).toBe(false);
    });
    it("says it of a copy that cannot say how old it is", () => {
      expect(connectionText({ state: "away" }, 0, { asOf: null, heardAt: 5 }).word).toBe("Venue offline");
    });
    it("leaves the connection its own words until the page has settled as away", () => {
      expect(connectionText({ state: "connecting", attempt: 1 }, 0, stale).word).toBe("Connecting");
      expect(connectionText({ state: "reconnecting", reason: "unreachable", retryAt: 5_000, attempt: 2 }, 0, stale).word).toBe("Server unreachable");
      expect(venueSilent({ state: "connecting", attempt: 1 }, stale)).toBe(false);
    });
    it("says nothing of a venue when the page is away and not on a mirror", () => {
      expect(connectionText({ state: "away" }, 0, null).word).toBe("Away from the venue");
      expect(connectionText({ state: "away" }, 0).word).toBe("Away from the venue");
    });
  });
});
