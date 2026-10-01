// What is low, on the dashboard, for whoever keeps the stock
// (backoffice-stock.md, "Low stock"). Not shown to anyone without
// stock.manage: they could not read it, and could do nothing about it.

import { Link } from "wouter";
import type { Permission } from "../../../shared/api-types";
import { formatQty } from "../../../shared/stock-qty";
import { Card } from "../../../shared/ui";
import { useStockItems } from "./use-stock";

export function LowStockCard({ held }: { held: readonly Permission[] }) {
  const can = held.includes("stock.manage");
  const stock = useStockItems(can);
  if (!can) return null;
  const low = stock.data?.items.filter((it) => it.low) ?? [];

  return (
    <section aria-labelledby="low-stock" className="mt-8">
      <h2 id="low-stock" className="text-meta font-semibold uppercase text-fg-faint">
        Low stock
      </h2>
      <Card className="mt-2" padding="none">
        {stock.isPending && <p className="px-4 py-3 text-sm text-fg-muted">Reading the stock</p>}
        {stock.data && low.length === 0 && <p className="px-4 py-3 text-sm text-fg-2">Nothing is low.</p>}
        {low.length > 0 && (
          <ul>
            {low.map((it) => (
              <li key={it.id} className="flex min-h-tap items-center justify-between gap-4 border-b border-line-soft px-4 py-2 text-sm last:border-b-0">
                <span>
                  <span className="font-semibold">{it.name}</span>
                  <span className="ml-2 text-meta font-semibold uppercase text-warning">Low</span>
                </span>
                <span className="text-right">
                  <span className="font-mono tabular-nums">{formatQty(it.on_hand, it.unit)}</span>
                  <span className="text-fg-3"> of {formatQty(it.low_level, it.unit)}</span>
                </span>
              </li>
            ))}
          </ul>
        )}
        {stock.data && (
          <Link href="/stock" className="flex min-h-tap items-center border-t border-line-soft px-4 text-sm font-semibold underline">
            Go to stock
          </Link>
        )}
      </Card>
    </section>
  );
}
