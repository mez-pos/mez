// Route guards, as a function of the auth state: where a path must go
// instead, or null to stay. Every guard in every app is this one function.

import type { AuthStatus } from "./auth-store";

export const paths = { pair: "/pair", lock: "/lock", home: "/" } as const;

export function routeFor(status: AuthStatus, path: string): string | null {
  switch (status.kind) {
    case "checking":
      return null; // the splash covers everything until it is known
    case "unpaired":
      return path === paths.pair ? null : paths.pair;
    case "no_venue":
      return null; // the Gate shows its own screen, whatever the path
    case "locked":
      return path === paths.lock ? null : paths.lock;
    case "signed_in":
      return path === paths.pair || path === paths.lock ? paths.home : null;
  }
}
