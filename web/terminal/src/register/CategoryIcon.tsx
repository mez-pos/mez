// The icon a menu category wears in the sidebar and on a photoless item card
// (register-logic.ts: categoryIconKey).

import { BeefIcon, CakeSliceIcon, CoffeeIcon, LeafIcon, PizzaIcon, SaladIcon, SoupIcon, StarIcon, UtensilsIcon, type LucideIcon } from "lucide-react";
import type { CategoryIconKey } from "./register-logic";

const icons: Record<CategoryIconKey, LucideIcon> = {
  utensils: UtensilsIcon,
  star: StarIcon,
  leaf: LeafIcon,
  soup: SoupIcon,
  pizza: PizzaIcon,
  burger: BeefIcon,
  salad: SaladIcon,
  coffee: CoffeeIcon,
  dessert: CakeSliceIcon,
};

export function CategoryIcon({ name, size = 19, className }: { name: CategoryIconKey; size?: number; className?: string }) {
  const Icon = icons[name];
  return <Icon aria-hidden="true" size={size} className={className} />;
}
