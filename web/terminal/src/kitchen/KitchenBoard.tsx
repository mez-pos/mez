// A station's board (D10): a header, and three columns a ticket crosses from
// left to right. New Orders are sent and untouched, with Start preparing;
// Preparing are the ones a cook has, with Mark ready; Ready for pickup are on
// the pass, for as long as they can be recalled. ADR-175 gave the server the
// middle column; ADR-176 records what this screen draws.
//
// Kept, where D10 draws nothing (rule 4): the station's name and Change
// station in the header, the all-day counts in a strip under it, and Recall
// on each ready card. A new ticket is started before it is marked ready here;
// the server would take a bump from new too (ADR-175).
//
// Not here yet: Today's stats and Upcoming (15d), and the foot with Night mode
// and Sound (15c).

import { BellIcon, ChefHatIcon, UtensilsIcon, type LucideIcon } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import type { KitchenTicketResponse, StationResponse } from "../../../shared/api-types";
import { ConnectionIndicator } from "../../../shared/ConnectionIndicator";
import { Logo } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { clockText } from "../../../shared/venue-text";
import { failureOf } from "../register/order-messages";
import { allDay, boardColumns } from "./kitchen-logic";
import { TicketCard, type CardStage } from "./TicketCard";
import { useKitchenBoard } from "./use-kitchen-board";

interface KitchenBoardProps {
  station: StationResponse;
  onChangeStation: () => void;
}

const columns: readonly { stage: CardStage; title: string; icon: LucideIcon; band: string; empty: string }[] = [
  { stage: "fresh", title: "New Orders", icon: BellIcon, band: "bg-attention text-attention-fg", empty: "Nothing new." },
  { stage: "preparing", title: "Preparing", icon: ChefHatIcon, band: "bg-highlight text-highlight-fg", empty: "Nothing on the go." },
  { stage: "ready", title: "Ready for pickup", icon: UtensilsIcon, band: "bg-accent text-accent-fg", empty: "The pass is clear." },
];

export function KitchenBoard({ station, onChangeStation }: KitchenBoardProps) {
  const board = useKitchenBoard(station.code);
  const now = useTick(1000);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const cols = boardColumns(board.data);
  const counts = allDay([...cols.fresh, ...cols.preparing]);
  const lateAfter = board.data?.late_after_minutes ?? 20;
  const acts: Readonly<Record<CardStage, (t: KitchenTicketResponse) => Promise<void>>> = { fresh: board.start, preparing: board.bump, ready: board.recall };

  const run = (id: string, fn: () => Promise<void>) => {
    setBusy(id);
    setError(null);
    fn()
      .catch((err: unknown) => setError(failureOf(err)))
      .finally(() => setBusy(null));
  };

  return (
    <main className="flex h-screen flex-col overflow-hidden bg-canvas text-fg">
      <header className="flex min-h-23 shrink-0 items-center gap-6 border-b border-line bg-chrome px-7 py-2">
        <Logo className="h-9 shrink-0" />
        <div className="min-w-0 flex-1 text-center">
          <h1 className="flex items-center justify-center gap-3 font-serif text-total leading-tight">
            <ChefHatIcon aria-hidden="true" size={30} strokeWidth={1.7} className="shrink-0 text-attention" />
            Kitchen Display
          </h1>
          <p className="truncate text-base text-fg-muted">{station.name} · Fresh orders. Happy people.</p>
        </div>
        <div className="flex shrink-0 items-center gap-4">
          <span className="hidden font-mono text-base font-semibold tabular-nums xl:block">{clockText(new Date(now), null)}</span>
          <ConnectionIndicator />
          <button
            type="button"
            onClick={onChangeStation}
            className="min-h-tap rounded-pill border border-line-strong px-5 text-base font-semibold outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-4"
          >
            Change station
          </button>
          <p aria-hidden="true" className="hidden text-right font-script text-xl leading-tight text-fg-2 2xl:block">
            Good Food,
            <br />
            Brighter Days ♡
          </p>
        </div>
      </header>

      {counts.length > 0 && (
        <section aria-label="All day" className="flex shrink-0 flex-wrap gap-x-6 gap-y-1 border-b border-line bg-chrome px-7 py-2 text-xl">
          <span className="text-fg-muted">All day</span>
          {counts.map((c) => (
            <span key={c.name}>
              <span className="font-mono font-bold tabular-nums text-attention-text">{c.qty}</span> {c.name}
            </span>
          ))}
        </section>
      )}

      {error && (
        <p role="alert" className="shrink-0 px-7 pt-3 text-xl text-danger">
          {error}
        </p>
      )}
      {board.loading && <p className="px-7 pt-3 text-xl text-fg-muted">Loading the tickets</p>}
      {!board.loading && board.error !== null && !board.data && (
        <p role="alert" className="px-7 pt-3 text-xl text-danger">
          {failureOf(board.error)}
        </p>
      )}

      <div className="grid min-h-0 flex-1 grid-cols-3 gap-4 px-5 py-4">
        {columns.map((col) => {
          const tickets = cols[col.stage];
          return (
            <Column key={col.stage} title={col.title} icon={col.icon} band={col.band} count={tickets.length}>
              {board.data && tickets.length === 0 && <p className="py-10 text-center text-xl text-fg-muted">{col.empty}</p>}
              {tickets.map((t) => {
                const id = `${t.order_id}-${t.ticket_seq}`;
                return (
                  <li key={id}>
                    <TicketCard
                      ticket={t}
                      stage={col.stage}
                      now={now}
                      lateAfter={lateAfter}
                      busy={busy === id}
                      disabled={busy !== null}
                      onAct={() => run(id, () => acts[col.stage](t))}
                    />
                  </li>
                );
              })}
            </Column>
          );
        })}
      </div>
    </main>
  );
}

interface ColumnProps {
  title: string;
  icon: LucideIcon;
  /** The band's fill and ink. */
  band: string;
  count: number;
  children: ReactNode;
}

function Column({ title, icon: Icon, band, count, children }: ColumnProps) {
  return (
    <section aria-label={`${title}, ${count}`} className="flex min-h-0 flex-col overflow-hidden rounded-card border border-line bg-chrome">
      <header className={cx("flex min-h-16 shrink-0 items-center gap-3 px-5", band)}>
        <Icon aria-hidden="true" size={24} strokeWidth={1.9} />
        <h2 className="font-serif text-xl text-current">{title}</h2>
        <span className="ml-auto flex size-10 items-center justify-center rounded-round bg-current/20 font-mono text-lg font-bold tabular-nums">{count}</span>
      </header>
      <ul className="grid min-h-0 flex-1 content-start gap-3 overflow-y-auto p-3.5">{children}</ul>
    </section>
  );
}

/** The time, every `every` ms: timers move by the second. */
function useTick(every: number): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), every);
    return () => clearInterval(id);
  }, [every]);
  return now;
}
