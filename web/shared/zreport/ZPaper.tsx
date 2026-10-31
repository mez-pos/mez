// The Z-report as a slip of paper (D23): the day's figures in four blocks
// between dashed rules, as the printout reads. It is a summary of the report
// beside it: every figure is the stored report's, but for the average order,
// which is its total over its bills.
//
// The design sets the slip in a monospace face. The sheet has none: figures
// are Archivo, the numeral face, which lines them up (tokens.md).

import type { ReactNode } from "react";
import type { Money as Amount, ZReportResponse } from "../api-types";
import { averageMoney, formatMoney } from "../money";
import { Card } from "../ui";
import { cx } from "../ui/cx";
import { useVenueClock } from "../use-venue";
import { carriedText, creditLines, differenceWords, methodName, rateText, staffName, stampText } from "./zreport-logic";

interface ZPaperProps {
  z: ZReportResponse;
  /** The venue's name for the slip's head, when it is known. */
  venueName: string | null;
  className?: string;
}

const minus = (a: Amount): string => formatMoney({ ...a, minor: -Math.abs(a.minor) });

export function ZPaper({ z, venueName, className }: ZPaperProps) {
  const { timezone } = useVenueClock();
  const r = z.report;
  const d = z.drawer;
  const average = averageMoney(r.sales.total, r.sales.bills);
  // A report issued before guests were counted has no figure for them (ADR-184).
  const covers: unknown = r.sales.covers;
  const credits = creditLines(r);
  const carried = carriedText(z.carried);

  return (
    <Card padding="lg" role="region" aria-label={`Z-report ${z.number}, in brief`} className={cx("font-mono text-sm tabular-nums", className)}>
      <div className="border-b border-dashed border-line-strong pb-3.5 text-center">
        <p className="text-base font-bold uppercase tracking-widest">Z-report {z.number}</p>
        {venueName && <p className="mt-1 text-fg-muted">{venueName}</p>}
        <p className="mt-1 text-fg-muted">{stampText(z.shift.trading_day, z.shift.closed_at, timezone)}</p>
      </div>

      <Block>
        <Line label="Gross sales" value={formatMoney(r.sales.gross)} />
        <Line label="Discounts" value={minus(r.sales.discounts)} />
        <Line label="Net" value={formatMoney(r.sales.net)} />
        {r.taxes.map((t) => (
          <Line key={`${t.name}-${t.rate}`} label={`${t.name} ${rateText(t.rate)}`} value={formatMoney(t.amount)} />
        ))}
        <Line label="Total" value={formatMoney(r.sales.total)} strong />
        {credits && (
          <>
            <Line label={credits.label} value={formatMoney(credits.less)} />
            <Line label="Net sales" value={formatMoney(credits.net)} strong />
          </>
        )}
        <Line label="Tips collected" value={formatMoney(r.tips)} />
      </Block>

      <Block>
        {r.payments.length === 0 && <p className="text-fg-muted">No payments</p>}
        {r.payments.map((p) => (
          <Line key={p.method} label={methodName(p.method)} value={formatMoney(p.net)} />
        ))}
      </Block>

      <Block>
        <Line label="Orders" value={String(r.sales.bills)} />
        {typeof covers === "number" && <Line label="Covers" value={String(covers)} />}
        {average && <Line label="Avg. order" value={formatMoney(average)} />}
        <Line label="Refunds" value={`${r.refunds.count} · ${formatMoney(r.refunds.amount)}`} />
        <Line label="Voids" value={`${r.voids.count} · ${formatMoney(r.voids.amount)}`} />
        {carried && <Line label="Open orders carried" value={carried} />}
      </Block>

      <Block last>
        <Line label="Expected drawer" value={formatMoney(d.expected)} />
        <Line label="Counted" value={formatMoney(d.counted)} />
        <Line label="Variance" value={differenceWords(d.variance).text} strong />
      </Block>

      <p className="border-t border-dashed border-line-strong pt-3.5 text-center text-meta uppercase text-fg-muted">
        Closed by {staffName(z.names, z.shift.closed_by)}
      </p>
    </Card>
  );
}

function Block({ children, last = false }: { children: ReactNode; last?: boolean }) {
  return <div className={cx("grid gap-2 py-3.5", !last && "border-b border-dashed border-line-strong")}>{children}</div>;
}

function Line({ label, value, strong = false }: { label: string; value: string; strong?: boolean }) {
  return (
    <p className={cx("flex justify-between gap-3", strong && "font-bold")}>
      <span className="font-sans">{label}</span>
      <span className="whitespace-nowrap">{value}</span>
    </p>
  );
}
