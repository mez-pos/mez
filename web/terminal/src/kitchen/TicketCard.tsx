// One ticket on the kitchen's board (D10): what it is for, when it was placed,
// how long it has waited, what to make, and the one thing to do with it next.
//
// The design draws a card for a desk: 20px titles and 16px lines. A pass is
// read from three metres with wet hands (ADR-091), so where it goes is 32px,
// the lines 26px and the action 64px tall. The column gives the card its
// colour, as drawn; the timer keeps its own tone and word, calm, Waiting, Late.
//
// Left out: "Called Rohan" on a ready ticket (nothing calls anyone, and the
// display knows no waiter). Recall stands there instead (rule 4).

import { BikeIcon, CheckIcon, PlayIcon, ShoppingBagIcon, TimerIcon, Undo2Icon, UtensilsIcon, type LucideIcon } from "lucide-react";
import type { KitchenTicketResponse, ServiceType } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";
import { clockText } from "../../../shared/venue-text";
import { agoText, latePercent, metaText, timerText, timerTone, toneWord, whereText, type TimerTone } from "./kitchen-logic";

/** Which column a card is in. */
export type CardStage = "fresh" | "preparing" | "ready";

const serviceIcons: Readonly<Record<ServiceType, LucideIcon>> = { dine_in: UtensilsIcon, takeaway: ShoppingBagIcon, delivery: BikeIcon };

// The column's colour: the rule down the card's left, its border and its icon tile.
const stageClass: Readonly<Record<CardStage, { card: string; tile: string; qty: string }>> = {
  fresh: { card: "border-chip-danger-border border-l-attention", tile: "bg-chip-danger-bg text-chip-danger-fg", qty: "text-attention-text" },
  preparing: { card: "border-chip-warning-border border-l-highlight", tile: "bg-chip-warning-bg text-chip-warning-fg", qty: "text-attention-text" },
  ready: { card: "border-chip-success-border border-l-accent", tile: "bg-chip-success-bg text-chip-success-fg", qty: "text-chip-success-fg" },
};

// The timer's own tone: how long it has waited against the venue's late time.
const tonePill: Readonly<Record<TimerTone, string>> = {
  calm: "bg-chip-neutral-bg text-chip-neutral-fg",
  waiting: "bg-chip-warning-bg text-chip-warning-fg",
  late: "bg-chip-danger-bg text-chip-danger-fg",
};

const action =
  "mt-4 flex min-h-16 w-full items-center justify-center gap-2.5 rounded-pill text-xl font-semibold outline-offset-2 outline-line-focus focus-visible:outline-4 disabled:opacity-50";

interface TicketCardProps {
  ticket: KitchenTicketResponse;
  stage: CardStage;
  now: number;
  lateAfter: number;
  /** This card's own action is on its way. */
  busy: boolean;
  /** Any action is on its way: one at a time. */
  disabled: boolean;
  /** Start preparing, Mark ready, or Recall: what this column does. */
  onAct: () => void;
}

export function TicketCard({ ticket: t, stage, now, lateAfter, busy, disabled, onAct }: TicketCardProps) {
  const elapsed = now - Date.parse(t.sent_at);
  const tone = timerTone(elapsed, lateAfter);
  const word = toneWord[tone];
  const Icon = serviceIcons[t.service];
  const look = stageClass[stage];
  const placed = clockText(new Date(t.sent_at), null).split(" · ")[1] ?? "";
  const ready = stage === "ready";

  return (
    <article
      aria-label={ready ? `${whereText(t)}, ready` : `${whereText(t)}, ${timerText(elapsed)}${word ? `, ${word}` : ""}`}
      className={cx("rounded-card border border-l-8 bg-raised p-4 shadow-rest", look.card)}
    >
      <header className="flex items-start gap-3">
        <span aria-hidden="true" className={cx("flex size-12 shrink-0 items-center justify-center rounded", look.tile)}>
          <Icon size={24} strokeWidth={1.9} />
        </span>
        <div className="min-w-0 flex-1">
          {/* Where it goes: what a runner reads first. */}
          <h3 className="break-words font-serif text-total leading-tight">{whereText(t)}</h3>
          <p className="text-lg text-fg-muted">{metaText(t)}</p>
        </div>
        {ready ? (
          <span className="inline-flex min-h-10 shrink-0 items-center rounded-pill bg-chip-success-bg px-4 text-lg font-bold text-chip-success-fg">
            Ready{t.ready_at ? ` · ${agoText(now - Date.parse(t.ready_at))}` : ""}
          </span>
        ) : (
          <div className="flex shrink-0 flex-col items-end gap-1.5">
            <span className="font-mono text-base tabular-nums text-fg-muted">{placed}</span>
            <span className={cx("inline-flex min-h-10 items-center gap-2 rounded-pill px-4 font-bold", tonePill[tone])}>
              <TimerIcon aria-hidden="true" size={18} strokeWidth={2.2} />
              {word && <span className="text-base uppercase">{word}</span>}
              <span className="font-mono text-xl tabular-nums">{timerText(elapsed)}</span>
            </span>
          </div>
        )}
      </header>

      {t.notes.length > 0 && <p className="mt-3 rounded bg-chip-warning-bg px-3 py-2 text-lg text-chip-warning-fg">{t.notes.join(" · ")}</p>}

      <ul className="mt-3 grid gap-2 border-t border-line-soft pt-3">
        {t.lines.map((l) => {
          const voided = l.status === "voided";
          return (
            <li key={l.id} className={cx("flex gap-3", voided && "text-fg-muted line-through")}>
              <span className={cx("min-w-7 font-mono text-kitchen font-bold tabular-nums", !voided && look.qty)}>{l.qty}</span>
              <div className="min-w-0">
                <p className="text-kitchen font-semibold leading-tight">
                  {l.name}
                  {voided && <span className="ml-2 text-lg no-underline">Void</span>}
                </p>
                {!ready && l.modifiers.length > 0 && <p className="text-lg text-fg-2">{l.modifiers.join(", ")}</p>}
                {!ready &&
                  l.notes.map((n, i) => (
                    <p key={i} className="text-lg text-chip-warning-fg">
                      {n}
                    </p>
                  ))}
              </div>
            </li>
          );
        })}
      </ul>

      {stage === "preparing" && (
        <div aria-hidden="true" className="mt-4 h-2 rounded-pill bg-sunken">
          <div className={cx("h-2 rounded-pill", tone === "late" ? "bg-danger" : "bg-highlight")} style={{ width: `${latePercent(elapsed, lateAfter)}%` }} />
        </div>
      )}

      {stage === "fresh" && (
        <button type="button" disabled={disabled} onClick={onAct} className={cx(action, "bg-accent text-accent-fg hover:bg-accent-hover")}>
          <PlayIcon aria-hidden="true" size={22} strokeWidth={2.2} />
          {busy ? "Starting" : "Start preparing"}
        </button>
      )}
      {stage === "preparing" && (
        <button type="button" disabled={disabled} onClick={onAct} className={cx(action, "border border-line-strong bg-raised hover:bg-raised-high")}>
          <CheckIcon aria-hidden="true" size={22} strokeWidth={2.4} />
          {busy ? "Marking" : "Mark ready"}
        </button>
      )}
      {ready && (
        <button type="button" disabled={disabled} onClick={onAct} className={cx(action, "bg-accent-tint text-accent-text hover:bg-raised-high")}>
          <Undo2Icon aria-hidden="true" size={22} strokeWidth={2.2} />
          {busy ? "Recalling" : "Recall"}
        </button>
      )}
    </article>
  );
}
