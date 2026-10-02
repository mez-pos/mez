// The sections of the Sales reports screen (backoffice-reports.md): the
// headline figures, sales over time, and the payment and channel mix.
// Every figure is the server's; see sales-logic.ts for the little that is
// worked out here.

import type { ReactNode } from "react";
import type { Group, Money as Amount, Payment, SalesPoint, SalesReport } from "../../../shared/api-types";
import { Card, EmptyState, InboxIcon, Money } from "../../../shared/ui";
import { DataTable, type Column } from "../../../shared/ui/DataTable";
import { channelName, methodName } from "../../../shared/zreport/zreport-logic";
import { dayText } from "../audit/audit-logic";
import { averageBill, barHeights, periodText, shares } from "./sales-logic";

export const count = (n: number) => <span className="font-mono tabular-nums">{n}</span>;

/** A titled part of the screen. `csv` is its Download CSV button, or null for a part with no file. */
export function Section({ title, csv, children }: { title: string; csv: ReactNode; children: ReactNode }) {
  return (
    <section aria-label={title} className="flex min-w-0 flex-col gap-3">
      <div className="flex min-h-9 flex-wrap items-center justify-between gap-4">
        <h2 className="font-serif text-xl">{title}</h2>
        {csv}
      </div>
      {children}
    </section>
  );
}

function Figure({ label, amount, note }: { label: string; amount: Amount; note?: string }) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-line-soft py-1.5 last:border-b-0">
      <dt className="text-sm text-fg-2">
        {label}
        {note && <span className="ml-2 font-mono text-fg-muted">{note}</span>}
      </dt>
      <dd>
        <Money amount={amount} />
      </dd>
    </div>
  );
}

export function Headline({ report, csv }: { report: SalesReport; csv: ReactNode }) {
  const t = report.totals;
  const average = averageBill(t.sales.total, t.sales.bills);
  return (
    <Section title="The range" csv={csv}>
      <Card padding="lg" className="grid gap-6 md:grid-cols-2">
        <div>
          <p className="text-sm font-semibold text-fg-2">Sales</p>
          <Money amount={t.sales.total} variant="total" />
          <p className="mt-1 text-sm text-fg-3">
            {count(t.sales.bills)} {t.sales.bills === 1 ? "bill" : "bills"}
            {average && (
              <>
                {" · average "}
                <Money amount={average} />
              </>
            )}
          </p>
        </div>
        <dl>
          <Figure label="Gross" amount={t.sales.gross} />
          <Figure label="Discounts" amount={t.sales.discounts} />
          <Figure label="Net sales" amount={t.sales.net} />
          <Figure label="Tax" amount={t.sales.tax} />
          <Figure label="Credit notes" note={String(t.credit_notes.count)} amount={t.credit_notes.amount} />
          <Figure label="Sales after credit notes" amount={t.sales.net_of_credits} />
          <Figure label="Refunds" note={String(t.refunds.count)} amount={t.refunds.amount} />
          <Figure label="Tips" amount={t.tips} />
        </dl>
      </Card>
    </Section>
  );
}

export function OverTime({ report, csv }: { report: SalesReport; csv: ReactNode }) {
  const heights = barHeights(report.series);
  const quiet = heights.every((h) => h === 0);
  const first = report.series[0];
  const last = report.series[report.series.length - 1];
  const columns: Column<SalesPoint>[] = [
    { id: "period", header: "Period", cell: (p) => <span className="whitespace-nowrap">{periodText(p)}</span> },
    { id: "bills", header: "Bills", align: "right", cell: (p) => count(p.sales.bills) },
    { id: "net", header: "Net", align: "right", cell: (p) => <Money amount={p.sales.net} /> },
    { id: "tax", header: "Tax", align: "right", cell: (p) => <Money amount={p.sales.tax} /> },
    { id: "total", header: "Total", align: "right", cell: (p) => <Money amount={p.sales.total} /> },
  ];
  return (
    <Section title="Sales over time" csv={csv}>
      {quiet ? (
        <Card>
          <EmptyState icon={<InboxIcon />} line={`No sales between ${dayText(report.from)} and ${dayText(report.to)}.`} />
        </Card>
      ) : (
        // A picture of the table below, so hidden from screen readers.
        <Card aria-hidden>
          <div className="flex h-32 items-end gap-0.5 border-b border-line">
            {report.series.map((p, i) => (
              <div key={p.start} className="flex h-full min-w-0 flex-1 items-end">
                {/* The one inline style: a height that is data. */}
                <div className="min-h-px w-full rounded-tight bg-accent" style={{ height: `${heights[i] ?? 0}%` }} />
              </div>
            ))}
          </div>
          {first && last && (
            <div className="mt-1.5 flex justify-between text-meta uppercase text-fg-faint">
              <span>{periodText(first)}</span>
              {last !== first && <span>{periodText(last)}</span>}
            </div>
          )}
        </Card>
      )}
      <div className="max-h-96 overflow-y-auto">
        <DataTable label="Sales by period" columns={columns} rows={report.series} rowKey={(p) => p.start} />
      </div>
    </Section>
  );
}

/** A share as a green bar and a whole percentage. */
export function Share({ percent }: { percent: number | undefined }) {
  if (percent === undefined) return null;
  return (
    <span className="flex items-center justify-end gap-2">
      <span aria-hidden className="h-2 w-16 overflow-hidden rounded-pill bg-sunken">
        <span className="block h-full rounded-pill bg-accent" style={{ width: `${percent}%` }} />
      </span>
      <span className="w-10 font-mono tabular-nums">{percent}%</span>
    </span>
  );
}

export function Mixes({ report, paymentsCsv, channelsCsv }: { report: SalesReport; paymentsCsv: ReactNode; channelsCsv: ReactNode }) {
  const payments = report.totals.payments;
  const channels = report.totals.by_channel;
  const paid = shares(payments.map((p) => p.net.minor));
  const came = shares(channels.map((c) => c.total.minor));
  const paymentColumns: Column<Payment>[] = [
    { id: "method", header: "Method", cell: (p) => methodName(p.method) },
    { id: "tenders", header: "Tenders", align: "right", cell: (p) => count(p.count) },
    { id: "net", header: "Net", align: "right", cell: (p) => <Money amount={p.net} /> },
    { id: "share", header: "Share", align: "right", cell: (p) => <Share percent={paid?.[payments.indexOf(p)]} /> },
  ];
  const channelColumns: Column<Group>[] = [
    { id: "channel", header: "Channel", cell: (c) => channelName(c.key) },
    { id: "bills", header: "Bills", align: "right", cell: (c) => count(c.qty) },
    { id: "total", header: "Total", align: "right", cell: (c) => <Money amount={c.total} /> },
    { id: "share", header: "Share", align: "right", cell: (c) => <Share percent={came?.[channels.indexOf(c)]} /> },
  ];
  return (
    <div className="grid gap-6 xl:grid-cols-2">
      <Section title="How it was paid" csv={paymentsCsv}>
        <DataTable
          label="Payments by method"
          columns={paymentColumns}
          rows={payments}
          rowKey={(p) => p.method}
          empty={<EmptyState icon={<InboxIcon />} line="Nothing was paid." />}
        />
      </Section>
      <Section title="Where it came from" csv={channelsCsv}>
        <DataTable label="Sales by channel" columns={channelColumns} rows={channels} rowKey={(c) => c.key} empty={<EmptyState icon={<InboxIcon />} line="No bills." />} />
      </Section>
    </div>
  );
}
