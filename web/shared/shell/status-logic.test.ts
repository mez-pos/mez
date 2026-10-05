import { describe, expect, it } from "vitest";
import { connectionText } from "./status-logic";

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
});
