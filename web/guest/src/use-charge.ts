// Paying online (ADR-222): start a charge and go to the gateway's page;
// back in the app, ask the venue how the charge the phone started stands,
// until it has an answer. The charge is for what the table owes, from the
// bill, or for one order the venue holds until it is paid (ADR-227).

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import type { GuestChargeResponse, GuestShareRequest } from "../../shared/api-types";
import {
  CHARGE_ASK_MS,
  chargeGivenUp,
  chargeOpen,
  letGoFailure,
  chargeStorageKey,
  checkoutUrlOf,
  parseStarted,
  parseStartedFor,
  roundChargeStorageKey,
  roundStartFailure,
  startFailure,
  type StartedCharge,
} from "./charge-logic";
import { billKey, letGoCharge, readCharge, startCharge, startRoundCharge } from "./use-guest";

/** What is being paid for: the bill, or one order by its id. */
export interface ChargeFor {
  round: string;
  /** Told when the charge has its answer, so the order is read again. */
  onAnswered: () => void;
}

export interface ChargeApi {
  /** The charge this phone started, as the venue last answered, if any. */
  charge: GuestChargeResponse | undefined;
  starting: boolean;
  /** Why paying online could not be started, or empty. */
  error: string;
  /**
   * Starts paying, and leaves for the gateway's page: the rest of what is
   * owed, or with a share this phone's part of the bill (ADR-233).
   */
  pay: (share?: GuestShareRequest | null) => void;
  /** Gives up the open payment's page, so it holds no part of the bill (ADR-234). */
  letGo: () => void;
  lettingGo: boolean;
  /** Puts an answered charge away, so the bill can offer paying again. */
  dismiss: () => void;
}

function load(code: string, round?: string): StartedCharge | null {
  try {
    if (round === undefined) return parseStarted(window.localStorage.getItem(chargeStorageKey(code)), Date.now());
    return parseStartedFor(window.localStorage.getItem(roundChargeStorageKey(code)), Date.now(), round);
  } catch {
    return null;
  }
}

function save(code: string, started: StartedCharge | null, round?: string): void {
  const key = round === undefined ? chargeStorageKey(code) : roundChargeStorageKey(code);
  try {
    if (started === null) window.localStorage.removeItem(key);
    else window.localStorage.setItem(key, JSON.stringify(started));
  } catch {
    // With storage refused the phone forgets the charge on leaving the
    // page. The venue does not: its own sweep puts the payment on the bill.
  }
}

/** Whether this phone has a charge to come back to: the app opens on the bill for it. */
export function hasStartedCharge(code: string): boolean {
  return load(code) !== null;
}

/** Whether this phone started paying for this order: the app opens on the order for it. */
export function hasStartedRoundCharge(code: string, round: string): boolean {
  return load(code, round) !== null;
}

export function useCharge(code: string, of?: ChargeFor): ChargeApi {
  const queryClient = useQueryClient();
  const round = of?.round;
  const [started, setStarted] = useState<StartedCharge | null>(() => load(code, round));
  const [starting, setStarting] = useState(false);
  const [lettingGo, setLettingGo] = useState(false);
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

  // An answer moves the bill, and the order it paid for: read them again.
  useEffect(() => {
    if (!status || chargeOpen(status)) return;
    void queryClient.invalidateQueries({ queryKey: billKey(code) });
    of?.onAnswered();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- told once for each answer
  }, [status, code, queryClient]);

  // A charge the venue no longer knows is forgotten here too, and so is
  // one whose page the table gave up: this phone, or another at the table.
  const givenUp = chargeGivenUp(query.data);
  useEffect(() => {
    if (query.isError || givenUp) {
      save(code, null, round);
      setStarted(null);
      if (givenUp) void queryClient.invalidateQueries({ queryKey: billKey(code) });
    }
  }, [query.isError, givenUp, code, round, queryClient]);

  const pay = (share: GuestShareRequest | null = null) => {
    if (starting) return;
    setStarting(true);
    setError("");
    (round === undefined ? startCharge(code, share) : startRoundCharge(code, round))
      .then((c) => {
        const url = checkoutUrlOf(c.url);
        const next: StartedCharge = round === undefined ? { id: c.id, at: Date.now() } : { id: c.id, at: Date.now(), round };
        save(code, next, round);
        setStarted(next);
        // Paid already, or an address the app will not follow: stay, and
        // the charge's own line says where things stand.
        if (url !== null) window.location.assign(url);
      })
      .catch((err: unknown) => {
        setError(round === undefined ? startFailure(err) : roundStartFailure(err));
        // Refused because it is paid already, or gone: the order says which,
        // and the bill what is left of it.
        of?.onAnswered();
        if (round === undefined) void queryClient.invalidateQueries({ queryKey: billKey(code) });
      })
      .finally(() => setStarting(false));
  };

  const letGo = () => {
    if (lettingGo || started === null) return;
    setLettingGo(true);
    setError("");
    letGoCharge(code, started.id)
      // The answer is the charge as it stands: given up, or paid after all.
      .then((c) => queryClient.setQueryData(["guest", "charge", code, started.id], c))
      .catch(() => setError(letGoFailure))
      .finally(() => setLettingGo(false));
  };

  const dismiss = () => {
    save(code, null, round);
    setStarted(null);
  };

  return { charge: started === null || givenUp ? undefined : query.data, starting, error, pay, dismiss, letGo, lettingGo };
}
