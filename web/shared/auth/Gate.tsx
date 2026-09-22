// The route guard for the terminal and the back office: routeFor, applied.
// Until the device has been checked, nothing but the splash is shown, so no
// screen ever renders against a guess.

import type { ReactNode } from "react";
import { Redirect, useLocation } from "wouter";
import { routeFor } from "./route";
import { useAuth } from "./use-auth";

export function Gate({ splash, children }: { splash: ReactNode; children: ReactNode }) {
  const status = useAuth();
  const [path] = useLocation();
  if (status.kind === "checking") return <>{splash}</>;
  const to = routeFor(status, path);
  return to ? <Redirect to={to} replace /> : <>{children}</>;
}
