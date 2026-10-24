// A pairing code carried in the address a device was opened at (ADR-367):
// the QR code the back office shows beside a pairing code is the app's
// address with `#code=7K3M-9QX2` on the end, so a camera lands on the
// pairing screen with the code typed in. A fragment never reaches a server.
//
// It is read once, as this module loads, which is before any screen is
// drawn: the route guard then sends the device to /pair with a bare
// address, and the hash would be gone by the time the form asked for it.

import { formatPairingCode, isCompletePairingCode } from "./format";

/** The complete pairing code in a location hash, or null when there is none. */
export function readLinkedCode(hash: string): string | null {
  const match = /(?:^#|&)code=([^&]*)/.exec(hash);
  if (!match || match[1] === undefined) return null;
  let raw: string;
  try {
    raw = decodeURIComponent(match[1]);
  } catch {
    return null;
  }
  const code = formatPairingCode(raw);
  return isCompletePairingCode(code) ? code : null;
}

/** The hash with its code taken out: "" when the code was all it held. */
export function withoutCode(hash: string): string {
  const rest = hash
    .replace(/^#/, "")
    .split("&")
    .filter((part) => part !== "" && !part.startsWith("code="));
  return rest.length > 0 ? `#${rest.join("&")}` : "";
}

const captured: string | null = typeof window === "undefined" ? null : readLinkedCode(window.location.hash);

// Taken out of the address at once, so a reload or a shared screenshot of
// the address bar does not carry a code that is good once.
if (captured !== null && typeof window !== "undefined") {
  window.history.replaceState(window.history.state, "", window.location.pathname + window.location.search + withoutCode(window.location.hash));
}

/** The code the device was opened with, if any. The same answer for the life of the page. */
export function linkedCode(): string | null {
  return captured;
}
