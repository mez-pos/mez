// The middle of the table's screen (T03): every item on the order, by course
// with where each course stands, or as one list. Fire is on the next course
// held back. Lines are changed on the order screen, where Add Item goes.

import { FlameIcon, PlusIcon, UtensilsIcon } from "lucide-react";
import { useState } from "react";
import type { LineResponse, OrderResponse } from "../../../shared/api-types";
import { Button, Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { courseGroups, readyLines, type CourseGroup } from "../register/send-logic";

interface TableItemsProps {
  order: OrderResponse;
  courseName: (course: number | undefined) => string | null;
  canFire: boolean;
  onFire: (course: number) => void;
  addDisabled: boolean;
  onAddItem: () => void;
}

const courseWords: Readonly<Record<CourseGroup["state"], { word: string; chip: string }>> = {
  sent: { word: "With the kitchen", chip: "bg-chip-success-bg text-chip-success-fg" },
  waiting: { word: "To send", chip: "bg-chip-warning-bg text-chip-warning-fg" },
  held: { word: "Held", chip: "bg-chip-info-bg text-chip-info-fg" },
};

/** A line's own state as a chip: colour plus a word. */
function lineChip(line: LineResponse, ready: boolean): { word: string; chip: string } | null {
  if (ready) return { word: "Ready", chip: "bg-chip-success-bg text-chip-success-fg" };
  if (line.sent) return { word: "In the kitchen", chip: "bg-chip-info-bg text-chip-info-fg" };
  if ((line.station ?? "") === "") return null;
  return { word: "Not sent", chip: "bg-chip-warning-bg text-chip-warning-fg" };
}

export function TableItems({ order, courseName, canFire, onFire, addDisabled, onAddItem }: TableItemsProps) {
  const [byCourse, setByCourse] = useState(true);
  const ready = readyLines(order);
  const lines = order.lines.filter((l) => l.status !== "voided");
  const groups = courseGroups(order, courseName);

  const row = (line: LineResponse) => {
    const chip = lineChip(line, ready.has(line.id));
    const mods = (line.modifiers ?? []).map((m) => m.name).join(", ");
    return (
      <li key={line.id} className="flex items-center gap-3 border-b border-line-soft py-2.5 last:border-b-0">
        <span className="min-w-5 font-mono text-base font-bold tabular-nums text-fg-muted">{line.qty}</span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-base font-bold">{line.name}</span>
          {mods && <span className="block truncate text-sm text-fg-muted">{mods}</span>}
        </span>
        {chip && <span className={cx("inline-flex min-h-7 shrink-0 items-center rounded-pill px-3 text-sm font-bold", chip.chip)}>{chip.word}</span>}
        <Money amount={line.gross} className="min-w-20 text-right font-semibold text-fg-heading" />
      </li>
    );
  };

  return (
    <section aria-label="Items" className="flex min-w-0 flex-1 flex-col overflow-hidden rounded-card border border-line bg-raised">
      <header className="flex items-center gap-2.5 border-b border-line px-4.5 py-4">
        <h2 className="inline-flex min-h-11 items-center rounded-pill bg-accent px-5 text-base font-semibold text-accent-fg">All Items ({lines.length})</h2>
        <button
          type="button"
          aria-pressed={byCourse}
          onClick={() => setByCourse((on) => !on)}
          className={cx(
            "ml-auto inline-flex min-h-tap items-center rounded-pill border px-4 text-base font-semibold outline-offset-2 outline-attention focus-visible:outline-3",
            byCourse ? "border-accent bg-accent-tint text-accent-text" : "border-line-strong hover:bg-raised-high",
          )}
        >
          Group by Course
        </button>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-4.5 py-4">
        {lines.length === 0 && (
          <div className="py-12 text-center">
            <p className="font-serif text-xl text-fg-muted">No items yet</p>
            <p className="mt-1.5 text-sm text-fg-muted">Add Item to start the order</p>
          </div>
        )}
        {byCourse ? (
          groups.map((group) => (
            <section key={group.course} aria-label={group.name} className="mb-5 last:mb-0">
              <div className="mb-2 flex items-center gap-2.5">
                <span aria-hidden="true" className="flex size-8 items-center justify-center rounded-tight bg-attention-tint text-attention">
                  <UtensilsIcon size={16} />
                </span>
                <h3 className="font-serif text-lg">
                  {group.name} ({group.lines.length})
                </h3>
                <span className={cx("ml-auto inline-flex min-h-7 items-center rounded-pill px-3 text-sm font-bold", courseWords[group.state].chip)}>
                  {courseWords[group.state].word}
                </span>
                {group.canFire && (
                  <Button disabled={!canFire} onClick={() => onFire(group.course)}>
                    <FlameIcon aria-hidden="true" size={17} />
                    Fire
                  </Button>
                )}
              </div>
              <ul>{group.lines.map(row)}</ul>
            </section>
          ))
        ) : (
          <ul>{lines.map(row)}</ul>
        )}
      </div>

      <footer className="border-t border-line px-4.5 py-3.5">
        <Button className="w-full" disabled={addDisabled} onClick={onAddItem}>
          <PlusIcon aria-hidden="true" size={18} />
          Add Item
        </Button>
      </footer>
    </section>
  );
}
