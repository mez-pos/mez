// A payment whose answer was lost, kept across a reload (ADR-368, ADR-376).
// The pay screen and the waiter's bill hold such a payment, and its
// idempotency key, until the order says whether it was recorded. In memory
// alone, a reload, a lock or a slip onto Back would lose them, and the next
// Charge would be a new payment with a new key: the one way left to charge a
// guest twice. sessionStorage lives as long as the tab, which is as long as
// the question can stay open.

const prefix = "mez.unsettled.";

/** Keeps what a screen holds for an order. Storage that refuses is tolerated: the screen still holds it in memory. */
export function saveUnsettled<T>(orderId: string, held: T): void {
  try {
    sessionStorage.setItem(prefix + orderId, JSON.stringify(held));
  } catch {
    // Refused storage (a private window, a full quota) is not worth a word.
  }
}

/** What was kept for an order, or null. Anything that does not parse, or is not the shape, is nothing. */
export function loadUnsettled<T>(orderId: string, isHeld: (value: unknown) => value is T): T | null {
  try {
    const raw = sessionStorage.getItem(prefix + orderId);
    if (raw === null) return null;
    const parsed: unknown = JSON.parse(raw);
    return isHeld(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

export function clearUnsettled(orderId: string): void {
  try {
    sessionStorage.removeItem(prefix + orderId);
  } catch {
    // nothing to clear
  }
}

/** What a screen keeps for an order: the payment, and the attempt whose key replays rather than repeats it. */
export interface Kept<U, A> {
  attempt: A | null;
  unsettled: U;
}

/** A screen's hold on a payment whose answer was lost. */
export interface UnsettledHold<U, A> {
  /** Shows the payment held and keeps it, with its attempt, across a reload. */
  hold(unsettled: U, attempt: A | null): void;
  /** Shows nothing held and forgets what was kept. */
  release(): void;
}

/**
 * The hold for an order, shown through `show` (the screen's setter). The pay
 * screen and the waiter's bill share it: each once had its own copy, and the
 * waiter's release called itself where it meant to show nothing (TBS-20).
 */
export function unsettledHold<U, A>(orderId: string, show: (unsettled: U | null) => void): UnsettledHold<U, A> {
  return {
    hold(unsettled, attempt) {
      show(unsettled);
      saveUnsettled<Kept<U, A>>(orderId, { attempt, unsettled });
    },
    release() {
      show(null);
      clearUnsettled(orderId);
    },
  };
}
