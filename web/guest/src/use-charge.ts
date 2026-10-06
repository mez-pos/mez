// Paying online from the bill (ADR-222): start a charge and go to the
// gateway's page; back on the bill, ask the venue how the charge the phone
// started stands, until it has an answer.

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import type { GuestChargeResponse } from "../../shared/api-types";
import { CHARGE_ASK_MS, chargeOpen, chargeStorageKey, checkoutUrlOf, parseStarted, startFailure, type StartedCharge } from "./charge-logic";
import { billKey, readCharge, startCharge } from "./use-guest";

export interface ChargeApi {
  /** The charge this phone started, as the venue last answered, if any. */
  charge: GuestChargeResponse | undefined;
  starting: boolean;
  /** Why paying online could not be started, or empty. */
  error: string;
  /** Starts paying what is owed, and leaves for the gateway's page. */
  pay: () => void;
  /** Puts an answered charge away, so the bill can offer paying again. */
  dismiss: () => void;
}

function load(code: string): StartedCharge | null {
  try {
    return parseStarted(window.localStorage.getItem(chargeStorageKey(code)), Date.now());
  } catch {
    return null;
  }
}

function save(code: string, started: StartedCharge | null): void {
  try {
    if (started === null) window.localStorage.removeItem(chargeStorageKey(code));
    else window.localStorage.setItem(chargeStorageKey(code), JSON.stringify(started));
  } catch {
    // With storage refused the phone forgets the charge on leaving the
    // page. The venue does not: its own sweep puts the payment on the bill.
  }
}

/** Whether this phone has a charge to come back to: the app opens on the bill for it. */
export function hasStartedCharge(code: string): boolean {
  return load(code) !== null;
}

export function useCharge(code: string): ChargeApi {
  const queryClient = useQueryClient();
  const [started, setStarted] = useState<StartedCharge | null>(() => load(code));
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState("");

  const query = useQuery({
    queryKey: ["guest", "charge", code, started?.id ?? ""],
    queryFn: ({ signal }) => readCharge(code, started?.id ?? "", signal),
    enabled: started !== null,
    // Asked again while it is open, and not once it has an answer.
    refetchInterval: (q) => (q.state.data === undefined || chargeOpen(q.state.data.status) ? CHARGE_ASK_MS : false),
    refetchOnWindowFocus: true,
    staleTime: 0,
    gcTime: 0,
    retry: false,
  });
  const status = query.data?.status;

  // An answer moves the bill: read it again.
  useEffect(() => {
    if (status && !chargeOpen(status)) void queryClient.invalidateQueries({ queryKey: billKey(code) });
  }, [status, code, queryClient]);

  // A charge the venue no longer knows is forgotten here too.
  useEffect(() => {
    if (query.isError) {
      save(code, null);
      setStarted(null);
    }
  }, [query.isError, code]);

  const pay = () => {
    if (starting) return;
    setStarting(true);
    setError("");
    startCharge(code)
      .then((c) => {
        const url = checkoutUrlOf(c.url);
        const next = { id: c.id, at: Date.now() };
        save(code, next);
        setStarted(next);
        // Paid already, or an address the app will not follow: stay, and
        // the charge's own line says where things stand.
        if (url !== null) window.location.assign(url);
      })
      .catch((err: unknown) => setError(startFailure(err)))
      .finally(() => setStarting(false));
  };

  const dismiss = () => {
    save(code, null);
    setStarted(null);
  };

  return { charge: started === null ? undefined : query.data, starting, error, pay, dismiss };
}
