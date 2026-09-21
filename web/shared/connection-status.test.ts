import { describe, expect, it } from "vitest";
import { describeConnection } from "./connection-status";
import type { RealtimeStatus } from "./realtime";

const now = 1_000_000;

describe("describeConnection", () => {
  const tests: { name: string; status: RealtimeStatus; tone: string; word: string; detail: string | null; retryable: boolean }[] = [
    { name: "no token", status: { state: "idle" }, tone: "muted", word: "Not paired", detail: null, retryable: false },
    { name: "first connect", status: { state: "connecting", attempt: 1 }, tone: "warning", word: "Connecting", detail: null, retryable: false },
    { name: "a later connect", status: { state: "connecting", attempt: 4 }, tone: "warning", word: "Connecting", detail: "attempt 4", retryable: false },
    { name: "replaying", status: { state: "syncing", target: 9 }, tone: "warning", word: "Syncing", detail: "catching up", retryable: false },
    { name: "live", status: { state: "live", since: now }, tone: "success", word: "Live", detail: null, retryable: false },
    {
      name: "lost, reason not known yet",
      status: { state: "reconnecting", attempt: 1, retryAt: now + 1200, reason: "checking" },
      tone: "warning", word: "Reconnecting", detail: "retrying in 2s", retryable: true,
    },
    {
      name: "server answers, stream dropped",
      status: { state: "reconnecting", attempt: 2, retryAt: now + 3000, reason: "stream_lost" },
      tone: "warning", word: "Reconnecting", detail: "retrying in 3s", retryable: true,
    },
    {
      name: "server gone",
      status: { state: "reconnecting", attempt: 3, retryAt: now + 8000, reason: "unreachable" },
      tone: "danger", word: "Server unreachable", detail: "retrying in 8s", retryable: true,
    },
    {
      name: "no network",
      status: { state: "reconnecting", attempt: 1, retryAt: now + 500, reason: "offline" },
      tone: "danger", word: "No network", detail: "retrying in 1s", retryable: true,
    },
    {
      name: "the retry is due",
      status: { state: "reconnecting", attempt: 1, retryAt: now - 10, reason: "checking" },
      tone: "warning", word: "Reconnecting", detail: "retrying now", retryable: true,
    },
    { name: "revoked", status: { state: "unpaired", code: "device_revoked" }, tone: "danger", word: "Device unpaired", detail: "pair it again", retryable: false },
    { name: "token unknown", status: { state: "unpaired", code: "unauthenticated" }, tone: "danger", word: "Device not recognised", detail: "pair it again", retryable: false },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(describeConnection(tt.status, now)).toEqual({ tone: tt.tone, word: tt.word, detail: tt.detail, retryable: tt.retryable });
    });
  }

  it("never describes a state with colour alone", () => {
    for (const tt of tests) expect(describeConnection(tt.status, now).word.length).toBeGreaterThan(0);
  });
});
