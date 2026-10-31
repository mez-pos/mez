// The rail's "..." (D03's More Actions): what the order screen can do that the
// design does not draw a button for. Open orders (every open order, the way to
// bring another to the front or start one), this order's history, a discount
// on the whole of it, firing the next course, and today's orders (ADR-151).
// How many orders are open is on the button itself, not only inside (TILL-9).

import { ClockIcon, EllipsisIcon, ListOrderedIcon, PercentIcon, ReceiptTextIcon, FlameIcon, WalletIcon, type LucideIcon } from "lucide-react";
import { cx } from "../../../shared/ui/cx";
import { useDisclosure } from "../../../shared/shell/use-disclosure";

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
  /** The way to the bill (T05), for a rail with no Pay button: the waiter's. */
  pay?: {
    readonly label: string;
    readonly blocked: boolean;
    /** Why it is blocked, shown under it. */
    reason?: string | null;
    readonly onPay: () => void;
  };
}

export function MoreActions({ orderCount, hasOrder, canDiscount, nextCourse, onOrders, onHistory, onDiscount, onFire, onPastBills, pay }: MoreActionsProps) {
  const { open, toggle, box, trigger, panel, panelId, pick, leaves } = useDisclosure();
  return (
    <div ref={box} onBlur={leaves} className="relative">
      <button
        ref={trigger}
        type="button"
        aria-label={orderCount > 0 ? `More actions, ${orderCount} open ${orderCount === 1 ? "order" : "orders"}` : "More actions"}
        aria-expanded={open}
        aria-controls={panelId}
        onClick={toggle}
        className={cx(
          "relative flex size-tap items-center justify-center rounded-round border border-line-strong outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2",
          open && "bg-raised-high",
        )}
      >
        <EllipsisIcon aria-hidden="true" size={18} />
        {orderCount > 0 && (
          <span
            aria-hidden="true"
            className="absolute -right-1.5 -top-1.5 flex min-w-6 items-center justify-center rounded-pill bg-attention px-1.5 font-mono text-sm font-bold tabular-nums text-attention-fg"
          >
            {orderCount}
          </span>
        )}
      </button>
      {open && (
        <div
          ref={panel}
          id={panelId}
          role="group"
          aria-label="More actions"
          className="absolute right-0 top-full z-20 mt-2 w-64 rounded-card border border-line bg-raised p-2 shadow-card"
        >
          {pay && (
            <ActionItem
              icon={WalletIcon}
              label={pay.label}
              disabled={pay.blocked}
              hint={pay.blocked ? (pay.reason ?? undefined) : undefined}
              onClick={pick(pay.onPay)}
            />
          )}
          <ActionItem icon={ListOrderedIcon} label="Open orders" detail={String(orderCount)} onClick={pick(onOrders)} />
          <ActionItem
            icon={ClockIcon}
            label="This order's history"
            disabled={!hasOrder}
            hint={hasOrder ? undefined : "Nothing on the order yet."}
            onClick={pick(onHistory)}
          />
          <ActionItem
            icon={PercentIcon}
            label="Discount the order"
            disabled={!canDiscount}
            hint={canDiscount ? undefined : "Needs an open order with something on it."}
            onClick={pick(onDiscount)}
          />
          {nextCourse !== null && <ActionItem icon={FlameIcon} label={`Fire course ${nextCourse}`} onClick={pick(() => onFire(nextCourse))} />}
          <ActionItem icon={ReceiptTextIcon} label="Today's orders" onClick={pick(onPastBills)} />
        </div>
      )}
    </div>
  );
}

/** One row of a "More Actions" menu: an icon, what it does, and a count if it has one. */
export function ActionItem({
  icon: Icon,
  label,
  detail,
  disabled,
  hint,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  detail?: string;
  disabled?: boolean;
  /** Why it is disabled, in a line under the label: a greyed-out row with no reason is a dead end. */
  hint?: string | undefined;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className="flex min-h-tap w-full items-center gap-3 rounded px-4 py-2 text-left text-base outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2 disabled:hover:bg-transparent"
    >
      <Icon aria-hidden="true" size={18} className={disabled ? "opacity-40" : undefined} />
      <span className="flex-1">
        <span className={disabled ? "block opacity-40" : "block"}>{label}</span>
        {disabled && hint && <span className="block text-sm text-fg-muted">{hint}</span>}
      </span>
      {detail && <span className="font-mono text-sm tabular-nums text-fg-muted">{detail}</span>}
    </button>
  );
}
