// The route guard for the terminal and the back office: routeFor, applied.
// Until the device has been checked, nothing but the splash is shown, so no
// screen ever renders against a guess. A paired device whose server has no
// venue yet is shown that and nothing else (ADR-339).

import type { ReactNode } from "react";
import { Redirect, useLocation } from "wouter";
import { NoVenueScreen } from "../screens/NoVenueScreen";
import { routeFor } from "./route";
import { useAuth } from "./use-auth";

export function Gate({ splash, children }: { splash: ReactNode; children: ReactNode }) {
  const status = useAuth();
  const [path] = useLocation();
  if (status.kind === "checking") return <>{splash}</>;
  // Over every path, and without moving from it: when the venue is there the
  // device carries on where it was.
  if (status.kind === "no_venue") return <NoVenueScreen />;
  const to = routeFor(status, path);
  return to ? <Redirect to={to} replace /> : <>{children}</>;
}
