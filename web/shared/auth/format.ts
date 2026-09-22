// Small text helpers for the pairing and lock screens.

/**
 * A pairing code as it is typed: uppercased, anything but letters and digits
 * dropped, a hyphen after the fourth, eight at most. "mw4chhwy" shows as
 * "MW4C-HHWY". The server does the real normalising (O for 0 and the like,
 * ADR-054); this only makes what is on screen match what is on the paper.
 */
export function formatPairingCode(input: string): string {
  const symbols = input.toUpperCase().replace(/[^0-9A-Z]/g, "").slice(0, 8);
  return symbols.length > 4 ? `${symbols.slice(0, 4)}-${symbols.slice(4)}` : symbols;
}

export function isCompletePairingCode(code: string): boolean {
  return /^[0-9A-Z]{4}-[0-9A-Z]{4}$/.test(code);
}

/** "Asha Kumar" to "AK", "Owner" to "O". For the name tiles. */
export function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  const first = words[0]?.[0] ?? "";
  const last = words.length > 1 ? (words[words.length - 1]?.[0] ?? "") : "";
  return (first + last).toUpperCase();
}

/** A wait as m:ss, rounded up so it never shows 0:00 while still locked. */
export function formatWait(ms: number): string {
  const total = Math.max(0, Math.ceil(ms / 1000));
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, "0")}`;
}
