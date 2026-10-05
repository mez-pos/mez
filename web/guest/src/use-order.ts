// Placing an order, and remembering the one placed (guest-order.md,
// ADR-213). The phone sends its cart with a key, keeps the answer by table
// code, and while the answer is "waiting" asks again with the same key,
// which is how it learns the order was accepted or declined until Task 5
// gives it something better to ask.

import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import {
  ASK_AGAIN_MS,
  awaitsAnswer,
  drawKey,
  keyFor,
  keyStorageKey,
  parseKey,
  parsePlaced,
  placedStorageKey,
  refusalOf,
  toRequest,
  type Placed,
} from "./order-logic";
import type { CartApi } from "./use-cart";
import { menuKey, sendOrder } from "./use-guest";

export interface OrderApi {
  /** The order this phone last placed at this table, if it still remembers one. */
  placed: Placed | null;
  placing: boolean;
  /** Why the last try did not place it, or empty. */
  error: string;
  /** Sends the cart. Resolves true once the venue has it. */
  place: () => Promise<boolean>;
}

// Storage can be refused. An order is then remembered for as long as the
// page, and a retry after a reload is a new order: the venue's staff see
// two requests and decline one.
function load(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function save(key: string, value: string | null): void {
  try {
    if (value === null) window.localStorage.removeItem(key);
    else window.localStorage.setItem(key, value);
  } catch {
    // Nothing to do, and nothing a guest could do about it.
  }
}

export function useOrder(code: string, cart: CartApi): OrderApi {
  const queryClient = useQueryClient();
  const [placed, setPlaced] = useState<Placed | null>(() => parsePlaced(load(placedStorageKey(code)), Date.now()));
  const [placing, setPlacing] = useState(false);
  const [error, setError] = useState("");
  const busy = useRef(false);

  const keep = (next: Placed) => {
    setPlaced(next);
    save(placedStorageKey(code), JSON.stringify(next));
  };

  // A waiting order asks again, with the key it was sent with: the server
  // answers with the same order as it stands now. A failure to ask is not
  // news; the next ask says.
  const waiting = awaitsAnswer(placed);
  useEffect(() => {
    if (!waiting || placed === null) return;
    const ask = () => {
      if (document.visibilityState === "hidden") return;
      sendOrder(code, placed.request, placed.key)
        .then((round) => keep({ ...placed, round }))
        .catch(() => undefined);
    };
    const t = setInterval(ask, ASK_AGAIN_MS);
    document.addEventListener("visibilitychange", ask);
    return () => {
      clearInterval(t);
      document.removeEventListener("visibilitychange", ask);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- keep is stable in what it does
  }, [code, waiting, placed]);

  const place = async (): Promise<boolean> => {
    // Two taps are one order: the second finds the first still going.
    if (busy.current || cart.lines.length === 0) return false;
    busy.current = true;
    setPlacing(true);
    setError("");
    const request = toRequest(cart.lines);
    const record = keyFor(parseKey(load(keyStorageKey(code))), request, drawKey);
    save(keyStorageKey(code), JSON.stringify(record));
    try {
      const round = await sendOrder(code, request, record.key);
      keep({ key: record.key, request, round, at: Date.now() });
      save(keyStorageKey(code), null);
      cart.clear();
      return true;
    } catch (err) {
      const refusal = refusalOf(err);
      setError(refusal.text);
      // The cart is held against the menu when it is read, and says what
      // it took out.
      if (refusal.readMenu) void queryClient.invalidateQueries({ queryKey: menuKey(code) });
      return false;
    } finally {
      busy.current = false;
      setPlacing(false);
    }
  };

  return { placed, placing, error, place };
}
