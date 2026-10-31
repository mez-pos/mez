// A station's board (D10): a header, and three columns a ticket crosses from
// left to right. New Orders are sent and untouched, with Start preparing;
// Preparing are the ones a cook has, with Mark ready; Ready for pickup are on
// the pass, for as long as they can be recalled. ADR-175 gave the server the
// middle column; ADR-176 records what this screen draws, ADR-381 its sizes.
//
// Kept, where D10 draws nothing (rule 4): the station's name as the title,
// the all-day counts in a strip under it, and Recall on each ready card. A
// new ticket is started before it is marked ready here; the server would take
// a bump from new too (ADR-175).
//
// The foot holds what the display is set to: its station, the chime, a
// table's tickets together, and espresso night (D11), the same board on the
// ink surface. Night is where a display starts (BoardFoot.tsx, ADR-381).
//
// On a 768 screen every fixed height is paid out of the columns, so `short:`
// trims the header, the bands and the foot, and a three-line ticket with its
// button fits a column at 1366x768. Beside the columns, from 1536px wide, are
// Coming next and the day's stats (BoardStats.tsx); under that the stats
// leave the board and Coming next is a strip, when it has rows (ADR-381).

import { ArrowDownIcon, BellIcon, ChefHatIcon, UtensilsIcon, type LucideIcon } from "lucide-react";
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode, type RefObject } from "react";
import type { KitchenTicketResponse, StationResponse } from "../../../shared/api-types";
import { browserStorage } from "../../../shared/auth/storage";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { useMenu } from "../../../shared/use-menu";
import { useRealtime } from "../../../shared/use-realtime";
import { courseNamer } from "../register/rail-logic";
import { BoardFoot, type SignedIn } from "./BoardFoot";
import { AllDayStrip, BoardHeader, ComingStrip } from "./BoardHead";
import { BoardSide } from "./BoardStats";
import { groupByTable, noArrivals, noteArrivals, readPrefs, surfaceOf, ticketId, writePrefs, type Arrived, type BoardPrefs } from "./board-prefs";
import { allDay, boardColumns, comingRows, isNewTicket, moveFailure, readFailure, staleText, statRows, tapHeld, ticketChip, whereText } from "./kitchen-logic";
import { TicketCard, type CardStage } from "./TicketCard";
import { useChime } from "./use-chime";
import { useKitchenBoard } from "./use-kitchen-board";

interface KitchenBoardProps {
  station: StationResponse;
  onChangeStation: () => void;
  /** Who is signed in, on a terminal; the wall display has nobody. */
  signedIn?: SignedIn | undefined;
}

const columns: readonly { stage: CardStage; title: string; icon: LucideIcon; band: string; empty: string }[] = [
  { stage: "fresh", title: "New Orders", icon: BellIcon, band: "bg-attention text-attention-fg", empty: "Nothing new." },
  { stage: "preparing", title: "Preparing", icon: ChefHatIcon, band: "bg-highlight text-highlight-fg", empty: "Nothing on the go." },
  { stage: "ready", title: "Ready for pickup", icon: UtensilsIcon, band: "bg-accent text-accent-fg", empty: "The pass is clear." },
];

export function KitchenBoard({ station, onChangeStation, signedIn }: KitchenBoardProps) {
  const board = useKitchenBoard(station.code);
  // Timers move by the second.
  const now = useClock(1000);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [prefs, setPrefs] = useState<BoardPrefs>(() => readPrefs(browserStorage("local")));
  const cols = useMemo(() => boardColumns(board.data), [board.data]);
  const fresh = board.data ? cols.fresh : undefined;
  // The tickets still to make, where a void is heard (KDS-13).
  const working = useMemo(() => (board.data ? [...cols.fresh, ...cols.preparing] : undefined), [board.data, cols]);
  const chime = useChime(prefs.sound, fresh, working);

  // When each new ticket came, so it says New for a minute (KDS-1). Noted as
  // the board is read: React's way to keep state that follows a value.
  const [arrived, setArrived] = useState<Arrived>(noArrivals);
  const [noted, setNoted] = useState(fresh);
  if (fresh !== noted) {
    setNoted(fresh);
    if (fresh) setArrived((a) => noteArrivals(a, fresh, now));
  }

  // Since when the server has been silent: after ten seconds the board says
  // it may be out of date, where a live-looking board loses tickets (KDS-2).
  const { status } = useRealtime();
  const live = status.state === "live" || status.state === "away";
  const [wasLive, setWasLive] = useState(true);
  const [downSince, setDownSince] = useState<number | null>(null);
  if (live !== wasLive) {
    setWasLive(live);
    setDownSince(live ? null : now);
  }
  const stale = staleText(downSince === null ? null : now - downSince);

  // When the board last moved: a tap within 700 ms of it is a double tap
  // landing on the card that slid under the finger, and is not taken (KDS-3).
  const movedAt = useRef(0);
  const order = useMemo(() => [cols.fresh, cols.preparing, cols.ready].map((c) => c.map(ticketId).join(",")).join("|"), [cols]);
  useEffect(() => {
    movedAt.current = Date.now();
  }, [order]);

  // A failed move says so in the kitchen's words, then goes: a line that
  // never clears stops being read (KDS-14).
  useEffect(() => {
    if (!error) return;
    const timer = setTimeout(() => setError(null), 8000);
    return () => clearTimeout(timer);
  }, [error]);

  // Night is the page's surface, so every token under it turns (tokens.md),
  // set before the screen is drawn. Leaving the board does not give the page
  // back to paper: the choice of station keeps the board's surface, and
  // StationBoard gives it back when the kitchen is left (KDS-18).
  useLayoutEffect(() => {
    document.documentElement.dataset.surface = surfaceOf(prefs);
  }, [prefs]);
  const keep = (next: BoardPrefs) => {
    setPrefs(next);
    writePrefs(browserStorage("local"), next);
  };
  const counts = allDay([...cols.fresh, ...cols.preparing]);
  const lateAfter = board.data?.late_after_minutes ?? 20;
  // A wall display moves its tickets; a person needs kitchen.work (ADR-180).
  const canWork = signedIn?.canWork ?? true;
  const stats = board.data ? statRows(board.data.stats) : null;
  const menu = useMenu();
  const courseName = useMemo(() => courseNamer(menu.data?.courses ?? []), [menu.data]);
  const coming = comingRows(board.data?.coming ?? [], courseName);
  const comingMore = board.data?.coming_more ?? 0;
  // With Skip preparing on, New Orders go straight to the pass: the server
  // takes a bump from new (ADR-175), for a drink made in thirty seconds (KDS-17).
  const acts: Readonly<Record<CardStage, (t: KitchenTicketResponse) => Promise<void>>> = {
    fresh: prefs.skipPreparing ? board.bump : board.start,
    preparing: board.bump,
    ready: board.recall,
  };

  const run = (t: KitchenTicketResponse, stage: CardStage) => {
    if (tapHeld(movedAt.current)) return;
    const id = ticketId(t);
    setBusy(id);
    setError(null);
    acts[stage](t)
      .catch((err: unknown) => setError(moveFailure(err, whereText(t))))
      .finally(() => setBusy(null));
  };
  // One function for every card, so a card is drawn again only when it
  // changes, and not every second with the clock (TECH-19).
  const latestRun = useRef(run);
  useEffect(() => {
    latestRun.current = run;
  });
  const act = useCallback((t: KitchenTicketResponse, stage: CardStage) => latestRun.current(t, stage), []);

  return (
    <main className="flex h-screen flex-col overflow-hidden bg-canvas text-fg">
      <BoardHeader stationName={station.name} now={now} />
      {counts.length > 0 && <AllDayStrip counts={counts} />}
      {coming.length > 0 && <ComingStrip coming={coming} comingMore={comingMore} />}

      {chime.blocked && (
        // A tap anywhere unblocks it; this says so, and is itself that tap.
        <button
          type="button"
          onClick={chime.unblock}
          className="shrink-0 bg-attention px-7 py-3 text-left text-xl font-semibold text-attention-fg outline-offset-[-4px] outline-current focus-visible:outline-3 short:px-5 short:py-2"
        >
          Sound is blocked by the browser. Tap here to turn it on, so new tickets chime.
        </button>
      )}
      {stale && (
        <p role="alert" className="shrink-0 bg-danger-fill px-7 py-3 text-xl font-semibold text-attention-fg short:px-5 short:py-2">
          {stale}
        </p>
      )}

      {!canWork && (
        <p className="shrink-0 border-b border-line bg-chrome px-7 py-2 text-lg text-fg-muted short:px-5 short:py-1">
          You can see the board. Starting and finishing tickets is the kitchen&apos;s.
        </p>
      )}

      {error && (
        <p role="alert" className="shrink-0 px-7 pt-3 text-xl text-danger">
          {error}
        </p>
      )}
      {board.loading && <p className="px-7 pt-3 text-xl text-fg-muted">Loading the tickets</p>}
      {!board.loading && board.error !== null && !board.data && (
        <p role="alert" className="px-7 pt-3 text-xl text-danger">
          {readFailure(board.error)}
        </p>
      )}

      <div className="flex min-h-0 flex-1 gap-3 px-4 py-3 short:py-2 xl:gap-4 xl:px-5 xl:py-4">
        <div className="grid min-h-0 min-w-0 flex-1 grid-cols-3 gap-3 xl:gap-4">
          {columns.map((col) => {
            const tickets = prefs.groupByTable ? groupByTable(cols[col.stage]) : cols[col.stage];
            return (
              <Column key={col.stage} title={col.title} icon={col.icon} band={col.band} count={tickets.length} watchNew={col.stage === "fresh"}>
                {board.data && tickets.length === 0 && <p className="py-10 text-center text-xl text-fg-muted short:py-6">{col.empty}</p>}
                {tickets.map((t) => {
                  const id = ticketId(t);
                  const isNew = col.stage === "fresh" && isNewTicket(arrived.at.get(id), now);
                  return (
                    <li key={id} data-new={isNew || undefined}>
                      <TicketCard
                        ticket={t}
                        stage={col.stage}
                        lateAfter={lateAfter}
                        busy={busy === id}
                        disabled={busy !== null}
                        onAct={canWork ? act : null}
                        isNew={isNew}
                        chip={ticketChip(t, (n) => courseName(n))}
                        skipPreparing={prefs.skipPreparing}
                      />
                    </li>
                  );
                })}
              </Column>
            );
          })}
        </div>
        {stats && <BoardSide rows={stats} coming={coming} comingMore={comingMore} />}
      </div>

      <BoardFoot prefs={prefs} onPrefs={keep} onChangeStation={onChangeStation} signedIn={signedIn} />
    </main>
  );
}

interface ColumnProps {
  title: string;
  icon: LucideIcon;
  /** The band's fill and ink. */
  band: string;
  count: number;
  /** New Orders: a new ticket below the column's fold is pointed to (KDS-1). */
  watchNew?: boolean;
  children: ReactNode;
}

function Column({ title, icon: Icon, band, count, watchNew = false, children }: ColumnProps) {
  const list = useRef<HTMLUListElement>(null);
  const below = useNewBelow(list, watchNew);
  return (
    <section aria-label={`${title}, ${count}`} className="relative flex min-h-0 flex-col overflow-hidden rounded-card border border-line bg-chrome">
      <header className={cx("flex min-h-16 shrink-0 items-center gap-3 px-5 short:min-h-12 short:px-4", band)}>
        <Icon aria-hidden="true" size={24} strokeWidth={1.9} />
        <h2 className="font-serif text-xl text-current">{title}</h2>
        <span className="ml-auto flex size-10 items-center justify-center rounded-round bg-current/20 font-mono text-lg font-bold tabular-nums">{count}</span>
      </header>
      <ul ref={list} className="relative grid min-h-0 flex-1 content-start gap-3 overflow-y-auto p-3 short:gap-2 short:p-2 xl:p-3.5">
        {children}
      </ul>
      {below.count > 0 && (
        <button
          type="button"
          onClick={below.show}
          className="absolute bottom-3 left-1/2 inline-flex min-h-tap -translate-x-1/2 items-center gap-2 rounded-pill bg-attention px-5 text-lg font-bold text-attention-fg shadow-modal outline-offset-2 outline-focus focus-visible:outline-3"
        >
          <ArrowDownIcon aria-hidden="true" size={20} strokeWidth={2.2} />
          {below.count === 1 ? "1 new below" : `${below.count} new below`}
        </button>
      )}
    </section>
  );
}

/** The new tickets under a list's fold, as they stand now. */
function newBelowFold(ul: HTMLUListElement | null): HTMLElement[] {
  if (!ul) return [];
  const fold = ul.scrollTop + ul.clientHeight;
  return [...ul.querySelectorAll<HTMLElement>("[data-new]")].filter((li) => li.offsetTop + 24 > fold);
}

/**
 * The new tickets under a column's fold, and a way to them. New Orders are
 * in the order they were sent, so a new one lands at the foot of a full
 * column, where nobody looks (KDS-1). Measured when the column scrolls,
 * changes or is resized.
 */
function useNewBelow(list: RefObject<HTMLUListElement | null>, watch: boolean): { count: number; show: () => void } {
  const [count, setCount] = useState(0);
  useEffect(() => {
    const ul = list.current;
    if (!watch || !ul) return;
    const measure = () => setCount(newBelowFold(ul).length);
    const frame = requestAnimationFrame(measure);
    const changes = new MutationObserver(measure);
    changes.observe(ul, { childList: true, subtree: true, attributes: true, attributeFilter: ["data-new"] });
    const sizes = new ResizeObserver(measure);
    sizes.observe(ul);
    ul.addEventListener("scroll", measure, { passive: true });
    return () => {
      cancelAnimationFrame(frame);
      changes.disconnect();
      sizes.disconnect();
      ul.removeEventListener("scroll", measure);
    };
  }, [watch, list]);
  return {
    count,
    show: () => {
      const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      newBelowFold(list.current)[0]?.scrollIntoView({ block: "nearest", behavior: still ? "auto" : "smooth" });
    },
  };
}
