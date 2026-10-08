// The words for "Back office from anywhere" (docs/design/screens/
// backoffice-devices.md): whether the back office can be opened away from
// the venue, and if it can, where. Pure, so each case's sentence is tested
// without a server that is in it. The setting is read when mez starts, so
// this says how it is turned on and is not a switch (ADR-306, ADR-309).

import type { TunnelResponse } from "../../../shared/api-types";
import type { RemoteState } from "../../../shared/ui";

type Remote = Pick<TunnelResponse, "state" | "back_office" | "back_office_url">;

/**
 * Where it stands: off, the usual case and no fault; on and reachable; or
 * on with the relay not reached, when the address would open nothing.
 */
export function remoteState(t: Pick<Remote, "state" | "back_office">): RemoteState {
  if (!t.back_office) return "off";
  return t.state === "connected" ? "on" : "unreachable";
}

export interface RemoteWords {
  /** What it means, in a sentence an owner can act on. */
  line: string;
  /** The address to open, when there is one that would open. */
  address: string | null;
  /** What to do the first time, or to turn it on. */
  steps: readonly string[];
}

/**
 * The card's words. `away` is this page itself being used through the relay,
 * when the address is the one in the browser's own bar and the steps are done.
 */
export function remoteWords(t: Remote, away: boolean): RemoteWords {
  const address = t.back_office ? (t.back_office_url ?? null) : null;

  if (away) {
    return { line: "You are using it now: this is the back office, opened away from the venue.", address, steps: [] };
  }
  if (!t.back_office) {
    if (t.state === "off") {
      return {
        line: "The back office opens only at the venue. Opening it from home needs a relay first, which is also what lets guests order from their phones.",
        address: null,
        steps: [],
      };
    }
    return {
      line: "The back office opens only at the venue. It can be opened from home too: the menu, tables, stock, staff and reports, and nothing that takes money.",
      address: null,
      steps: [
        "Ask whoever runs the relay for a key that allows it.",
        "Put that key in the settings file as relay_key, and add remote_backoffice = true.",
        "Restart mez.",
      ],
    };
  }
  if (address === null) {
    return {
      line: "It is turned on, but this venue has not said where it is reached, so there is no address to open.",
      address: null,
      steps: ["Set public_url in the settings file to the venue's own address, such as https://my-cafe.example.com, and restart mez."],
    };
  }
  const first = [
    "Open the address on your phone or laptop.",
    "The first time, it asks for a pairing code. Make one here with Pair a device, as a Back office. A code is made at the venue.",
    "Sign in with your PIN. Only someone who manages the venue can.",
  ];
  if (t.state !== "connected") {
    return {
      line: "It is turned on, but the relay is not reached right now, so the address opens nothing until it is. The counter is not affected.",
      address,
      steps: first,
    };
  }
  return { line: "The back office can be opened away from the venue, at this address.", address, steps: first };
}
