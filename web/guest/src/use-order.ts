// Placing an order, and remembering the one placed (guest-order.md,
// ADR-213). The phone sends its cart with a key, keeps the answer by table
// code, and asks about it at the order's own address (ADR-238): whether it
// was accepted or declined, and once it is sent how far the kitchen has come.

import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import {
  askAfterMs,
  drawKey,
  keyFor,
  keyStorageKey,
  orderGone,
  parseKey,
  parsePlaced,
  placedStorageKey,
  refusalOf,
  toRequest,
  withRound,
  type Placed,
} from "./order-logic";
import type { CartApi } from "./use-cart";
import { menuKey, sendOrder, trackOrder } from "./use-guest";
import { keepBill } from "./use-receipt";

export interface OrderApi {
  /** The order this phone last placed at this table, if it still remembers one. */
  placed: Placed | null;
  placing: boolean;
  /** Why the last try did not place it, or empty. */
  error: string;
  /** Sends the cart. Resolves true once the venue has it. */
  place: () => Promise<boolean>;
  /** Asks now how the order placed stands: its payment has just been answered. */
  refresh: () => void;
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
  // The id of an order the venue answered "not found" for: asking again
  // would say the same, every few seconds, for hours. What the phone
  // remembers of it stays on the screen.
  const [gone, setGone] = useState<string | null>(null);

  const keep = (next: Placed) => {
    setPlaced(next);
    // A sent order says which bill it is on: remembered for the receipt.
    keepBill(code, next.round.bill);
    save(placedStorageKey(code), JSON.stringify(next));
  };

  // An order with an answer still to come is asked about, at its own
  // address: accepted or declined, then each thing the kitchen does with it.
  // How often follows where it is, and it stops when nothing more is coming.
  // A failure to ask is not news; the next ask says. Not while the page is
  // hidden, and at once when it is shown again.
  const lost = placed !== null && placed.round.id === gone;
  const every = lost ? null : askAfterMs(placed);
  const ask = () => {
    if (placed === null || lost || document.visibilityState === "hidden") return;
    trackOrder(code, placed.round.id)
      .then((round) => keep(withRound(placed, round, true)))
      .catch((err: unknown) => {
        if (orderGone(err)) setGone(placed.round.id);
      });
  };
  // An order just sent has not been asked about at its own address yet:
  // ask now, and not at the next tick, for where it is in the kitchen.
  const unasked = placed !== null && !lost && placed.round.status === "sent" && !placed.tracked;
  useEffect(() => {
    if (unasked) ask();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- once for each order that becomes sent
  }, [unasked, code]);
  useEffect(() => {
    if (every === null) return;
    const t = setInterval(ask, every);
    document.addEventListener("visibilitychange", ask);
    return () => {
      clearInterval(t);
      document.removeEventListener("visibilitychange", ask);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- ask closes over the order as it stands
  }, [code, every, placed]);

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
      keep(withRound({ key: record.key, request, at: Date.now() }, round));
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

  const refresh = ask;

  return { placed, placing, error, place, refresh };
}
