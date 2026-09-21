// Reconnect delays for the realtime stream.
//
// Equal jitter: each delay is uniform between half and all of an
// exponentially growing ceiling. When the server restarts, every tablet in
// the building loses it at the same instant; without jitter they would all
// come back on the same tick, forever. The half floor keeps a retry from
// landing at zero, so the countdown staff see never flickers.

export interface Backoff {
  /** Delay before the first retry, and the growth unit. */
  readonly baseMs: number;
  /** No wait is ever longer than this, however long the outage. */
  readonly capMs: number;
}

// The server is on the local network, so a first retry can be quick. The cap
// is short too: a till that took thirty seconds to notice the server was
// back would be worse than one that asked a little too often.
export const defaultBackoff: Backoff = { baseMs: 500, capMs: 10_000 };

/**
 * The delay before retry number `attempt` (1 for the first retry).
 * `random` is injectable so tests can pin it; it must return [0, 1).
 */
export function backoffDelay(attempt: number, policy: Backoff = defaultBackoff, random: () => number = Math.random): number {
  const n = Math.max(1, Math.floor(attempt));
  // 2 ** 30 already dwarfs any sane cap; clamping the exponent keeps the
  // arithmetic finite after a week-long outage.
  const ceiling = Math.min(policy.capMs, policy.baseMs * 2 ** Math.min(n - 1, 30));
  const half = ceiling / 2;
  return Math.floor(half + random() * half);
}
