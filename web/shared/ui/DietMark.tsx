// The mark beside a dish's name (D03, D16), as India's packaging marks it
// (FSSAI): a square with a dot for veg and vegan, in green; a square with a
// triangle for non-veg, in brick. The shape tells them apart without the
// colour, for anyone who cannot tell green from brick (GST-1, TILL-15,
// ADR-389). The word is its label, for a screen reader and on hover.

import type { Diet } from "../api-types";
import { cx } from "./cx";

const dietWords: Record<Exclude<Diet, "">, string> = { veg: "Vegetarian", non_veg: "Non-vegetarian", vegan: "Vegan" };

export function DietMark({ diet, className = "mt-1" }: { diet: Diet | undefined; className?: string }) {
  if (!diet) return null;
  const nonVeg = diet === "non_veg";
  return (
    <svg
      role="img"
      aria-label={dietWords[diet]}
      viewBox="0 0 16 16"
      className={cx("block size-4 shrink-0", nonVeg ? "text-danger" : "text-success", className)}
    >
      <title>{dietWords[diet]}</title>
      <rect x="1" y="1" width="14" height="14" rx="2" fill="none" stroke="currentColor" strokeWidth="2" />
      {nonVeg ? <path d="M8 4.25 11.75 11.25H4.25Z" fill="currentColor" /> : <circle cx="8" cy="8" r="3" fill="currentColor" />}
    </svg>
  );
}
