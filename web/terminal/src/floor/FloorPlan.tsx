// The floor plan (D05): the tables of an area where the back office drew them,
// each a tile with its name, who is at it, its state in colour and a word, and
// how long and how much. A tap picks the table for the panel beside it; while
// moving or joining a party, a tap is a step of that instead (ADR-088).

import { BellRingIcon, UsersIcon } from "lucide-react";
import type { AreaResponse, LiveFloorResponse, LiveTableResponse } from "../../../shared/api-types";
import { Money, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { selectedRing } from "../../../shared/ui/focus";
import { paidTable, tableChip, tablePaid, tableStatuses } from "../../../shared/ui/status";
import { packed, tileText, tilesOf, turnOrder } from "./floor-logic";
import { tableTone } from "./tones";

interface FloorPlanProps {
  data: LiveFloorResponse;
  areas: readonly AreaResponse[];
  now: number;
  names: ReadonlyMap<number, string>;
  /** The table in the panel, or the party's table while moving or joining. */
  picked: number | null;
  onTap: (t: LiveTableResponse) => void;
  /** The tables a guest's phone has asked to order at, unanswered. */
  wanting?: ReadonlySet<number> | undefined;
}

export function FloorPlan({ data, areas, now, names, picked, onTap, wanting }: FloorPlanProps) {
  return (
    <div className="min-h-0 flex-1 overflow-auto rounded-card border border-line-strong bg-sunken p-3 2xl:p-6">
      <div className="flex min-h-full flex-col gap-6">
        {areas.map((area) => (
          <section key={area.id} aria-label={area.name} className="flex flex-1 flex-col gap-3">
            {areas.length > 1 && <h2 className="text-meta uppercase text-fg-faint">{area.name}</h2>}
            <AreaGrid data={data} areaId={area.id} now={now} names={names} picked={picked} onTap={onTap} wanting={wanting} />
          </section>
        ))}
      </div>
    </div>
  );
}

interface AreaGridProps extends Omit<FloorPlanProps, "areas"> {
  areaId: number;
}

function AreaGrid({ data, areaId, now, names, picked, onTap, wanting }: AreaGridProps) {
  const rows = packed(tilesOf(data, areaId));
  if (rows.length === 0) return <p className="text-base text-fg-muted">No tables in this area.</p>;
  const columns = rows[0]?.length ?? 0;
  return (
    <ul
      aria-label="Tables"
      className="grid flex-1 gap-2 2xl:gap-5"
      // The tiles share the room they are given, as drawn: no narrower than a
      // chip, no shorter than the lines a tile holds (a floor too big for that
      // scrolls), and no taller than a tile should be when the area has only a
      // row or two.
      style={{
        gridTemplateColumns: `repeat(${columns}, minmax(5.75rem, 1fr))`,
        gridTemplateRows: `repeat(${rows.length}, minmax(min-content, 1fr))`,
        maxHeight: `${rows.length * 17}rem`,
      }}
    >
      {rows.flatMap((row, y) =>
        row.map((t, x) =>
          t === null ? (
            <li key={`${x},${y}`} aria-hidden="true" />
          ) : (
            <li key={t.id}>
              <TableTile table={t} now={now} names={names} picked={picked === t.id} wants={wanting?.has(t.id) ?? false} onTap={() => onTap(t)} />
            </li>
          ),
        ),
      )}
    </ul>
  );
}

interface TableTileProps {
  table: LiveTableResponse;
  now: number;
  names: ReadonlyMap<number, string>;
  picked: boolean;
  wants: boolean;
  onTap: () => void;
}

function TableTile({ table: t, now, names, picked, wants, onTap }: TableTileProps) {
  const text = tileText(t, now, names);
  const paid = tablePaid(t);
  const tone = tableTone(t.status, paid);
  const owes = t.balance !== undefined && t.balance.minor !== 0;
  return (
    <button
      type="button"
      onClick={onTap}
      aria-pressed={picked}
      aria-label={`Table ${t.name}, ${tableChip(t.status, paid).word}, ${text.people}${text.time ? `, ${text.time}` : ""}${wants ? ", wants to order" : ""}`}
      className={cx(
        "relative flex h-full w-full flex-col items-center justify-center gap-1 rounded-card border-2 bg-raised p-1.5 shadow-rest 2xl:gap-1.5 2xl:p-3",
        "outline-offset-2 outline-focus hover:bg-raised-high focus-visible:outline-3",
        tone.border,
        // Chosen is a green ring inside its edge; focus is the outline outside it (TECH-12).
        picked && selectedRing,
      )}
    >
      {/* The table itself, in its state's colour: round if it is a round one. */}
      <span aria-hidden="true" className={cx("absolute inset-2 opacity-40 2xl:inset-3", tone.fill, t.shape === "round" ? "rounded-round" : "rounded")} />
      <span className="relative max-w-full truncate font-serif text-total leading-none">{t.name}</span>
      <span className="relative inline-flex max-w-full items-center gap-1.5 text-sm text-fg">
        <UsersIcon aria-hidden="true" size={15} className="shrink-0" />
        <span className="truncate">{text.people}</span>
      </span>
      {/* A phone asking to order here is what staff need to know of this table, more than that it is free. */}
      {wants ? (
        <WantsToOrder brief className="relative" />
      ) : (
        <StatusChip status={{ kind: "table", status: t.status, paid }} className="relative max-2xl:px-2" />
      )}
      {/* How long, and under it what is owed: side by side they ran past a narrow tile's edge. */}
      {(text.time || owes) && (
        <span className="relative flex max-w-full flex-col items-center font-mono text-sm tabular-nums text-fg-2">
          {text.time && <span>{text.time}</span>}
          {owes && t.balance && <Money amount={t.balance} />}
        </span>
      )}
    </button>
  );
}

/** The five states, a dot and a word each, under the floor (D05). */
export function FloorLegend() {
  return (
    <ul aria-label="Table states" className="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm text-fg">
      {turnOrder.map((status) => (
        <li key={status} className="flex items-center gap-2">
          <span aria-hidden="true" className={cx("size-3 rounded-round", tableTone(status).dot)} />
          {tableStatuses[status].word}
        </li>
      ))}
      {/* Paid is a settling table with nothing owed: a word of its own, not a sixth turn (ADR-383). */}
      <li className="flex items-center gap-2">
        <span aria-hidden="true" className={cx("size-3 rounded-round", tableTone("settling", true).dot)} />
        {paidTable.word}
      </li>
    </ul>
  );
}

/**
 * The mark on a table a guest's phone has asked to order at: a bell and a
 * word. A tile on a small screen has room for one word, so `brief` says
 * "Request" there and the whole of it from 1536px.
 */
export function WantsToOrder({ className, brief = false }: { className?: string; brief?: boolean }) {
  return (
    <span
      className={cx(
        "inline-flex max-w-full items-center gap-1 whitespace-nowrap rounded-pill bg-attention px-2 py-0.5 text-sm font-semibold text-attention-fg",
        className,
      )}
    >
      <BellRingIcon aria-hidden="true" size={13} className="shrink-0" />
      {brief ? (
        <>
          <span className="2xl:hidden">Request</span>
          <span className="max-2xl:hidden">Wants to order</span>
        </>
      ) : (
        "Wants to order"
      )}
    </span>
  );
}
