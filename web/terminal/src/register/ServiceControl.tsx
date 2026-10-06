// How the order is served (D03): Dine-in, Takeaway, Delivery. It is chosen
// before the order opens; once it has, it is written on the opening event and
// moves only by seating (ADR-146), so the others are shown dimmed and cannot be
// tapped, and the one it is stays plain.

import { BikeIcon, ShoppingBagIcon, UtensilsIcon, type LucideIcon } from "lucide-react";
import type { ServiceType } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";
import { serviceLabels } from "./rail-logic";

const options: readonly { value: ServiceType; icon: LucideIcon }[] = [
  { value: "dine_in", icon: UtensilsIcon },
  { value: "takeaway", icon: ShoppingBagIcon },
  { value: "delivery", icon: BikeIcon },
];

interface ServiceControlProps {
  value: ServiceType;
  locked: boolean;
  onChange: (next: ServiceType) => void;
  /** Wider buttons, for a bar with room: the waiter's top bar (T02). */
  roomy?: boolean;
}

export function ServiceControl({ value, locked, onChange, roomy = false }: ServiceControlProps) {
  return (
    <div role="group" aria-label="How it is served" className="flex rounded-pill bg-sunken p-1">
      {options.map(({ value: option, icon: Icon }) => {
        const on = option === value;
        return (
          <button
            key={option}
            type="button"
            aria-pressed={on}
            disabled={locked && !on}
            title={locked && !on ? "Set when the order started" : undefined}
            onClick={() => onChange(option)}
            className={cx(
              "flex flex-1 items-center justify-center rounded-pill text-base outline-offset-2 outline-attention focus-visible:outline-3",
              roomy ? "min-h-tap gap-2 whitespace-nowrap px-4" : "min-h-11 gap-1.5 whitespace-nowrap px-1.5 2xl:px-2",
              on ? "bg-accent font-semibold text-accent-fg" : "text-fg-2 hover:bg-raised-high disabled:opacity-40 disabled:hover:bg-transparent",
            )}
          >
            {/* In the rail the icon gives way to the word below 1536 wide: three words fit a narrow rail, three words and icons do not. */}
            <Icon aria-hidden="true" size={16} className={roomy ? undefined : "hidden 2xl:block"} />
            {serviceLabels[option]}
          </button>
        );
      })}
    </div>
  );
}
