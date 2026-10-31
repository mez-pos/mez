// The table list (D06's table list): the same tables as the floor plan, as
// rows, for a floor too big to read as a picture or a screen too small for it.
// Above them, how the floor stands: a bar for each state (D06's "Table
// availability"). A row picks its table for the panel, as a tile does.

import type { AreaResponse, LiveFloorResponse, LiveTableResponse } from "../../../shared/api-types";
import { Money, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { tableStatuses } from "../../../shared/ui/status";
import { availability, listRows, tileText } from "./floor-logic";
import { WantsToOrder } from "./FloorPlan";
import { tableTone } from "./tones";

interface TableListProps {
  data: LiveFloorResponse;
  areas: readonly AreaResponse[];
  now: number;
  names: ReadonlyMap<number, string>;
  picked: number | null;
  onTap: (t: LiveTableResponse) => void;
  /** The tables a guest's phone has asked to order at, unanswered. */
  wanting?: ReadonlySet<number> | undefined;
}

// Six columns need about 640px. Below xl, Area goes and the time sits under
// the guests, so Balance, the column a cashier is looking for, never leaves.
const columns =
  "grid grid-cols-[minmax(4rem,1fr)_minmax(6rem,1.6fr)_7.5rem_6rem] items-center gap-4 xl:grid-cols-[minmax(4rem,1fr)_minmax(5rem,1.2fr)_minmax(6rem,1.4fr)_7.5rem_4rem_6rem]";

export function TableList({ data, areas, now, names, picked, onTap, wanting }: TableListProps) {
  const rows = listRows(data, areas);
  const areaNames = new Map(data.areas.map((a) => [a.id, a.name]));
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      <Availability tables={rows} />
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-card border border-line bg-raised">
        <div className={cx(columns, "border-b border-line px-5 py-3.5 text-meta uppercase text-fg-faint short:py-3")}>
          <span>Table</span>
          <span className="max-xl:hidden">Area</span>
          <span>
            Guests<span className="xl:hidden"> · Time</span>
          </span>
          <span>Status</span>
          <span className="max-xl:hidden">Time</span>
          <span className="text-right">Balance</span>
        </div>
        {rows.length === 0 ? (
          <p className="px-5 py-8 text-base text-fg-muted">No tables in this area.</p>
        ) : (
          <ul aria-label="Tables" className="min-h-0 flex-1 overflow-y-auto">
            {rows.map((t) => {
              const text = tileText(t, now, names);
              return (
                <li key={t.id} className="border-b border-line-soft last:border-b-0">
                  <button
                    type="button"
                    onClick={() => onTap(t)}
                    aria-pressed={picked === t.id}
                    className={cx(
                      columns,
                      "min-h-14 w-full px-5 py-2 text-left text-base outline-offset-[-3px] outline-attention focus-visible:outline-3 short:min-h-tap short:py-1.5",
                      picked === t.id ? "bg-accent-tint" : "hover:bg-raised-high",
                    )}
                  >
                    <span className="truncate font-serif text-xl">{t.name}</span>
                    <span className="truncate text-fg-2 max-xl:hidden">{areaNames.get(t.area_id) ?? ""}</span>
                    <span className="flex min-w-0 flex-col">
                      <span className="truncate">{text.people}</span>
                      {text.time && <span className="font-mono text-sm tabular-nums text-fg-2 xl:hidden">{text.time}</span>}
                    </span>
                    <span>{wanting?.has(t.id) ? <WantsToOrder /> : <StatusChip status={{ kind: "table", status: t.status }} />}</span>
                    <span className="font-mono tabular-nums text-fg-2 max-xl:hidden">{text.time ?? ""}</span>
                    <span className="text-right">{t.balance && t.balance.minor !== 0 && <Money amount={t.balance} />}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}

function Availability({ tables }: { tables: readonly LiveTableResponse[] }) {
  return (
    <section aria-label="Table availability" className="rounded-card border border-line bg-raised px-5 py-4 short:py-3">
      <h2 className="font-serif text-xl">Table availability</h2>
      <ul className="mt-3 grid grid-cols-5 gap-4 short:mt-2">
        {availability(tables).map((a) => (
          <li key={a.status} className="flex min-w-0 flex-col gap-2">
            <span className="flex items-baseline justify-between gap-2 text-sm text-fg-2">
              <span className="truncate">{tableStatuses[a.status].word}</span>
              <strong className="font-mono text-base tabular-nums text-fg">{a.count}</strong>
            </span>
            <span aria-hidden="true" className="h-2.5 rounded-pill bg-sunken">
              <span className={cx("block h-2.5 rounded-pill", tableTone(a.status).dot)} style={{ width: `${a.share}%` }} />
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
