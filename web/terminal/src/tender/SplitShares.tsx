// A split in use (D08a): the same guests' cards, now saying who has paid, with
// Collect on each that has not. Collect goes to the payment step for that
// share alone; a share is paid whole, in one payment. And the strip the
// payment step carries while a share is being paid.

import { UsersIcon } from "lucide-react";
import type { Money as Amount, OrderResponse, SplitResponse } from "../../../shared/api-types";
import { Button, Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { payerLabel, splitNames } from "./split-logic";
import { ColumnHeading, GuestCard, SplitFrame, modeTile, modeTileOn, type ShareLine } from "./SplitParts";

const staleText = "The bill changed after it was split. Change the split and make it again.";

interface SplitSharesProps {
  order: OrderResponse;
  title: string;
  split: SplitResponse;
  amountsKeyed: number;
  paid: ReadonlySet<number>;
  /** The share the next payment is for. */
  current: number | null;
  /** The bill moved since the split: it must be made again. */
  stale: boolean;
  disabled: boolean;
  onCollect: (payer: number) => void;
  onClear: () => void;
  onBack: () => void;
}

export function SplitShares({ order, title, split, amountsKeyed, paid, current, stale, disabled, onCollect, onClear, onBack }: SplitSharesProps) {
  const money = (minor: number): Amount => ({ minor, currency: split.total.currency });
  const taken = split.shares.filter((s) => paid.has(s.payer)).reduce((sum, s) => sum + s.amount.minor, 0);
  // How many shares a line is in: its own figure is shown only where one guest pays it.
  const sharedBy = (lineId: number) => split.shares.filter((s) => s.line_ids.includes(lineId)).length;

  const linesOf = (lineIds: readonly number[]): ShareLine[] =>
    lineIds.flatMap((id) => {
      const line = order.lines.find((l) => l.id === id);
      if (!line) return [];
      const by = sharedBy(id);
      return [{ id, label: `${line.qty} × ${line.name}`, amount: by === 1 ? line.due : null, sharedBy: by }];
    });

  const aside = (
    <>
      <ColumnHeading>Split mode</ColumnHeading>
      <p className={cx(modeTile, modeTileOn)}>{splitNames[split.kind]}</p>
      <div aria-hidden="true" className="my-5 h-px bg-line" />
      <ColumnHeading>Shares</ColumnHeading>
      <p className="text-base text-fg">
        {paid.size} of {split.shares.length} paid
      </p>
      <p className="mt-1 text-sm text-fg-muted">A share is paid whole, in one payment.</p>
      {stale && (
        <p role="alert" className="mt-4 rounded bg-danger-tint p-3 text-sm text-danger">
          {staleText}
        </p>
      )}
      <Button disabled={disabled} onClick={onClear} className="mt-5 w-full">
        Change the split
      </Button>
    </>
  );

  const footer = (
    <>
      <p className="min-w-0 flex-1 text-base text-fg-muted" aria-live="polite">
        Paid <Money amount={money(taken)} className="font-bold text-fg" /> of <Money amount={split.total} />
      </p>
      <Button onClick={onBack} disabled={disabled} size="charge" className="w-40">
        Back
      </Button>
    </>
  );

  return (
    <SplitFrame heading={`Split bill · ${title}`} totalLabel="Still owed" total={order.totals.balance_due} aside={aside} footer={footer}>
      {split.shares.map((s) => {
        const done = paid.has(s.payer);
        const name = payerLabel(split.kind, s.payer, amountsKeyed);
        return (
          <GuestCard key={s.payer} seat={s.payer + 1} name={name} paid={done} lines={linesOf(s.line_ids)} due={s.amount}>
            {!done && (
              // One green button: the share that is next. The others can still be taken first.
              <Button variant={s.payer === current ? "primary" : "secondary"} disabled={disabled || stale} onClick={() => onCollect(s.payer)}>
                Collect <Money amount={s.amount} />
              </Button>
            )}
          </GuestCard>
        );
      })}
    </SplitFrame>
  );
}

interface ShareStripProps {
  split: SplitResponse;
  amountsKeyed: number;
  paid: ReadonlySet<number>;
  current: number | null;
  stale: boolean;
  disabled: boolean;
  /** Back to the guests' cards. */
  onShares: () => void;
}

/** On the payment step: whose share this payment is. */
export function ShareStrip({ split, amountsKeyed, paid, current, stale, disabled, onShares }: ShareStripProps) {
  return (
    <section aria-label="Split" className="flex flex-wrap items-center gap-3 rounded-card border border-line bg-raised px-4 py-2">
      <UsersIcon aria-hidden="true" size={19} strokeWidth={1.9} className="shrink-0 text-fg-muted" />
      {stale ? (
        <p role="alert" className="min-w-0 flex-1 text-base text-danger">
          {staleText}
        </p>
      ) : (
        <p className="min-w-0 flex-1 text-base text-fg-2">
          {current !== null && <strong className="text-fg">{payerLabel(split.kind, current, amountsKeyed)}</strong>}
          {current !== null && " · "}
          {paid.size} of {split.shares.length} shares paid
        </p>
      )}
      <Button variant="ghost" disabled={disabled} onClick={onShares}>
        All shares
      </Button>
    </section>
  );
}
