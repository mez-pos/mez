// An order's event stream, in order (docs/design/components.md). Every event is
// shown, voids and refunds included: the history of an order is the order.

import type { OrderEvent } from "../api-types";
import { cx } from "./cx";
import { Money } from "./Money";
import type { ChipTone } from "./status";
import { timelineEntries, type DeviceName, type StaffName } from "./timeline-entries";

export interface TimelineProps {
  events: readonly OrderEvent[];
  staffName?: StaffName;
  deviceName?: DeviceName;
  /** The venue's time zone, once settings carry it; the device's until then. */
  timeZone?: string;
  className?: string;
}

const dots: Record<ChipTone, string> = {
  neutral: "text-fg-faint",
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  accent: "text-attention",
  info: "text-info",
};

export function Timeline({ events, staffName, deviceName, timeZone, className }: TimelineProps) {
  const time = new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit", ...(timeZone ? { timeZone } : {}) });
  const entries = timelineEntries(events, staffName, deviceName);

  return (
    <ol className={cx("flex flex-col", className)}>
      {entries.map((entry) => (
        <li key={entry.seq} className="grid grid-cols-[auto_auto_1fr_auto] items-baseline gap-x-3 border-b border-line-soft py-2.5 last:border-b-0">
          <time dateTime={entry.at} className="font-mono text-sm tabular-nums text-fg-muted">
            {time.format(new Date(entry.at))}
          </time>
          <span aria-hidden="true" className={cx("size-2 self-start mt-2 rounded-round bg-current", dots[entry.tone])} />
          <div className="min-w-0">
            <p className={cx("text-base", entry.tone === "danger" ? "text-danger" : "text-fg")}>{entry.title}</p>
            {entry.detail && <p className="text-sm text-fg-2">{entry.detail}</p>}
            <p className="text-sm text-fg-faint">
              {entry.by}
              {entry.device && ` · ${entry.device}`}
            </p>
          </div>
          {entry.amount ? <Money amount={entry.amount} tone={entry.tone === "danger" ? "danger" : "default"} /> : <span />}
        </li>
      ))}
    </ol>
  );
}
