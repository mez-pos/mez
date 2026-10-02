// What the pairing and lock screens say when something goes wrong. Each
// message says what happened and what to do next, in words for whoever is
// standing at the till, not for whoever wrote the server.

import { ApiError } from "../api";
import type { DeviceKind } from "../api-types";
import { WrongDeviceKind } from "./auth-store";
import { formatWait } from "./format";

const kindNames: Record<DeviceKind, string> = {
  terminal: "terminal",
  kitchen: "kitchen display",
  back_office: "back office",
  guest: "guest",
};

export function unreachableMessage(host: string): string {
  return `Can't reach the mez server at ${host}. Check this device is on the venue's network.`;
}

export function pairingError(err: unknown, host: string): string {
  if (err instanceof WrongDeviceKind) {
    return `That code pairs a ${kindNames[err.got]}, and this is the ${kindNames[err.want]}. Ask for a ${kindNames[err.want]} code.`;
  }
  if (!(err instanceof ApiError)) return "Something went wrong. Try again.";
  if (err.unreachable) return unreachableMessage(host);
  switch (err.code) {
    case "invalid_pairing_code":
      return "That code didn't work. Codes last 10 minutes and work once, so it may have expired or been used. Ask for a new one.";
    case "rate_limited":
      return err.retryAfterMs !== null ? `Too many tries. Wait ${formatWait(err.retryAfterMs)} and try again.` : "Too many tries. Wait a minute and try again.";
    default:
      return `The server refused: ${err.message}`;
  }
}

/** A sign-in failure, and whether the PIN pad should wait before the next try. */
export interface SignInFailure {
  readonly message: string;
  readonly lockedUntil: number | null;
}

export function signInError(err: unknown, host: string, now: number): SignInFailure {
  if (!(err instanceof ApiError)) return { message: "Something went wrong. Try again.", lockedUntil: null };
  if (err.unreachable) return { message: `${unreachableMessage(host)} Your PIN wasn't checked.`, lockedUntil: null };
  switch (err.code) {
    case "invalid_pin":
      return { message: "That PIN isn't right. Try again.", lockedUntil: null };
    case "locked_out": {
      // The server always sends Retry-After with a lockout; five minutes is
      // a floor for a proxy that dropped it, not a guess at the real wait.
      const wait = err.retryAfterMs ?? 5 * 60_000;
      return { message: "Too many wrong PINs.", lockedUntil: now + wait };
    }
    default:
      return { message: `The server refused: ${err.message}`, lockedUntil: null };
  }
}
