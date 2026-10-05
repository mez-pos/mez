// The stock levels table of the Inventory screen (D19): each ingredient with
// what is on hand, the level it is low at, its cost, a bar for how full it
// is and a chip for where it stands. What can be done to one (receive, log
// wastage, count, its history, edit) is under the row's "...".
//
// The design's Supplier column is not drawn: suppliers and purchase orders
// are out of v1.

import { PackageIcon } from "lucide-react";
import type { StockItem } from "../../../shared/api-types";
import { formatQty } from "../../../shared/stock-qty";
import { Card, EmptyState, InboxIcon, Money, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { RowMenu } from "../RowMenu";
import { levelOf, levelPercent } from "./levels-logic";
import { costMeasures, type HandKind } from "./stock-logic";

export type RowAction = { kind: "move"; move: HandKind } | { kind: "history" } | { kind: "edit" };

interface StockLevelsProps {
  rows: readonly StockItem[];
  /** Ingredients exist but none is shown: they are all retired and hidden. */
  allRetired: boolean;
  onAction: (item: StockItem, action: RowAction) => void;
}

const head = "pb-3 text-meta font-bold uppercase text-fg-muted";
const fills = { ok: "bg-accent", low: "bg-highlight", out: "bg-danger-fill" } as const;

export function StockLevels({ rows, allRetired, onAction }: StockLevelsProps) {
  if (rows.length === 0) {
    return (
      <Card>
        <EmptyState
          icon={<InboxIcon />}
          line={allRetired ? "Every ingredient is retired. Tick the box below to show them." : "No ingredients yet. Add what you keep: beans, milk, batter."}
        />
      </Card>
    );
  }
  return (
    <Card padding="lg" className="min-w-0">
      {/* Under 1280px it scrolls sideways, and a scroll box clips what hangs out of it: it is kept tall enough for a row's menu to open inside. */}
      <div className="min-h-96 overflow-x-auto xl:min-h-0 xl:overflow-visible">
        <table aria-label="Stock levels" className="w-full min-w-160 text-base">
          <thead>
            <tr className="border-b border-line text-left">
              <th scope="col" className={head}>
                Ingredient
              </th>
              <th scope="col" className={cx(head, "text-right")}>
                In stock
              </th>
              <th scope="col" className={cx(head, "text-right")}>
                Low at
              </th>
              <th scope="col" className={cx(head, "text-right")}>
                Unit cost
              </th>
              <th scope="col" className={cx(head, "pl-6")}>
                Stock level
              </th>
              <th scope="col" className={head}>
                Status
              </th>
              <th scope="col" className={head}>
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((it, row) => {
              const level = levelOf(it);
              const percent = levelPercent(it);
              return (
                <tr key={it.id} className="border-b border-line-soft last:border-b-0">
                  <td className="py-2">
                    <span className="flex items-center gap-2.5">
                      <span aria-hidden="true" className="flex size-9 shrink-0 items-center justify-center rounded-tight bg-raised-high text-fg-muted">
                        <PackageIcon size={17} />
                      </span>
                      <span className="min-w-0">
                        <strong className={cx("block", !it.active && "font-semibold text-fg-muted")}>{it.name}</strong>
                        {it.active && it.count_at_close && <span className="block text-sm text-fg-muted">Counted at the close</span>}
                      </span>
                    </span>
                  </td>
                  <td className={cx("py-2 text-right font-mono font-bold tabular-nums", it.on_hand < 0 && "text-danger")}>{formatQty(it.on_hand, it.unit)}</td>
                  <td className="py-2 text-right font-mono tabular-nums text-fg-2">
                    {it.low_level === 0 ? <span className="font-sans text-sm text-fg-muted">Not set</span> : formatQty(it.low_level, it.unit)}
                  </td>
                  <td className="py-2 text-right text-fg-2">
                    {it.cost.minor === 0 ? (
                      <span className="text-sm text-fg-muted">Not set</span>
                    ) : (
                      <span className="whitespace-nowrap">
                        <Money amount={it.cost} /> <span className="text-sm text-fg-muted">/ {costMeasures[it.unit].short}</span>
                      </span>
                    )}
                  </td>
                  <td className="w-40 py-2 pl-6">
                    {it.active && percent !== null && (
                      <span aria-hidden="true" className="block h-2.5 overflow-hidden rounded-pill bg-sunken">
                        <span className={cx("block h-full rounded-pill", fills[level])} style={{ width: `${percent}%` }} />
                      </span>
                    )}
                  </td>
                  <td className="py-2">
                    {it.active ? <StatusChip status={{ kind: "stock", status: level }} /> : <span className="text-sm text-fg-muted">Retired</span>}
                  </td>
                  <td className="py-2 text-right">
                    <span className="flex justify-end">
                      <RowMenu
                        name={it.name}
                        disabled={false}
                        up={row >= 3 && row >= rows.length - 3}
                        actions={[
                          ...(it.active
                            ? [
                                { label: "Receive", run: () => onAction(it, { kind: "move", move: "received" }) },
                                { label: "Log wastage", run: () => onAction(it, { kind: "move", move: "wasted" }) },
                                { label: "Count", run: () => onAction(it, { kind: "move", move: "counted" }) },
                              ]
                            : []),
                          { label: "History", run: () => onAction(it, { kind: "history" }) },
                          { label: "Edit", run: () => onAction(it, { kind: "edit" }) },
                        ]}
                      />
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </Card>
  );
}
