// A Z-report on screen (docs/design/screens/zreport.md), shared by the
// till and the back office. It shows the stored document as it was
// issued; every figure is the document's.

import type { ReactNode } from "react";
import type { Money as Amount, ZReportResponse } from "../api-types";
import { Meta, Money } from "../ui";
import { cx } from "../ui/cx";
import { GroupTable, Pair, Row, Section } from "./ZTables";
import { channelName, clockText, differenceWords, methodName, rateText, staffName } from "./zreport-logic";

interface ZReportViewProps {
  z: ZReportResponse;
  /** Beside the title: Print a copy, and the way back. */
  actions?: ReactNode;
}

const negate = (a: Amount): Amount => ({ ...a, minor: -a.minor });

export function ZReportView({ z, actions }: ZReportViewProps) {
  const r = z.report;
  const d = z.drawer;
  const who = (id: number) => staffName(z.names, id);
  const diff = differenceWords(d.variance);

  return (
    <article aria-label={`Z-report ${z.number}`} className="mx-auto flex w-full max-w-3xl flex-col gap-4">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Z-report {z.number}</h1>
          <Meta
            className="mt-1"
            items={[
              z.shift.trading_day,
              `Opened ${clockText(z.shift.opened_at)}, closed ${clockText(z.shift.closed_at)}`,
              `Closed by ${who(z.shift.closed_by)}`,
            ]}
          />
        </div>
        {actions && <div className="flex flex-wrap gap-tap-gap">{actions}</div>}
      </header>
      {z.corrects !== undefined && z.corrects > 0 && (
        <p role="note" className="rounded border border-warning px-4 py-3 text-base">
          <span className="font-semibold">Corrects Z-report {z.corrects}.</span> {z.reason}
        </p>
      )}

      <Section title="Sales">
        <Pair label="Bills" value={String(r.sales.bills)} />
        <Row label="Gross" amount={r.sales.gross} />
        <Row label="Discounts" count={r.discounts.count} amount={negate(r.sales.discounts)} />
        <Row label="Net" amount={r.sales.net} />
        <Row label="Taxable value" amount={r.sales.taxable} />
        <Row label="Tax" amount={r.sales.tax} />
        <Row label="Total" amount={r.sales.total} strong />
        {r.credit_notes.count > 0 && (
          <>
            <Row label="Less credit notes" count={r.credit_notes.count} amount={negate(r.credit_notes.amount)} />
            <Row label="Net sales" amount={r.sales.net_of_credits} strong />
          </>
        )}
        <Pair label="Invoices issued" value={String(r.invoices.count)} />
        {r.invoices.count > 0 && <Pair label="First to last" value={`${r.invoices.first ?? ""} to ${r.invoices.last ?? ""}`} />}
      </Section>

      {r.taxes.length > 0 && (
        <Section title="Tax">
          {r.taxes.map((t) => (
            <div key={`${t.name}-${t.rate}`} className="grid grid-cols-[1fr_auto_auto_auto] items-baseline gap-3 py-0.5 text-base text-fg-2">
              <span>
                {t.name} {rateText(t.rate)}
              </span>
              <span className="text-sm text-fg-muted">on</span>
              <Money amount={t.taxable} tone="muted" />
              <Money amount={t.amount} className="min-w-28 text-right" />
            </div>
          ))}
        </Section>
      )}

      <GroupTable title="By category" groups={r.by_category} unit="Units" />
      <GroupTable title="By channel" groups={r.by_channel} unit="Bills" name={(g) => channelName(g.key)} />
      <GroupTable title="By staff" groups={r.by_staff} unit="Bills" />
      <GroupTable title="By item" groups={r.by_item} unit="Units" fold={10} />

      <Section title="Payments">
        {r.payments.map((p) => (
          <div key={p.method} className="border-b border-line py-1 last:border-b-0">
            <Row label={methodName(p.method)} count={p.count} amount={p.net} strong />
            {p.voided.minor > 0 && <Row label="Voided" amount={negate(p.voided)} tone="muted" />}
            {p.refunded.minor > 0 && <Row label="Refunded" amount={negate(p.refunded)} tone="muted" />}
          </div>
        ))}
        {r.payments.length === 0 && <p className="text-base text-fg-muted">No payments in this shift.</p>}
        <Row label="Tips, apart from sales" amount={r.tips} />
        {r.balance.difference.minor !== 0 && <Row label="Paid for bills of other shifts" amount={r.balance.difference} />}
      </Section>

      <Section title="Exceptions">
        <Row label="Comps" count={r.comps.count} amount={r.comps.amount} />
        <Row label="Voids" count={r.voids.count} amount={r.voids.amount} />
        {r.removed.map((t) => (
          <Row key={t.staff_id} label={`Removed in error by ${who(t.staff_id)}`} count={t.count} amount={t.amount} />
        ))}
        <Row label="Credit notes" count={r.credit_notes.count} amount={r.credit_notes.amount} />
        <Row label="Refunds" count={r.refunds.count} amount={r.refunds.amount} />
        <Pair label="Reprints" value={String(r.reprints)} />
      </Section>

      <Section title="Cash drawer">
        <Row label="Float" amount={d.float} />
        <Row label="Cash in" amount={d.cash_in} />
        <Row label="Cash out" amount={negate(d.cash_out)} />
        <Row label="Cash taken" amount={d.sales.taken} />
        {d.sales.voided.minor > 0 && <Row label="Cash voided" amount={negate(d.sales.voided)} />}
        {d.sales.refunded.minor > 0 && <Row label="Cash refunded" amount={negate(d.sales.refunded)} />}
        <Row label="Expected" amount={d.expected} strong />
        <Row label={`Counted by ${who(d.counted_by)}`} amount={d.counted} />
        <p
          className={cx(
            "border-t border-line pt-2 text-lg font-semibold",
            diff.tone === "danger" && "text-danger",
            diff.tone === "warning" && "text-warning",
            diff.tone === "success" && "text-success",
          )}
        >
          {diff.text}
        </p>
        {d.reason && <blockquote className="border-l-2 border-line pl-4 text-base text-fg-2">{d.reason}</blockquote>}
      </Section>
    </article>
  );
}
