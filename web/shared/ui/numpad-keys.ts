// What a NumPad key does to what is being typed, apart from the component.
//
// Amounts are typed the way a till does it: digits shift in from the right,
// so 1, 2, 5, 0 is 12.50. The value is always an integer of minor units
// (docs/02-domain.md); there is no decimal point to mistype, and nothing to
// parse.

export type Digit = "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9";
export type NumKey = Digit | "00" | "back" | "clear";

export const digits: readonly Digit[] = ["0", "1", "2", "3", "4", "5", "6", "7", "8", "9"];

/** Minor units after a key. A key that would pass `max` is ignored, not clamped. */
export function applyAmountKey(minor: number, key: NumKey, max: number): number {
  switch (key) {
    case "back":
      return Math.trunc(minor / 10);
    case "clear":
      return 0;
    case "00": {
      const next = minor * 100;
      return next <= max ? next : minor;
    }
    default: {
      const next = minor * 10 + Number(key);
      return next <= max ? next : minor;
    }
  }
}

/** A PIN after a key. PINs are 4 to 8 digits (ADR-056); "00" is not a PIN key. */
export function applyPinKey(pin: string, key: NumKey, maxLength = 8): string {
  switch (key) {
    case "back":
      return pin.slice(0, -1);
    case "clear":
      return "";
    case "00":
      return pin;
    default:
      return pin.length < maxLength ? pin + key : pin;
  }
}

/** The key a physical keyboard press means, if any. Escape is left alone: it closes a modal. */
export function keyFromKeyboard(key: string): NumKey | null {
  if ((digits as readonly string[]).includes(key)) return key as Digit;
  if (key === "Backspace") return "back";
  if (key === "Delete") return "clear";
  return null;
}
