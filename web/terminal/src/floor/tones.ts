// A table's state in colour (D05, T01): the tile's border, the soft fill of the
// table drawn inside it, the saturated dot of the legend and the bars, and for
// the waiter's cards the ink of the word and a wash of the fill under the whole
// card. The word always goes with it (tokens.md).

import type { TableStatus } from "../../../shared/api-types";
import type { ChipTone } from "../../../shared/ui";
import { tableStatuses } from "../../../shared/ui/status";

export interface TableTone {
  readonly border: string;
  readonly fill: string;
  readonly dot: string;
  readonly ink: string;
  readonly wash: string;
}

// Written out, not built from the tone's name: Tailwind only makes the classes
// it can read whole in the source.
const tones: Readonly<Record<ChipTone, TableTone>> = {
  neutral: {
    border: "border-chip-neutral-border",
    fill: "bg-chip-neutral-bg",
    dot: "bg-chip-neutral-fg",
    ink: "text-chip-neutral-fg",
    wash: "bg-chip-neutral-bg/40",
  },
  success: {
    border: "border-chip-success-border",
    fill: "bg-chip-success-bg",
    dot: "bg-chip-success-fg",
    ink: "text-chip-success-fg",
    wash: "bg-chip-success-bg/40",
  },
  warning: {
    border: "border-chip-warning-border",
    fill: "bg-chip-warning-bg",
    dot: "bg-chip-warning-fg",
    ink: "text-chip-warning-fg",
    wash: "bg-chip-warning-bg/40",
  },
  danger: {
    border: "border-chip-danger-border",
    fill: "bg-chip-danger-bg",
    dot: "bg-chip-danger-fg",
    ink: "text-chip-danger-fg",
    wash: "bg-chip-danger-bg/40",
  },
  accent: {
    border: "border-chip-accent-border",
    fill: "bg-chip-accent-bg",
    dot: "bg-chip-accent-fg",
    ink: "text-chip-accent-fg",
    wash: "bg-chip-accent-bg/40",
  },
  info: { border: "border-chip-info-border", fill: "bg-chip-info-bg", dot: "bg-chip-info-fg", ink: "text-chip-info-fg", wash: "bg-chip-info-bg/40" },
};

export function tableTone(status: TableStatus): TableTone {
  return tones[tableStatuses[status].tone];
}

/** The same classes by tone, for a screen that groups the states its own way (T01). */
export function toneClasses(tone: ChipTone): TableTone {
  return tones[tone];
}
