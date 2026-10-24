// A payment whose answer was lost, kept across a reload (ADR-368). The pay
// screen holds such a payment, and its idempotency key, until the order says
// whether it was recorded. In memory alone, a reload, a lock or a slip onto
// Back would lose them, and the next Charge would be a new payment with a
// new key: the one way left to charge a guest twice. sessionStorage lives as
// long as the tab, which is as long as the question can stay open.

const prefix = "mez.unsettled.";

/** Keeps what the pay screen holds for an order. Storage that refuses is tolerated: the screen still holds it in memory. */
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
