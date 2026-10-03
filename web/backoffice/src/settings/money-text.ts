// An amount as it is spoken and typed, "0.50", against how the server holds
// it, minor units (50). `perMajor` is how many minor units make one major unit
// of the currency: 100 for rupees, 1 for yen. The server reports it, so no
// currency's decimals are written into the client. Never through a float.

const decimalsOf = (perMajor: number): number => String(perMajor).length - 1;

/** Minor units as major units with the currency's decimals: 50 is "0.50". */
export function majorText(minor: number, perMajor: number): string {
  const d = decimalsOf(perMajor);
  if (d === 0) return String(minor);
  return `${Math.trunc(minor / perMajor)}.${String(minor % perMajor).padStart(d, "0")}`;
}

/**
 * What was typed, in minor units, or null when it is not a non-negative amount
 * with no more decimals than the currency has. Zero is an amount; the range is
 * the caller's to check.
 */
export function parseMajor(text: string, perMajor: number): number | null {
  const d = decimalsOf(perMajor);
  const m = /^(\d{1,9})(?:\.(\d+))?$/.exec(text.trim());
  const fraction = m?.[2] ?? "";
  // No more decimals than the currency has: "0.505" is not an amount in rupees.
  if (!m || fraction.length > d) return null;
  return Number(m[1]) * perMajor + (d === 0 ? 0 : Number(fraction.padEnd(d, "0")));
}
