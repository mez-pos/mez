// The closing stock count in the close-of-day screen (terminal-close.md,
// ADR-114): after the drawer, the items marked count at close, counted
// blind. Once it is in, what each count moved is shown, as the drawer's
// variance is.

import { useState } from "react";
import type { ClosingStockCount, ClosingStockSheet } from "../../../shared/api-types";
import { formatDelta, formatQty } from "../../../shared/stock-qty";
import { Button } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { toCounts } from "./closing-stock-logic";

interface ClosingStockProps {
  sheet: ClosingStockSheet;
  onRecord: (counts: ClosingStockCount[]) => Promise<unknown>;
}

export function ClosingStock({ sheet, onRecord }: ClosingStockProps) {
  const [typed, setTyped] = useState<Record<number, string>>({});
  const [problem, setProblem] = useState<{ itemId: number | null; message: string } | null>(null);
  const [busy, setBusy] = useState(false);

  if (sheet.done) {
    return (
      <section aria-label="Closing stock" className="flex flex-col gap-3">
        <h2 className="font-serif text-xl">Closing stock</h2>
        <dl className="flex flex-col">
          {sheet.results.map((r) => (
            <div key={r.stock_item_id} className="flex items-baseline justify-between gap-4 py-1 text-base">
              <dt className="text-fg-2">{r.name}</dt>
              <dd className="flex gap-4 font-mono tabular-nums">
                <span>{formatQty(r.counted, r.unit)}</span>
                <span className={cx("w-28 text-right", r.delta < 0 ? "text-danger" : "text-fg-muted")}>{formatDelta(r.delta, r.unit)}</span>
              </dd>
            </div>
          ))}
        </dl>
        <p className="text-sm text-fg-muted">The change is the count against what the stock record said.</p>
      </section>
    );
  }
  if (sheet.items.length === 0) return null;

  const save = async () => {
    const built = toCounts(sheet.items, typed);
    if ("problem" in built) {
      setProblem(built.problem);
      return;
    }
    setBusy(true);
    setProblem(null);
    try {
      await onRecord(built.counts);
    } catch (err) {
      setProblem({ itemId: null, message: err instanceof Error ? err.message : String(err) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <section aria-label="Closing stock" className="flex flex-col gap-4">
      <div>
        <h2 className="font-serif text-xl">Count the stock</h2>
        <p className="mt-1 text-base text-fg-2">How much of each is there now? The record is shown once the count is in.</p>
      </div>
      <form
        className="flex flex-col gap-2.5"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        {sheet.items.map((it) => (
          <div key={it.id} className="flex flex-col gap-1">
            <label className="flex items-center justify-between gap-4 text-base">
              <span>{it.name}</span>
              <span className="flex items-center gap-2">
                <input
                  aria-label={`${it.name}, in ${it.unit}`}
                  className="min-h-tap w-36 rounded border border-line-strong bg-raised-high px-3 text-right font-mono text-base tabular-nums text-fg outline-offset-2 outline-line-focus focus-visible:outline-2"
                  inputMode="decimal"
                  autoComplete="off"
                  value={typed[it.id] ?? ""}
                  disabled={busy}
                  onChange={(e) => {
                    setTyped((t) => ({ ...t, [it.id]: e.target.value }));
                    setProblem(null);
                  }}
                />
                <span className="w-10 text-sm text-fg-muted">{it.unit}</span>
              </span>
            </label>
            {problem?.itemId === it.id && (
              <p role="alert" className="text-sm text-danger">
                {problem.message}
              </p>
            )}
          </div>
        ))}
        {problem && problem.itemId === null && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
        <Button type="submit" disabled={busy} className="self-start">
          {busy ? "Saving" : "Save the stock count"}
        </Button>
      </form>
    </section>
  );
}
