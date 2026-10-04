// The rail's "..." (D03's More Actions): what the order screen can do that the
// design does not draw a button for. Orders (every open order, the way to bring
// another to the front or start one), the history of this order, a discount on
// the whole of it, firing the next course, and past bills (ADR-151).

import { ClockIcon, EllipsisIcon, ListOrderedIcon, PercentIcon, ReceiptTextIcon, FlameIcon, type LucideIcon } from "lucide-react";
import { cx } from "../../../shared/ui/cx";
import { useDisclosure } from "../shell/use-disclosure";

export interface MoreActionsProps {
  /** Open orders at the venue, this one included. */
  orderCount: number;
  hasOrder: boolean;
  canDiscount: boolean;
  /** The next course held back, if one is. */
  nextCourse: number | null;
  onOrders: () => void;
  onHistory: () => void;
  onDiscount: () => void;
  onFire: (course: number) => void;
  onPastBills: () => void;
}

export function MoreActions({ orderCount, hasOrder, canDiscount, nextCourse, onOrders, onHistory, onDiscount, onFire, onPastBills }: MoreActionsProps) {
  const { open, toggle, box, trigger, panel, panelId, pick, leaves } = useDisclosure();
  return (
    <div ref={box} onBlur={leaves} className="relative">
      <button
        ref={trigger}
        type="button"
        aria-label="More actions"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={toggle}
        className={cx(
          "flex size-tap items-center justify-center rounded-round border border-line-strong outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2",
          open && "bg-raised-high",
        )}
      >
        <EllipsisIcon aria-hidden="true" size={18} />
      </button>
      {open && (
        <div
          ref={panel}
          id={panelId}
          role="group"
          aria-label="More actions"
          className="absolute right-0 top-full z-20 mt-2 w-64 rounded-card border border-line bg-raised p-2 shadow-card"
        >
          <Item icon={ListOrderedIcon} label="Orders" detail={String(orderCount)} onClick={pick(onOrders)} />
          <Item icon={ClockIcon} label="Order history" disabled={!hasOrder} onClick={pick(onHistory)} />
          <Item icon={PercentIcon} label="Discount the order" disabled={!canDiscount} onClick={pick(onDiscount)} />
          {nextCourse !== null && <Item icon={FlameIcon} label={`Fire course ${nextCourse}`} onClick={pick(() => onFire(nextCourse))} />}
          <Item icon={ReceiptTextIcon} label="Past bills" onClick={pick(onPastBills)} />
        </div>
      )}
    </div>
  );
}

function Item({ icon: Icon, label, detail, disabled, onClick }: { icon: LucideIcon; label: string; detail?: string; disabled?: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className="flex min-h-tap w-full items-center gap-3 rounded px-4 text-left text-base outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2 disabled:opacity-40 disabled:hover:bg-transparent"
    >
      <Icon aria-hidden="true" size={18} />
      <span className="flex-1">{label}</span>
      {detail && <span className="font-mono text-sm tabular-nums text-fg-muted">{detail}</span>}
    </button>
  );
}
