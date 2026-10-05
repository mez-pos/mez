// One of the four figures along the top of the dashboard (D12): an icon in a
// tinted round, what the figure is, the figure in the serif, and a line under
// it. The first tile is the day's sales, on the green tint with a leaf behind.

import { LeafIcon, MinusIcon, TrendingDownIcon, TrendingUpIcon, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { cx } from "../../../shared/ui/cx";
import { changeText, type Change } from "./dashboard-logic";

const rounds = {
  success: "bg-chip-success-bg text-accent",
  attention: "bg-attention-tint text-attention",
  warning: "bg-chip-warning-bg text-chip-warning-fg",
  info: "bg-chip-info-bg text-chip-info-fg",
} as const;

interface StatTileProps {
  icon: LucideIcon;
  tone: keyof typeof rounds;
  label: string;
  /** The figure, or the words in its place while it is read. */
  children: ReactNode;
  note?: ReactNode;
  /** The day's sales: the green tint and the leaf. */
  featured?: boolean;
}

export function StatTile({ icon: Icon, tone, label, children, note, featured = false }: StatTileProps) {
  return (
    <div
      className={cx(
        "relative flex min-w-0 items-center gap-4 overflow-hidden rounded-card border p-5",
        featured ? "border-chip-success-border bg-accent-tint" : "border-line bg-raised shadow-card",
      )}
    >
      {featured && <LeafIcon aria-hidden="true" size={110} strokeWidth={1} className="absolute -bottom-4 -right-4 text-accent opacity-15" />}
      <span aria-hidden="true" className={cx("relative flex size-14 shrink-0 items-center justify-center rounded-round", rounds[tone])}>
        <Icon size={26} />
      </span>
      <div className="relative min-w-0">
        <p className="text-sm font-medium text-fg-2">{label}</p>
        <div className="font-serif text-total leading-tight tabular-nums">{children}</div>
        {note && <div className="mt-0.5 text-sm text-fg-muted">{note}</div>}
      </div>
    </div>
  );
}

const trends = {
  up: { icon: TrendingUpIcon, ink: "text-success" },
  down: { icon: TrendingDownIcon, ink: "text-danger" },
  same: { icon: MinusIcon, ink: "text-fg-muted" },
} as const;

/**
 * Today so far beside the whole of yesterday: an arrow, the percentage and the
 * words. Never the colour alone.
 */
export function ChangeNote({ change }: { change: Change | null }) {
  if (!change || change.direction === "same") return <span>{changeText(change)}</span>;
  const { icon: Icon, ink } = trends[change.direction];
  return (
    <span className="flex items-center gap-1.5">
      <span className="sr-only">{changeText(change)}</span>
      <span aria-hidden="true" className={cx("flex items-center gap-1.5 font-bold", ink)}>
        <Icon size={14} strokeWidth={2.4} />
        {change.percent}%
      </span>
      <span aria-hidden="true">{change.direction === "up" ? "up on" : "down on"} all of yesterday</span>
    </span>
  );
}
