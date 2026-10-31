// One table on the waiter's home (T01): a chair in the group's colour, the
// table's name, the group's word with its dot, and who is there and what they
// owe. The whole card is the button; it leads to the table.

import { ArmchairIcon, ChevronRightIcon } from "lucide-react";
import type { LiveTableResponse } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { WantsToOrder } from "../floor/FloorPlan";
import { toneClasses } from "../floor/tones";
import { cardMeta, cardWords } from "./waiter-logic";

interface TableCardProps {
  table: LiveTableResponse;
  names: ReadonlyMap<number, string>;
  onOpen: (table: LiveTableResponse) => void;
  /** A guest's phone has asked to order here, and nobody has answered. */
  wants?: boolean;
}

export function TableCard({ table, names, onOpen, wants = false }: TableCardProps) {
  const group = cardWords(table);
  const tone = toneClasses(group.tone);
  const owes = table.balance && table.balance.minor !== 0 ? table.balance : null;
  return (
    <button
      type="button"
      onClick={() => onOpen(table)}
      className={cx(
        "flex min-h-30 w-full items-center gap-3.5 rounded-card border p-4 text-left outline-offset-2 outline-focus focus-visible:outline-3",
        tone.border,
        tone.wash,
      )}
    >
      <span aria-hidden="true" className={cx("flex size-20 shrink-0 items-center justify-center rounded", tone.fill, tone.ink)}>
        <ArmchairIcon size={34} strokeWidth={1.5} />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate font-serif text-xl leading-none">{table.name}</span>
        <span className={cx("mt-1.5 flex items-center gap-2 text-base font-bold", tone.ink)}>
          <span aria-hidden="true" className={cx("size-2.5 shrink-0 rounded-round", tone.dot)} />
          {group.label}
        </span>
        {wants && <WantsToOrder className="mt-1.5" />}
        <span className="mt-1.5 block truncate text-sm text-fg">
          {cardMeta(table, names)}
          {owes && (
            <>
              {" · "}
              <Money amount={owes} />
            </>
          )}
        </span>
      </span>
      <ChevronRightIcon aria-hidden="true" size={22} className="shrink-0 text-fg-muted" />
    </button>
  );
}
