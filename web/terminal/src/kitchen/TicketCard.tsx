// One ticket on the kitchen's board (D10): what it is for, when it was placed,
// how long it has waited, what to make, and the one thing to do with it next.
//
// The design draws a card for a desk: 20px titles and 16px lines. A pass is
// read from two or three metres with wet hands, so where it goes and its timer
// are `text-display` and the lines, choices and notes `text-kitchen`, both of
// which grow with the screen (ADR-381, over ADR-176's 32px and 20px). The
// column gives the card its colour, as drawn; the timer keeps its own tone and
// word, calm, Waiting, Late, on the digits' line.
//
// Where it goes and the timer share the first row when the column has room,
// so D10's icon tile is gone and the time it was placed sits in the line under
// them, with a small icon for how it leaves the pass. A 1024 display has three
// columns of 300px; there the timer wraps under where it goes.
//
// Left out: "Called Rohan" on a ready ticket (nothing calls anyone, and the
// display knows no waiter). Recall stands there instead (rule 4).

import { BikeIcon, CheckIcon, PlayIcon, ShoppingBagIcon, Undo2Icon, UtensilsIcon, type LucideIcon } from "lucide-react";
import { memo, useEffect, useState, type ReactNode } from "react";
import type { KitchenTicketResponse, ServiceType } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { clockText } from "../../../shared/venue-text";
import { agoText, latePercent, metaText, timerText, timerTone, toneWord, whereParts, whereText, type TimerTone } from "./kitchen-logic";

/** Which column a card is in. */
export type CardStage = "fresh" | "preparing" | "ready";

const serviceIcons: Readonly<Record<ServiceType, LucideIcon>> = { dine_in: UtensilsIcon, takeaway: ShoppingBagIcon, delivery: BikeIcon };

// The column's colour: the rule down the card's left, its border, and the quantities.
const stageClass: Readonly<Record<CardStage, { card: string; qty: string }>> = {
  fresh: { card: "border-chip-danger-border border-l-attention", qty: "text-attention-text" },
  preparing: { card: "border-chip-warning-border border-l-highlight", qty: "text-attention-text" },
  ready: { card: "border-chip-success-border border-l-accent", qty: "text-chip-success-fg" },
};

// The timer's own tone: how long it has waited against the venue's late time.
const tonePill: Readonly<Record<TimerTone, string>> = {
  calm: "bg-chip-neutral-bg text-chip-neutral-fg",
  waiting: "bg-chip-warning-bg text-chip-warning-fg",
  late: "bg-chip-danger-bg text-chip-danger-fg",
};

// On night every action has a 2px edge in the muted ink, 5:1 on the card:
// the fills alone are 1.3 to 1.5:1 there, and a button with no edge is a
// guess (KDS-9, ADR-391).
const action =
  "mt-4 flex min-h-16 w-full items-center justify-center gap-2.5 rounded-pill text-xl font-semibold outline-offset-2 outline-focus focus-visible:outline-3 disabled:opacity-50 short:mt-3 ink:border-2 ink:border-fg-muted";

/** How long Recall waits for its second tap. */
const recallArmedMs = 4000;

interface TicketCardProps {
  ticket: KitchenTicketResponse;
  stage: CardStage;
  lateAfter: number;
  /** This card's own action is on its way. */
  busy: boolean;
  /** Any action is on its way: one at a time. */
  disabled: boolean;
  /**
   * Start preparing, Mark ready, or Recall: what this column does. Null for
   * someone who may only look (ADR-180). The same function for every card,
   * so a card is drawn again only when it changes.
   */
  onAct: ((ticket: KitchenTicketResponse, stage: CardStage) => void) | null;
  /** It arrived in the last minute: it says New (KDS-1). */
  isNew?: boolean;
  /** Its course, when a later one, or "Add-on" (KDS-12). */
  chip?: string | null;
  /** New Orders go straight to ready: the button is Mark ready (KDS-17). */
  skipPreparing?: boolean;
}

/**
 * A ticket's card. Drawn again only when what it shows changes: its clocks
 * are leaves that tick by themselves, so a board of twenty tickets is not
 * twenty cards redrawn every second (TECH-19, ADR-392).
 */
export const TicketCard = memo(function TicketCard({
  ticket: t,
  stage,
  lateAfter,
  busy,
  disabled,
  onAct,
  isNew = false,
  chip = null,
  skipPreparing = false,
}: TicketCardProps) {
  // Recall undoes a cook's work, so it asks for a second tap: one wet tap is not enough (KDS-15).
  const [armed, setArmed] = useState(false);
  useEffect(() => {
    if (!armed) return;
    const timer = setTimeout(() => setArmed(false), recallArmedMs);
    return () => clearTimeout(timer);
  }, [armed]);
  const Icon = serviceIcons[t.service];
  const look = stageClass[stage];
  const placed = clockText(new Date(t.sent_at), null).split(" · ")[1] ?? "";
  const ready = stage === "ready";
  const where = whereParts(t);

  return (
    <article
      aria-label={`${whereText(t)}${isNew ? ", new" : ""}${chip ? `, ${chip}` : ""}${ready ? ", ready" : ""}`}
      className={cx("rounded-card border border-l-8 bg-raised p-4 shadow-rest short:p-3", look.card)}
    >
      <header>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          {/* Where it goes: what a runner reads first. */}
          <h3 className="min-w-0 wrap-break-word font-serif text-display leading-tight">
            {where.lead && <span className="text-kitchen">{where.lead} · </span>}
            {where.main}
          </h3>
          {ready ? <ReadyChip readyAt={t.ready_at} /> : <TimerPill sentAt={t.sent_at} lateAfter={lateAfter} />}
        </div>
        <p className="mt-0.5 flex items-center gap-2 text-lg text-fg-muted">
          {isNew && <span className="shrink-0 rounded-pill bg-attention px-2.5 text-lg font-bold uppercase text-attention-fg">New</span>}
          {/* A later course or add-on, so a table sent to twice does not look like the same ticket twice (KDS-12). */}
          {chip && <span className="shrink-0 rounded-pill bg-chip-info-bg px-2.5 text-lg font-bold text-chip-info-fg">{chip}</span>}
          <Icon aria-hidden="true" size={18} strokeWidth={1.9} className="shrink-0" />
          <span className="min-w-0">{[metaText(t), placed].filter(Boolean).join(" · ")}</span>
        </p>
      </header>

      {t.notes.length > 0 && <Note className="mt-3 short:mt-2">{t.notes.join(" · ")}</Note>}

      <ul className="mt-3 grid gap-2 border-t border-line-soft pt-3 short:mt-2 short:pt-2">
        {t.lines.map((l) => {
          const voided = l.status === "voided";
          return (
            <li key={l.id} className={cx("flex gap-3", voided && "text-fg-muted")}>
              <span className={cx("min-w-7 font-mono text-kitchen font-bold leading-tight tabular-nums", voided ? "line-through" : look.qty)}>{l.qty}</span>
              <div className="grid min-w-0 flex-1 gap-1">
                <p className="text-kitchen font-semibold leading-tight">
                  <span className={voided ? "line-through" : undefined}>{l.name}</span>
                  {/* The dish is struck; the word that says so is not, and is read across the pass (KDS-13). */}
                  {voided && (
                    <span className="ml-2 inline-block rounded-pill bg-chip-danger-bg px-2.5 align-middle text-lg font-bold uppercase text-chip-danger-fg">
                      Void
                    </span>
                  )}
                </p>
                {/* A ready card is for the runner: names and quantities, and the notes still. */}
                {!ready && l.modifiers.length > 0 && <p className="text-kitchen leading-tight text-fg-2">{l.modifiers.join(", ")}</p>}
                {l.notes.map((n, i) => (
                  <Note key={i}>{n}</Note>
                ))}
              </div>
            </li>
          );
        })}
      </ul>

      {stage === "preparing" && <LateBar sentAt={t.sent_at} lateAfter={lateAfter} />}

      {onAct && stage === "fresh" && (
        <button type="button" disabled={disabled} onClick={() => onAct(t, stage)} className={cx(action, "bg-accent text-accent-fg hover:bg-accent-hover")}>
          {skipPreparing ? <CheckIcon aria-hidden="true" size={22} strokeWidth={2.4} /> : <PlayIcon aria-hidden="true" size={22} strokeWidth={2.2} />}
          {busy ? (skipPreparing ? "Marking" : "Starting") : skipPreparing ? "Mark ready" : "Start preparing"}
        </button>
      )}
      {onAct && stage === "preparing" && (
        <button
          type="button"
          disabled={disabled}
          onClick={() => onAct(t, stage)}
          className={cx(action, "border border-line-strong bg-raised hover:bg-raised-high")}
        >
          <CheckIcon aria-hidden="true" size={22} strokeWidth={2.4} />
          {busy ? "Marking" : "Mark ready"}
        </button>
      )}
      {onAct && ready && (
        <button
          type="button"
          disabled={disabled}
          onClick={() => {
            if (!armed) return setArmed(true);
            setArmed(false);
            onAct(t, stage);
          }}
          className={cx(action, armed ? "bg-attention text-attention-fg" : "bg-accent-tint text-accent-text hover:bg-raised-high")}
        >
          <Undo2Icon aria-hidden="true" size={22} strokeWidth={2.2} />
          {busy ? "Recalling" : armed ? "Tap again to recall" : "Recall"}
        </button>
      )}
    </article>
  );
});

/** How long it has waited and in what tone, by the second: the one part of a card that moves on its own. */
function TimerPill({ sentAt, lateAfter }: { sentAt: string; lateAfter: number }) {
  const elapsed = useClock(1000) - Date.parse(sentAt);
  const tone = timerTone(elapsed, lateAfter);
  const word = toneWord[tone];
  return (
    <span className={cx("ml-auto inline-flex shrink-0 items-baseline gap-2 rounded-pill px-3.5 py-1 font-bold", tonePill[tone])}>
      {word && <span className="text-xl uppercase">{word}</span>}
      <span className="font-mono text-display leading-none tabular-nums">{timerText(elapsed)}</span>
    </span>
  );
}

/** How far a ticket in preparation is towards late, by the second. */
function LateBar({ sentAt, lateAfter }: { sentAt: string; lateAfter: number }) {
  const elapsed = useClock(1000) - Date.parse(sentAt);
  return (
    <div aria-hidden="true" className="mt-4 h-2 rounded-pill bg-sunken short:mt-3">
      <div
        className={cx("h-2 rounded-pill", timerTone(elapsed, lateAfter) === "late" ? "bg-attention" : "bg-highlight")}
        style={{ width: `${latePercent(elapsed, lateAfter)}%` }}
      />
    </div>
  );
}

/** Ready, and for how long: it counts in minutes, so it looks every fifteen seconds. */
function ReadyChip({ readyAt }: { readyAt: string | undefined }) {
  const now = useClock(15_000);
  return (
    <span className="ml-auto inline-flex min-h-10 shrink-0 items-center rounded-pill bg-chip-success-bg px-4 text-xl font-bold text-chip-success-fg">
      Ready{readyAt ? ` · ${agoText(now - Date.parse(readyAt))}` : ""}
    </span>
  );
}

interface NoteProps {
  className?: string;
  children: ReactNode;
}

/** An order's or a line's note: a box that starts with the word, so it reads apart from a choice without its colour. */
function Note({ className, children }: NoteProps) {
  return (
    <p className={cx("rounded bg-chip-warning-bg px-2.5 py-1 text-kitchen leading-tight text-chip-warning-fg", className)}>
      <span className="font-bold">Note:</span> {children}
    </p>
  );
}
