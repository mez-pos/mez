// Course View (T02's second tab): the order by course, so a waiter can see what
// the kitchen has, what goes with the next Send, and what is held back, and fire
// the next course when the table is ready for it. Nothing is changed here but by
// Fire: lines are stepped and removed under Current Order.

import { FlameIcon } from "lucide-react";
import type { LineResponse, OrderResponse } from "../../../shared/api-types";
import { Button } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { courseGroups, type CourseGroup } from "./send-logic";

interface CourseViewProps {
  order: OrderResponse | undefined;
  courseName: (course: number | undefined) => string | null;
  /** Lines whose ticket is on the pass (ADR-090). */
  ready: ReadonlySet<number>;
  /** Fire is not offered on a saved order. */
  canFire: boolean;
  onFire: (course: number) => void;
}

// A word and its chip colours for each state. Colour plus a word (tokens.md).
const states: Readonly<Record<CourseGroup["state"], { word: string; chip: string }>> = {
  sent: { word: "With the kitchen", chip: "bg-chip-success-bg text-chip-success-fg" },
  waiting: { word: "To send", chip: "bg-chip-warning-bg text-chip-warning-fg" },
  held: { word: "Held", chip: "bg-chip-info-bg text-chip-info-fg" },
};

function lineState(line: LineResponse, ready: boolean): string {
  if (ready) return "Ready";
  if (line.sent) return "With the kitchen";
  return (line.station ?? "") === "" ? "" : "Not sent";
}

export function CourseView({ order, courseName, ready, canFire, onFire }: CourseViewProps) {
  const groups = courseGroups(order, courseName);
  if (groups.length === 0) {
    return (
      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-12 text-center">
        <p className="font-serif text-xl text-fg-muted">No items yet</p>
        <p className="mt-1.5 text-sm text-fg-muted">Tap a dish to start the order</p>
      </div>
    );
  }
  return (
    <div className="min-h-0 flex-1 overflow-y-auto px-5 py-3">
      {groups.map((group) => (
        <section key={group.course} aria-label={group.name} className="border-b border-line-soft py-3.5 last:border-b-0">
          <div className="flex items-center gap-3">
            <h3 className="min-w-0 flex-1 truncate text-base font-bold text-attention-text">{group.name}</h3>
            <span className={cx("inline-flex min-h-7 items-center rounded-pill px-3 text-sm font-bold", states[group.state].chip)}>
              {states[group.state].word}
            </span>
          </div>
          <ul className="mt-2 grid gap-1.5">
            {group.lines.map((line) => (
              <li key={line.id} className="flex items-baseline gap-3 text-base">
                <span className="w-7 shrink-0 font-mono font-bold tabular-nums text-fg-muted">{line.qty}×</span>
                <span className="min-w-0 flex-1 truncate font-semibold">{line.name}</span>
                <span className="shrink-0 text-sm text-fg-muted">{lineState(line, ready.has(line.id))}</span>
              </li>
            ))}
          </ul>
          {group.canFire && canFire && (
            <Button className="mt-3 w-full" onClick={() => onFire(group.course)}>
              <FlameIcon aria-hidden="true" size={17} />
              Fire {group.name}
            </Button>
          )}
        </section>
      ))}
    </div>
  );
}
