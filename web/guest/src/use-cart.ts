// The cart as the screens use it: kept on the phone by table code, held
// against the menu each time the menu is read, and priced from it.

import { useEffect, useMemo, useState } from "react";
import type { GuestMenuResponse, Money } from "../../shared/api-types";
import {
  addAll,
  addLine,
  cartFullText,
  countOf,
  droppedText,
  emptyCart,
  parse,
  reconcile,
  serialize,
  setQty,
  storageKey,
  totalOf,
  type Cart,
  type CartLine,
  type PricedLine,
} from "./cart-logic";

export interface CartApi {
  /** Each line, priced from the menu as it is now. Empty until the menu has been read. */
  lines: readonly PricedLine[];
  /** How many dishes, counting each one of a line. */
  count: number;
  total: Money;
  /** What was taken out because it can no longer be ordered, or empty. */
  notice: string;
  dismissNotice: () => void;
  add: (line: CartLine) => void;
  /** Adds an order's lines onto what is in the cart, and says if not all of it went in. */
  addMany: (lines: readonly CartLine[]) => void;
  setQty: (key: string, qty: number) => void;
  clear: () => void;
}

// Storage can be switched off, full, or refused in a private window. The
// cart then lasts as long as the page, which is still a cart.
function read(key: string): Cart {
  try {
    return parse(window.localStorage.getItem(key));
  } catch {
    return emptyCart;
  }
}

function write(key: string, cart: Cart): void {
  try {
    if (cart.length === 0) window.localStorage.removeItem(key);
    else window.localStorage.setItem(key, serialize(cart));
  } catch {
    // Nothing to do, and nothing a guest could do about it.
  }
}

export function useCart(code: string, menu: GuestMenuResponse | undefined): CartApi {
  const key = storageKey(code);
  const [cart, setCart] = useState<Cart>(() => read(key));
  const [notice, setNotice] = useState("");

  useEffect(() => write(key, cart), [key, cart]);

  const held = useMemo(() => (menu ? reconcile(cart, menu) : null), [cart, menu]);

  // The menu was read again and something in the cart has gone from it.
  useEffect(() => {
    if (held && held.cart !== cart) {
      setCart(held.cart);
      setNotice(droppedText(held.dropped));
    }
  }, [held, cart]);

  const lines = held?.lines ?? [];
  return {
    lines,
    count: held ? countOf(held.cart) : countOf(cart),
    total: totalOf(lines, menu?.currency ?? ""),
    notice,
    dismissNotice: () => setNotice(""),
    add: (line) => setCart((c) => addLine(c, line)),
    addMany: (lines) => {
      const added = addAll(cart, lines);
      setCart(added.cart);
      // What the menu no longer has is said when the cart is next held
      // against it, in the same place.
      setNotice(added.short ? cartFullText : "");
    },
    setQty: (lineKey, qty) => setCart((c) => setQty(c, lineKey, qty)),
    clear: () => setCart(emptyCart),
  };
}
