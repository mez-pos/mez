// A station's board (docs/design/screens/kitchen-display.md): the tickets
// still to make as cards in large type, oldest first, each with a timer that
// turns mustard and then terracotta, and a Bump big enough for a wet thumb; the
// all-day counts; Recall for the ticket bumped last.

import { useEffect, useState } from "react";
import type { KitchenTicketResponse, StationResponse } from "../../../shared/api-types";
import { ConnectionIndicator } from "../../../shared/ConnectionIndicator";
import { cx } from "../../../shared/ui/cx";
import { failureOf } from "../register/order-messages";
import { allDay, lastBumped, timerText, timerTone, toMake, toneWord, whereText, type TimerTone } from "./kitchen-logic";
import { useKitchenBoard } from "./use-kitchen-board";

interface KitchenBoardProps {
  station: StationResponse;
  onChangeStation: () => void;
}

export function KitchenBoard({ station, onChangeStation }: KitchenBoardProps) {
  const board = useKitchenBoard(station.code);
  const now = useTick(1000);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const tickets = toMake(board.data);
  const counts = allDay(tickets);
  const recallable = lastBumped(board.data);
  const lateAfter = board.data?.late_after_minutes ?? 20;

  const run = (id: string, fn: () => Promise<void>) => {
    setBusy(id);
    setError(null);
    fn()
      .catch((err: unknown) => setError(failureOf(err)))
      .finally(() => setBusy(null));
  };

  return (
    <main className="flex min-h-screen flex-col bg-canvas text-fg">
      <header className="flex flex-wrap items-center gap-4 border-b border-line bg-chrome px-6 py-3">
        <h1 className="font-serif text-display leading-none">{station.name}</h1>
        <span className="font-mono text-kitchen tabular-nums text-fg-2">{clockText(now)}</span>
        <ConnectionIndicator />
        <div className="ml-auto flex flex-wrap gap-3">
          {recallable && (
            <button
              type="button"
              disabled={busy !== null}
              onClick={() => run(`recall-${recallable.order_id}-${recallable.ticket_seq}`, () => board.recall(recallable))}
              className="min-h-18 rounded-pill border-2 border-line-strong bg-raised px-8 text-kitchen font-semibold outline-offset-2 outline-line-focus focus-visible:outline-4 disabled:opacity-50"
            >
              Recall {whereText(recallable)}
            </button>
          )}
          <button
            type="button"
            onClick={onChangeStation}
            className="min-h-14 rounded-pill border border-line-strong px-6 text-lg text-fg-2 outline-offset-2 outline-line-focus focus-visible:outline-4"
          >
            Change station
          </button>
        </div>
      </header>

      {counts.length > 0 && (
        <section aria-label="All day" className="flex flex-wrap gap-x-6 gap-y-2 border-b border-line px-6 py-3 text-kitchen">
          <span className="text-fg-muted">All day</span>
          {counts.map((c) => (
            <span key={c.name}>
              <span className="font-mono font-semibold tabular-nums text-highlight">{c.qty}</span> {c.name}
            </span>
          ))}
        </section>
      )}

      {error && (
        <p role="alert" className="px-6 pt-3 text-kitchen text-danger">
          {error}
        </p>
      )}

      <div className="flex-1 overflow-y-auto p-6">
        {board.loading && <p className="text-kitchen text-fg-muted">Loading the tickets</p>}
        {!board.loading && board.error !== null && !board.data && <p className="text-kitchen text-danger">{failureOf(board.error)}</p>}
        {board.data && tickets.length === 0 && <p className="py-20 text-center font-serif text-display text-fg-muted">All clear.</p>}
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(18rem,1fr))] items-start gap-4">
          {tickets.map((t) => {
            const id = `${t.order_id}-${t.ticket_seq}`;
            return (
              <li key={id}>
                <TicketCard
                  ticket={t}
                  now={now}
                  lateAfter={lateAfter}
                  busy={busy === id}
                  disabled={busy !== null}
                  onBump={() => run(id, () => board.bump(t))}
                />
              </li>
            );
          })}
        </ul>
      </div>
    </main>
  );
}

// A timer's tone as the rule down the card's left and the timer's pill:
// mustard while it waits, the late red once it is late (tokens.md).
const toneClass: Readonly<Record<TimerTone, { rule: string; pill: string }>> = {
  calm: { rule: "border-l-fg-muted", pill: "bg-chip-neutral-bg text-fg" },
  waiting: { rule: "border-l-highlight", pill: "bg-chip-warning-bg text-chip-warning-fg" },
  late: { rule: "border-l-danger-line", pill: "bg-chip-danger-bg text-chip-danger-fg" },
};

interface TicketCardProps {
  ticket: KitchenTicketResponse;
  now: number;
  lateAfter: number;
  busy: boolean;
  disabled: boolean;
  onBump: () => void;
}

function TicketCard({ ticket: t, now, lateAfter, busy, disabled, onBump }: TicketCardProps) {
  const elapsed = now - Date.parse(t.sent_at);
  const tone = timerTone(elapsed, lateAfter);
  const word = toneWord[tone];
  return (
    <article
      aria-label={`${whereText(t)}, ${timerText(elapsed)}${word ? `, ${word}` : ""}`}
      className={cx("flex flex-col overflow-hidden rounded-card border border-l-8 border-line-strong bg-raised shadow-card", toneClass[tone].rule)}
    >
      <header className="flex flex-col gap-1 border-b border-line px-4 py-3">
        {/* Where it goes on a line of its own: it is what a runner reads first. */}
        <p className="break-words font-serif text-display leading-tight">{whereText(t)}</p>
        <div className="flex items-baseline justify-between gap-3">
          <span className="text-lg text-fg-2">{t.covers > 0 ? `${t.covers} ${t.covers === 1 ? "cover" : "covers"}` : ""}</span>
          <span className={cx("flex items-baseline gap-2 rounded-pill px-4 py-1.5", toneClass[tone].pill)}>
            {word && <span className="text-lg font-semibold uppercase">{word}</span>}
            <span className="font-mono text-display font-semibold leading-none tabular-nums">{timerText(elapsed)}</span>
          </span>
        </div>
      </header>
      {t.notes.length > 0 && <p className="border-b border-line bg-chip-warning-bg px-4 py-2 text-kitchen text-chip-warning-fg">{t.notes.join(" · ")}</p>}
      <ul className="flex flex-col gap-2 px-4 py-3">
        {t.lines.map((l) => (
          <li key={l.id} className={cx(l.status === "voided" && "text-fg-muted line-through")}>
            <p className="text-kitchen font-semibold">
              <span className={cx("font-mono tabular-nums", l.status !== "voided" && "text-highlight")}>{l.qty}</span> × {l.name}
              {l.status === "voided" && <span className="ml-2 text-lg no-underline">Void</span>}
            </p>
            {l.modifiers.length > 0 && <p className="text-lg text-fg-2">{l.modifiers.join(", ")}</p>}
            {l.notes.map((n, i) => (
              <p key={i} className="text-lg text-chip-warning-fg">
                {n}
              </p>
            ))}
          </li>
        ))}
      </ul>
      <button
        type="button"
        disabled={disabled}
        onClick={onBump}
        className="m-4 mt-auto min-h-18 rounded-pill bg-accent text-display font-semibold text-accent-fg outline-offset-2 outline-line-focus hover:bg-accent-hover focus-visible:outline-4 disabled:opacity-50"
      >
        {busy ? "Bumping" : "Bump"}
      </button>
    </article>
  );
}

function clockText(now: number): string {
  return new Date(now).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
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
