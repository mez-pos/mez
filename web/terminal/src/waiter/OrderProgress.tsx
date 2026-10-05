// "Order Status" on the table's screen (T03): the order's way through the
// kitchen as steps, each done, where it is now, or not reached. The dot's colour
// goes with the step's words; the words carry it.

import type { OrderResponse } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";
import { orderProgress, timeText, type ProgressStep } from "./table-logic";

const dots: Readonly<Record<ProgressStep["state"], string>> = {
  done: "bg-accent",
  now: "bg-highlight ring-4 ring-chip-warning-bg",
  todo: "border-2 border-line-strong bg-sunken",
};

export function OrderProgress({ order, timezone }: { order: OrderResponse; timezone: string | null }) {
  return (
    <section aria-label="Order status" className="rounded-card border border-line bg-raised p-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-serif text-lg">Order Status</h2>
        <span className="inline-flex min-h-7 items-center gap-1.5 rounded-pill bg-chip-success-bg px-3 text-sm font-bold text-chip-success-fg">
          <span aria-hidden="true" className="size-2 rounded-round bg-chip-success-fg" />
          Live
        </span>
      </div>
      <ol className="mt-3.5 grid gap-3.5">
        {orderProgress(order).map((step) => (
          <li key={step.key} className="flex gap-3">
            <span aria-hidden="true" className={cx("mt-0.5 size-4.5 shrink-0 rounded-round", dots[step.state])} />
            <div>
              <p className={cx("text-base font-bold", step.state === "todo" && "text-fg-muted")}>{step.label}</p>
              <p className="text-sm text-fg-muted">
                {step.at && <span className="font-mono tabular-nums">{timeText(step.at, timezone)} · </span>}
                {step.detail}
              </p>
            </div>
          </li>
        ))}
      </ol>
    </section>
  );
}
