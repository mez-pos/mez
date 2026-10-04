// A table's state in colour, three ways (D05): the tile's border, the soft
// fill of the table drawn inside it, and the saturated dot of the legend and
// the bars. The word always goes with it (tokens.md): StatusChip says it.

import type { TableStatus } from "../../../shared/api-types";
import type { ChipTone } from "../../../shared/ui";
import { tableStatuses } from "../../../shared/ui/status";

export interface TableTone {
  readonly border: string;
  readonly fill: string;
  readonly dot: string;
}

// Written out, not built from the tone's name: Tailwind only makes the classes
// it can read whole in the source.
const tones: Readonly<Record<ChipTone, TableTone>> = {
  neutral: { border: "border-chip-neutral-border", fill: "bg-chip-neutral-bg", dot: "bg-chip-neutral-fg" },
  success: { border: "border-chip-success-border", fill: "bg-chip-success-bg", dot: "bg-chip-success-fg" },
  warning: { border: "border-chip-warning-border", fill: "bg-chip-warning-bg", dot: "bg-chip-warning-fg" },
  danger: { border: "border-chip-danger-border", fill: "bg-chip-danger-bg", dot: "bg-chip-danger-fg" },
  accent: { border: "border-chip-accent-border", fill: "bg-chip-accent-bg", dot: "bg-chip-accent-fg" },
  info: { border: "border-chip-info-border", fill: "bg-chip-info-bg", dot: "bg-chip-info-fg" },
};

export function tableTone(status: TableStatus): TableTone {
  return tones[tableStatuses[status].tone];
}
