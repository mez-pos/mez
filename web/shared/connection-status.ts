// What the connection indicator says, as data. Kept apart from the component
// so every state's wording is tested without rendering anything.
//
// The rule from docs/design/tokens.md: colour and a word, never colour alone.
// The word says what staff can do (keep selling, wait, call someone), and the
// detail says when the next try is, so "is it doing anything?" never needs
// asking.

import type { RealtimeStatus } from "./realtime";

export type Tone = "success" | "warning" | "danger" | "muted";

export interface ConnectionDescription {
  tone: Tone;
  word: string;
  detail: string | null;
  /** True when a "retry now" control makes sense. */
  retryable: boolean;
}

export function describeConnection(status: RealtimeStatus, now: number): ConnectionDescription {
  switch (status.state) {
    case "idle":
      return { tone: "muted", word: "Not paired", detail: null, retryable: false };
    case "connecting":
      return { tone: "warning", word: "Connecting", detail: status.attempt > 1 ? `attempt ${status.attempt}` : null, retryable: false };
    case "syncing":
      return { tone: "warning", word: "Syncing", detail: "catching up", retryable: false };
    case "live":
      return { tone: "success", word: "Live", detail: null, retryable: false };
    case "unpaired":
      return {
        tone: "danger",
        word: status.code === "device_revoked" ? "Device unpaired" : "Device not recognised",
        detail: "pair it again",
        retryable: false,
      };
    case "away":
      // Not a fault and nothing to retry: the server answers. It says why
      // nothing on the page moves by itself.
      return { tone: "muted", word: "Away from the venue", detail: null, retryable: false };
    case "reconnecting":
      return reconnecting(status, now);
  }
}

function reconnecting(status: Extract<RealtimeStatus, { state: "reconnecting" }>, now: number): ConnectionDescription {
  const detail = `retrying ${countdown(status.retryAt - now)}`;
  switch (status.reason) {
    case "offline":
      return { tone: "danger", word: "No network", detail, retryable: true };
    case "unreachable":
      return { tone: "danger", word: "Server unreachable", detail, retryable: true };
    case "checking":
    case "stream_lost":
      return { tone: "warning", word: "Reconnecting", detail, retryable: true };
  }
}

function countdown(ms: number): string {
  const seconds = Math.ceil(ms / 1000);
  return seconds <= 0 ? "now" : `in ${seconds}s`;
}
