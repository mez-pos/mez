// The floor (docs/design/screens/terminal-floor.md): every table in service
// as a tile with its state in colour and a word, its covers, its time and
// what it owes. A tap seats a party, opens a table's ticket, or opens a
// sheet when there is a choice to make: guests leaving, a table to clear.
// Move, Join and Part turn taps into moving a party and joining tables
// (ADR-088) until Done.

import { useEffect, useState } from "react";
import { ApiError } from "../../../shared/api";
import type { ChipTone } from "../../../shared/ui";
import type { LiveFloorResponse, LiveTableResponse, TicketSummary } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon, Money, Qty, Sheet, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { tableStatuses } from "../../../shared/ui/status";
import { failureOf } from "../register/order-messages";
import { elapsed, ticketLabel } from "../register/ticket-time";
import type { Ticket } from "../register/use-ticket";
import {
  areaToShow,
  countsText,
  idle,
  modeText,
  startingCovers,
  stepMode,
  tableNames,
  tapFor,
  tileText,
  tilesOf,
  type Mode,
} from "./floor-logic";
import type { LiveFloorApi } from "./use-live-floor";

interface FloorViewProps {
  floor: LiveFloorApi;
  ticket: Ticket;
  /** Back to the register, on the ticket now in front. */
  onRegister: () => void;
}

type Open = { kind: "seat"; table: LiveTableResponse } | { kind: "table"; table: LiveTableResponse };

export function FloorView({ floor, ticket, onRegister }: FloorViewProps) {
  const [areaChoice, setAreaChoice] = useState<number | null>(null);
  const [open, setOpen] = useState<Open | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<Mode>(idle);
  const [busy, setBusy] = useState(false);
  const now = useClock(30_000);
  const data = floor.data;
  const areaId = data ? areaToShow(data, areaChoice) : null;
  const names = tableNames(data);
  const banner = modeText(mode);
  const picked = mode.kind === "move" ? mode.from : mode.kind === "join" ? mode.to : null;

  /** A tap while Move, Join or Part is on: the mode decides, the server does. */
  const tapInMode = (t: LiveTableResponse) => {
    const step = stepMode(mode, t, names);
    setMode(step.mode);
    setError(step.refused ?? null);
    const action = step.action;
    if (!action) return;
    setBusy(true);
    floor
      .act(action)
      .catch((err: unknown) => setError(wordsOf(err)))
      .finally(() => setBusy(false));
  };

  /** Brings a ticket to the front, off hold if it was held, and goes to it. */
  const openTicket = async (orderId: string) => {
    const held = ticket.open?.orders.some((o) => o.id === orderId && o.status === "held");
    if (held) await ticket.resume(orderId);
    else ticket.switchTo(orderId);
    onRegister();
  };

  const tap = (t: LiveTableResponse) => {
    if (mode.kind !== "idle") {
      if (!busy) tapInMode(t);
      return;
    }
    setError(null);
    const what = tapFor(t);
    if (what.kind === "seat") setOpen({ kind: "seat", table: t });
    else if (what.kind === "sheet") setOpen({ kind: "table", table: t });
    else openTicket(what.orderId).catch((err: unknown) => setError(wordsOf(err)));
  };

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-4">
      <div className="flex flex-wrap items-baseline justify-between gap-4">
        <div className="flex flex-wrap items-baseline gap-4">
          <h1 className="font-serif text-total">Floor</h1>
          {data && <p className="text-base text-fg-2">{countsText(data.tables)}</p>}
        </div>
        <div className="flex flex-wrap gap-tap-gap">
          {data && data.tables.length > 0 && mode.kind === "idle" && (
            <>
              <Button onClick={() => setMode({ kind: "move", from: null })}>Move</Button>
              <Button onClick={() => setMode({ kind: "join", to: null })}>Join</Button>
              <Button onClick={() => setMode({ kind: "part" })}>Part</Button>
            </>
          )}
          <Button onClick={onRegister}>Register</Button>
        </div>
      </div>

      {banner && (
        <div role="status" className="flex flex-wrap items-center gap-tap-gap rounded border border-line-focus bg-raised px-4 py-2">
          <p className="flex-1 text-base font-semibold">{banner}</p>
          <Button
            variant="primary"
            onClick={() => {
              setMode(idle);
              setError(null);
            }}
          >
            Done
          </Button>
        </div>
      )}

      {data && data.areas.length > 1 && (
        <div role="tablist" aria-label="Areas" className="flex flex-wrap gap-tap-gap">
          {data.areas.map((a) => (
            <button
              key={a.id}
              type="button"
              role="tab"
              aria-selected={a.id === areaId}
              onClick={() => setAreaChoice(a.id)}
              className={cx(
                "min-h-tap rounded border px-4 text-base outline-offset-2 outline-line-focus focus-visible:outline-2",
                a.id === areaId ? "border-line-focus bg-raised-high font-semibold text-fg" : "border-line text-fg-2",
              )}
            >
              {a.name}
            </button>
          ))}
        </div>
      )}

      {error && (
        <p role="alert" className="text-base text-danger">
          {error}
        </p>
      )}
      {floor.loading && <p className="text-base text-fg-muted">Loading the floor</p>}
      {!floor.loading && floor.error !== null && !data && (
        <EmptyState icon={<InboxIcon />} line={failureOf(floor.error)} action={{ label: "Try again", onClick: floor.refetch }} />
      )}
      {data && data.tables.length === 0 && (
        <EmptyState icon={<InboxIcon />} line="No tables yet. Lay out the floor in the back office." action={{ label: "Register", onClick: onRegister }} />
      )}
      {data && areaId !== null && (
        <Grid data={data} areaId={areaId} now={now} names={names} picked={picked?.id ?? null} onTap={tap} />
      )}

      {open?.kind === "seat" && (
        <SeatSheet
          table={open.table}
          onSeat={async (covers) => {
            await ticket.seat({ channel: "counter", table_id: open.table.id, covers });
            setOpen(null);
            onRegister();
          }}
          onClose={() => setOpen(null)}
        />
      )}
      {open?.kind === "table" && data && (
        <TableSheet
          // The sheet follows the table as the floor moves under it.
          table={data.tables.find((t) => t.id === open.table.id) ?? open.table}
          tickets={ticket.open?.orders ?? []}
          names={names}
          now={now}
          lateAfter={ticket.open?.late_after_minutes ?? 20}
          floor={floor}
          onTicket={async (id) => {
            await openTicket(id);
            setOpen(null);
          }}
          onClose={() => setOpen(null)}
        />
      )}
    </div>
  );
}

interface GridProps {
  data: LiveFloorResponse;
  areaId: number;
  now: number;
  names: ReadonlyMap<number, string>;
  /** The party's table while moving or joining, drawn with the focus outline. */
  picked: number | null;
  onTap: (t: LiveTableResponse) => void;
}

function Grid({ data, areaId, now, names, picked, onTap }: GridProps) {
  const rows = tilesOf(data, areaId);
  if (rows.length === 0) return <p className="text-base text-fg-muted">No tables in this area.</p>;
  const columns = rows[0]?.length ?? 0;
  return (
    <div className="overflow-x-auto">
      <ul
        aria-label="Tables"
        className="grid gap-2.5"
        // A tile is 88 to 144px, whatever the width: a small floor stays
        // tiles, not a wall of squares, and a wide one scrolls sideways.
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(5.5rem, 9rem))` }}
      >
        {rows.flatMap((row, y) =>
          row.map((t, x) =>
            t === null ? (
              <li key={`${x},${y}`} aria-hidden="true" />
            ) : (
              <li key={t.id}>
                <TableTile table={t} now={now} names={names} picked={picked === t.id} onTap={() => onTap(t)} />
              </li>
            ),
          ),
        )}
      </ul>
    </div>
  );
}

// The tile's border takes its state's tone; the chip carries the word.
const borders: Readonly<Record<ChipTone, string>> = {
  neutral: "border-chip-neutral-border",
  success: "border-chip-success-border",
  warning: "border-chip-warning-border",
  danger: "border-chip-danger-border",
  accent: "border-chip-accent-border",
  info: "border-chip-info-border",
};

interface TableTileProps {
  table: LiveTableResponse;
  now: number;
  names: ReadonlyMap<number, string>;
  picked: boolean;
  onTap: () => void;
}

function TableTile({ table: t, now, names, picked, onTap }: TableTileProps) {
  const text = tileText(t, now, names);
  const spec = tableStatuses[t.status];
  const round = t.shape === "round";
  return (
    <button
      type="button"
      onClick={onTap}
      aria-pressed={picked || undefined}
      aria-label={`Table ${t.name}, ${spec.word}, ${text.people}${text.time ? `, ${text.time}` : ""}`}
      className={cx(
        "flex aspect-square min-h-22 w-full flex-col border-2 bg-raised outline-offset-2 outline-line-focus focus-visible:outline-2",
        // A round table is drawn round, with everything in the middle of it.
        round ? "items-center justify-center gap-1 rounded-round p-4 text-center" : "justify-between rounded p-2.5 text-left",
        borders[spec.tone],
        picked && "outline outline-4",
      )}
    >
      <span className={cx("flex gap-2", round ? "flex-col items-center" : "items-start justify-between")}>
        <span className="max-w-full truncate text-xl font-semibold leading-tight">{t.name}</span>
        {text.time && <span className="font-mono text-sm tabular-nums text-fg-muted">{text.time}</span>}
      </span>
      <span className={cx("flex flex-col gap-1", round ? "items-center" : "items-start")}>
        <StatusChip status={{ kind: "table", status: t.status }} />
        <span className="max-w-full truncate text-sm text-fg-2">{text.people}</span>
        {t.balance && t.balance.minor !== 0 && <Money amount={t.balance} className={round ? "" : "self-stretch text-right"} />}
      </span>
    </button>
  );
}

interface SeatSheetProps {
  table: LiveTableResponse;
  onSeat: (covers: number) => Promise<void>;
  onClose: () => void;
}

function SeatSheet({ table, onSeat, onClose }: SeatSheetProps) {
  const [covers, setCovers] = useState(() => startingCovers(table.seats));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const seat = () => {
    setBusy(true);
    setError(null);
    onSeat(covers).catch((err: unknown) => {
      setError(wordsOf(err));
      setBusy(false);
    });
  };
  return (
    <Sheet
      open
      onClose={onClose}
      title={`Seat table ${table.name}`}
      footer={
        <Button variant="primary" size="charge" className="w-full" disabled={busy} onClick={seat}>
          {busy ? "Seating" : `Seat ${covers} ${covers === 1 ? "guest" : "guests"}`}
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {error && (
          <p role="alert" className="text-base text-danger">
            {error}
          </p>
        )}
        <div className="flex items-center justify-between gap-4">
          <span className="text-base font-semibold">Covers</span>
          <Qty value={covers} onChange={setCovers} label="Covers" bounds={{ min: 1, max: 99 }} />
        </div>
        <p className="text-sm text-fg-muted">
          {table.seats} {table.seats === 1 ? "seat" : "seats"} at this table.
        </p>
      </div>
    </Sheet>
  );
}

interface TableSheetProps {
  table: LiveTableResponse;
  tickets: readonly TicketSummary[];
  names: ReadonlyMap<number, string>;
  now: number;
  lateAfter: number;
  floor: LiveFloorApi;
  onTicket: (orderId: string) => Promise<void>;
  onClose: () => void;
}

function TableSheet({ table: t, tickets, names, now, lateAfter, floor, onTicket, onClose }: TableSheetProps) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const run = (fn: () => Promise<void>, close: boolean) => {
    setBusy(true);
    setError(null);
    fn()
      .then(() => {
        if (close) onClose();
        else setBusy(false);
      })
      .catch((err: unknown) => {
        setError(wordsOf(err));
        setBusy(false);
      });
  };
  const here = t.orders.map((id) => tickets.find((o) => o.id === id)).filter((o): o is TicketSummary => o !== undefined);

  return (
    <Sheet open onClose={onClose} title={`Table ${t.name}`}>
      <div className="flex flex-col gap-4">
        <StatusChip status={{ kind: "table", status: t.status }} />
        {error && (
          <p role="alert" className="text-base text-danger">
            {error}
          </p>
        )}

        {t.status === "free" && <p className="text-base text-fg-2">This table is free.</p>}
        {t.status === "dirty" && (
          <Button variant="primary" disabled={busy} onClick={() => run(() => floor.clear(t.id), true)}>
            Clear table
          </Button>
        )}

        {here.length > 0 && (
          <ul className="flex flex-col gap-tap-gap">
            {here.map((o) => {
              const time = elapsed(o.opened_at, now, lateAfter);
              return (
                <li key={o.id}>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => run(() => onTicket(o.id), false)}
                    className="grid min-h-tap w-full grid-cols-[1fr_auto] items-center gap-3 rounded border border-line bg-raised px-4 py-3 text-left outline-offset-2 outline-line-focus focus-visible:outline-2"
                  >
                    <span className="flex flex-col gap-1">
                      <span className="text-base font-semibold">Open ticket: {ticketLabel(o, names)}</span>
                      <span className="text-sm text-fg-muted">
                        {o.lines === 0 ? "Nothing ordered yet" : `${o.lines} ${o.lines === 1 ? "item" : "items"}`}
                      </span>
                    </span>
                    <span className="flex flex-col items-end gap-1">
                      <Money amount={o.total} />
                      <span className={cx("font-mono text-sm tabular-nums", time.late ? "text-accent-text" : "text-fg-muted")}>
                        {time.text}
                      </span>
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}

        {t.status === "seated" && (
          <div className="flex flex-col gap-2 border-t border-line pt-4">
            <Button disabled={busy} onClick={() => run(() => floor.release(t.id), true)}>
              Guests left
            </Button>
            <p className="text-sm text-fg-muted">They ordered nothing: their empty ticket closes and the table is free.</p>
          </div>
        )}
      </div>
    </Sheet>
  );
}

/**
 * The till's words for a failure: a server refusal by its code, or the words
 * the ticket queue already chose (it rejects with an Error in those words).
 */
function wordsOf(err: unknown): string {
  if (err instanceof ApiError || !(err instanceof Error)) return failureOf(err);
  return err.message;
}

/** The time, every `every` ms, for tiles whose minutes must move. */
function useClock(every: number): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), every);
    return () => clearInterval(id);
  }, [every]);
  return now;
}
