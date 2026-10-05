// The square-and-dot mark beside a dish's name (D03, D16): green for veg and
// vegan, brick for non-veg. Colour plus a word: the word is its label, for a
// screen reader and on hover.

import type { Diet } from "../api-types";
import { cx } from "./cx";

const dietWords: Record<Exclude<Diet, "">, string> = { veg: "Vegetarian", non_veg: "Non-vegetarian", vegan: "Vegan" };

export function DietMark({ diet, className = "mt-1" }: { diet: Diet | undefined; className?: string }) {
  if (!diet) return null;
  const tone = diet === "non_veg" ? "border-danger text-danger" : "border-success text-success";
  return (
    <span
      role="img"
      aria-label={dietWords[diet]}
      title={dietWords[diet]}
      className={cx("flex size-4 shrink-0 items-center justify-center rounded-[3px] border-2", tone, className)}
    >
      <span className="size-1.5 rounded-round bg-current" />
    </span>
  );
}
