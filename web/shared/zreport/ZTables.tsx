// The pieces a Z-report is laid out from: a card, a row of figures, a
// table of groups. Every amount is given; nothing is summed here.

import { useState, type ReactNode } from "react";
import type { Group, Money as Amount } from "../api-types";
import { Button, Card, Money } from "../ui";
import { cx } from "../ui/cx";

export function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card padding="lg" className="flex flex-col gap-2">
      <h2 className="font-serif text-xl">{title}</h2>
      {children}
    </Card>
  );
}

interface RowProps {
  label: ReactNode;
  amount: Amount;
  count?: number;
  strong?: boolean;
  tone?: "default" | "muted" | "danger" | "success";
}

/** A label, an optional count, and an amount, right-aligned. */
export function Row({ label, amount, count, strong, tone = "default" }: RowProps) {
  return (
    <div className={cx("flex items-baseline justify-between gap-3 py-0.5 text-base", strong ? "font-semibold" : "text-fg-2")}>
      <span>
        {label}
        {count !== undefined && <span className="ml-2 font-mono text-sm tabular-nums text-fg-muted">×{count}</span>}
      </span>
      <Money amount={amount} tone={tone} variant={strong ? "total" : "figure"} />
    </div>
  );
}

export function Pair({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-3 py-0.5 text-base text-fg-2">
      <span>{label}</span>
      <span className="font-mono tabular-nums">{value}</span>
    </div>
  );
}

interface GroupTableProps {
  title: string;
  groups: readonly Group[];
  /** What the count counts: units of an item, bills of a channel. */
  unit: string;
  name?: (g: Group) => string;
  /** Show only this many rows until asked for all. */
  fold?: number;
}

/** A heading's groups: name, count, gross, discounts, total. */
export function GroupTable({ title, groups, unit, name = (g) => g.name, fold }: GroupTableProps) {
  const [all, setAll] = useState(false);
  if (groups.length === 0) return null;
  const shown = fold !== undefined && !all ? groups.slice(0, fold) : groups;
  return (
    <Section title={title}>
      <table className="w-full text-base">
        <thead>
          <tr className="text-left text-sm text-fg-muted">
            <th className="py-1 font-normal">Name</th>
            <th className="py-1 text-right font-normal">{unit}</th>
            <th className="py-1 text-right font-normal">Gross</th>
            <th className="py-1 text-right font-normal">Discounts</th>
            <th className="py-1 text-right font-normal">Total</th>
          </tr>
        </thead>
        <tbody>
          {shown.map((g) => (
            <tr key={g.key} className="border-t border-line">
              <td className="py-1.5 pr-3">{name(g)}</td>
              <td className="py-1.5 text-right font-mono tabular-nums">{g.qty}</td>
              <td className="py-1.5 text-right">
                <Money amount={g.gross} />
              </td>
              <td className="py-1.5 text-right">
                <Money amount={g.discounts} tone="muted" />
              </td>
              <td className="py-1.5 text-right">
                <Money amount={g.total} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {shown.length < groups.length && (
        <Button className="self-start" onClick={() => setAll(true)}>
          Show all {groups.length} items
        </Button>
      )}
    </Section>
  );
}
