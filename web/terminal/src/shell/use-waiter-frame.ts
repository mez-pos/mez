// Whether the person signed in works from the waiter's frame (ADR-163): whoever
// lands on the waiter's home does, on every screen they reach.

import { useAuth } from "../../../shared/auth/use-auth";
import { landingFor } from "./landing";
import { route } from "./routes";

export function useWaiterFrame(): boolean {
  const auth = useAuth();
  return auth.kind === "signed_in" && landingFor(auth.staff.role, auth.permissions) === route.waiter;
}
