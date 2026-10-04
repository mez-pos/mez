// The ground of the shift screens (D02): a card for the task on the left, and
// the shift's own details on the right, with a leaf faint behind them. Below
// 1280px the right column drops under the card.

import { CircleAlertIcon, LeafIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Card } from "../../../shared/ui";

export function ShiftStage({ main, aside }: { main: ReactNode; aside: ReactNode }) {
  return (
    <div className="relative flex min-w-0 flex-1 overflow-hidden">
      <LeafIcon aria-hidden="true" strokeWidth={1} className="pointer-events-none absolute -bottom-10 -left-10 size-72 text-accent opacity-10" />
      <div className="relative flex min-w-0 flex-1 justify-center overflow-y-auto px-6 py-8 xl:px-11 xl:py-11">
        <div className="flex w-full max-w-300 flex-col gap-7 xl:flex-row xl:items-start">
          <div className="min-w-0 flex-1 rounded-sheet border border-line bg-raised p-6 shadow-card sm:p-9">{main}</div>
          <aside className="flex w-full flex-col gap-4 xl:w-105 xl:shrink-0">{aside}</aside>
        </div>
      </div>
    </div>
  );
}

export interface DetailRow {
  label: string;
  value: string;
}

/** "Tonight's shift": who and where, as label and value pairs. */
export function ShiftDetailsCard({ rows }: { rows: readonly DetailRow[] }) {
  return (
    <Card padding="lg" className="rounded-card">
      <h2 className="mb-4 text-meta font-bold uppercase text-fg-muted">{"Tonight's shift"}</h2>
      <dl className="grid gap-3 text-base">
        {rows.map((row) => (
          <div key={row.label} className="flex justify-between gap-4">
            <dt className="text-fg-2">{row.label}</dt>
            <dd className="text-right font-semibold">{row.value}</dd>
          </div>
        ))}
      </dl>
    </Card>
  );
}

/** What is sold out before service, with a way to the menu for someone who may change it. */
export function SoldOutCard({ title, body, menuHref }: { title: string; body: string; menuHref: string | null }) {
  return (
    <section aria-label="Before service" className="rounded-card border border-chip-warning-border bg-chip-warning-bg p-5 text-chip-warning-fg">
      <div className="flex gap-3">
        <CircleAlertIcon aria-hidden="true" size={20} className="mt-0.5 shrink-0" />
        <div>
          <h2 className="text-base font-bold">{title}</h2>
          <p className="mt-1 text-sm leading-normal text-fg">{body}</p>
        </div>
      </div>
      {menuHref && (
        <a
          href={menuHref}
          className="mt-3.5 inline-flex min-h-tap w-full items-center justify-center rounded-pill border border-chip-warning-border bg-raised px-5 text-base font-bold outline-offset-2 outline-attention focus-visible:outline-3"
        >
          Review the menu
        </a>
      )}
    </section>
  );
}
