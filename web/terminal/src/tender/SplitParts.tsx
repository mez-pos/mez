// What the split step is drawn from (D08a), while a split is being made and
// once it is in use: the frame (a title bar, the mode column, the guests, a
// footer bar), one guest's card, and the pill saying whether they have paid.

import { XIcon } from "lucide-react";
import type { ReactNode } from "react";
import type { Money as Amount } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";

interface SplitFrameProps {
  /** "Split bill · Table T4". */
  heading: string;
  /** What is being split, named: "Total", or "Still owed" after a payment. */
  totalLabel: string;
  total: Amount;
  aside: ReactNode;
  footer: ReactNode;
  children: ReactNode;
}

export function SplitFrame({ heading, totalLabel, total, aside, footer, children }: SplitFrameProps) {
  return (
    <section aria-label="Split bill" className="flex min-h-96 flex-1 flex-col overflow-hidden rounded-sheet border border-line bg-canvas shadow-card">
      <header className="flex min-h-16 shrink-0 items-center gap-3.5 border-b border-line bg-chrome px-6">
        <h2 className="min-w-0 truncate font-serif text-xl">{heading}</h2>
        <p className="ml-auto flex shrink-0 items-baseline gap-2 text-base text-fg-muted">
          {totalLabel} <Money amount={total} className="font-bold text-fg" />
        </p>
      </header>
      <div className="flex min-h-0 flex-1">
        <aside className="w-72 shrink-0 overflow-y-auto border-r border-line bg-chrome p-5 2xl:w-82">{aside}</aside>
        <div className="grid min-w-0 flex-1 grid-cols-2 content-start gap-4 overflow-y-auto p-5">{children}</div>
      </div>
      <footer className="flex min-h-20 shrink-0 flex-wrap items-center gap-3.5 border-t border-line bg-chrome px-6 py-3">{footer}</footer>
    </section>
  );
}

/** A split mode's tile, and how the chosen one is drawn. */
export const modeTile = "flex min-h-13 w-full items-center gap-3 rounded border px-4 text-left text-base";
export const modeTileOn = "border-accent bg-accent-tint font-bold text-accent-text";

/** The small capital heading over a group in the mode column. */
export function ColumnHeading({ children }: { children: ReactNode }) {
  return <h3 className="mb-3.5 text-meta font-bold uppercase text-fg-muted">{children}</h3>;
}

/** A line on a guest's card. */
export interface ShareLine {
  id: number;
  /** "1 × Chicken Biryani". */
  label: string;
  /** The line's own figure, when this guest alone pays for it. */
  amount: Amount | null;
  /** How many guests share it, when more than one. */
  sharedBy: number;
  /** Takes this guest off the line. Absent once the split is in use. */
  onRemove?: (() => void) | undefined;
}

interface GuestCardProps {
  /** Counted from one. */
  seat: number;
  name: string;
  paid: boolean;
  lines: readonly ShareLine[];
  /** The server's figure for this guest, once it has answered. */
  due: Amount | undefined;
  /** The figure shown is the last answer, and the next is on its way. */
  settling?: boolean;
  /** The guest the next item tapped goes to. */
  selected?: boolean;
  onSelect?: (() => void) | undefined;
  /** Collect, or Remove: what can be done with this share. */
  children?: ReactNode;
}

export function GuestCard({ seat, name, paid, lines, due, settling = false, selected = false, onSelect, children }: GuestCardProps) {
  const title = (
    <>
      <span
        aria-hidden="true"
        className={cx(
          "flex size-9 shrink-0 items-center justify-center rounded-round text-base font-bold",
          paid || selected ? "bg-accent-tint text-accent-text" : "bg-sunken text-fg-muted",
        )}
      >
        {seat}
      </span>
      <span className="min-w-0 truncate text-lg font-bold text-fg">{name}</span>
    </>
  );
  return (
    <article
      aria-label={name}
      className={cx("flex flex-col rounded-card border bg-raised p-4", selected ? "border-accent outline-3 outline-accent-tint" : "border-line")}
    >
      <div className="flex items-center gap-2.5">
        {onSelect ? (
          <button
            type="button"
            aria-pressed={selected}
            onClick={onSelect}
            className="-m-1 flex min-h-tap min-w-0 flex-1 items-center gap-2.5 rounded p-1 text-left outline-offset-2 outline-attention focus-visible:outline-3"
          >
            {title}
          </button>
        ) : (
          <div className="flex min-w-0 flex-1 items-center gap-2.5">{title}</div>
        )}
        <SharePill paid={paid} />
      </div>

      <div className="mt-3 grid gap-2 text-base">
        {lines.length > 0 && (
          <ul className="grid gap-1">
            {lines.map((l) => (
              <li key={l.id} className="flex items-center gap-2">
                <span className="min-w-0 flex-1">
                  {l.label}
                  {l.sharedBy > 1 && <span className="block text-sm text-fg-muted">Shared by {l.sharedBy}</span>}
                </span>
                {l.amount && <Money amount={l.amount} />}
                {l.onRemove && (
                  <button
                    type="button"
                    aria-label={`Take ${l.label} off ${name}`}
                    onClick={l.onRemove}
                    className="flex size-tap shrink-0 items-center justify-center rounded-round text-fg-muted outline-offset-2 outline-attention hover:bg-raised-high focus-visible:outline-3"
                  >
                    <XIcon aria-hidden="true" size={18} />
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
        {lines.length > 0 && <div aria-hidden="true" className="h-px bg-line-soft" />}
        <div className="flex items-baseline justify-between gap-3">
          <strong>Due</strong>
          {due ? <Money amount={due} variant="heading" className={cx(settling && "opacity-50")} /> : <span className="text-fg-muted">…</span>}
        </div>
      </div>

      {children && <div className="mt-4 grid">{children}</div>}
    </article>
  );
}

/** Paid or Pending: a colour and a word (tokens.md). */
function SharePill({ paid }: { paid: boolean }) {
  return (
    <span
      className={cx(
        "inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-pill border px-3 py-1 text-sm font-semibold",
        paid ? "border-chip-success-border bg-chip-success-bg text-chip-success-fg" : "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg",
      )}
    >
      <span aria-hidden="true" className="size-1.5 shrink-0 rounded-round bg-current" />
      {paid ? "Paid" : "Pending"}
    </span>
  );
}
