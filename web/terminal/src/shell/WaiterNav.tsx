// The waiter's navigation (T01): tall tiles, an icon over a word, for a tablet
// held in one hand. Home, Orders and Menu are screens; More opens a menu of what
// the app has and T01 does not draw: the floor plan, the shift summary (T08) and
// the print queue. The label is 15px,
// not the design's 12.5: the floor for anything tapped (ADR-014).

import { ClockIcon, EllipsisIcon, HouseIcon, LayoutGridIcon, PrinterIcon, ReceiptTextIcon, UtensilsIcon, type LucideIcon } from "lucide-react";
import { Link } from "wouter";
import { cx } from "../../../shared/ui/cx";
import { waiterNav, type WaiterNavId } from "./nav-logic";
import { route } from "./routes";
import { useDisclosure } from "../../../shared/shell/use-disclosure";

const icons: Record<Exclude<WaiterNavId, "more">, LucideIcon> = {
  home: HouseIcon,
  orders: ReceiptTextIcon,
  menu: UtensilsIcon,
};

const tile = (pressed: boolean) =>
  cx(
    "flex h-15 w-24 flex-col items-center justify-center gap-0.5 rounded text-base outline-offset-2 outline-line-focus focus-visible:outline-2",
    pressed ? "bg-accent font-bold text-accent-fg" : "font-medium text-fg hover:bg-raised-high",
  );

interface WaiterNavProps {
  active: WaiterNavId | null;
  onPrintQueue: () => void;
}

export function WaiterNav({ active, onPrintQueue }: WaiterNavProps) {
  const { open, toggle, box, trigger, panel, panelId, pick, leaves } = useDisclosure();
  const row =
    "flex min-h-tap w-full items-center gap-3 rounded px-4 text-left text-base outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2";

  return (
    <nav aria-label="Main" className="ml-4 flex gap-2">
      {waiterNav.map((item) => {
        const Icon = icons[item.id];
        const pressed = item.id === active;
        return (
          <Link key={item.id} href={item.href} aria-current={pressed ? "page" : undefined} className={tile(pressed)}>
            <Icon aria-hidden="true" size={22} />
            {item.label}
          </Link>
        );
      })}
      <div ref={box} onBlur={leaves} className="relative">
        <button ref={trigger} type="button" aria-expanded={open} aria-controls={panelId} onClick={toggle} className={tile(active === "more")}>
          <EllipsisIcon aria-hidden="true" size={22} />
          More
        </button>
        {open && (
          <div
            ref={panel}
            id={panelId}
            role="group"
            aria-label="More"
            className="absolute left-0 top-full z-20 mt-2 w-64 rounded-card border border-line bg-raised p-2 shadow-card"
          >
            {/* A link, so the floor opens as any screen does; the click closes the menu behind it. */}
            <Link href={route.tables} onClick={pick(() => undefined)} className={row}>
              <LayoutGridIcon aria-hidden="true" size={18} />
              Floor plan
            </Link>
            <Link href={route.waiterShift} onClick={pick(() => undefined)} className={row}>
              <ClockIcon aria-hidden="true" size={18} />
              Your shift
            </Link>
            <button type="button" onClick={pick(onPrintQueue)} className={row}>
              <PrinterIcon aria-hidden="true" size={18} />
              Print queue
            </button>
          </div>
        )}
      </div>
    </nav>
  );
}
