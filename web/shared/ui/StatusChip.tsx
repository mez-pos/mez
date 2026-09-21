// Colour and a word, always (tokens.md). Takes a status from one of the
// vocabularies in status.ts, never free text.

import { chipSpec, type ChipStatus, type ChipTone } from "./status";
import { cx } from "./cx";

const tones: Record<ChipTone, string> = {
  neutral: "bg-chip-neutral-bg border-chip-neutral-border text-chip-neutral-fg",
  success: "bg-chip-success-bg border-chip-success-border text-chip-success-fg",
  warning: "bg-chip-warning-bg border-chip-warning-border text-chip-warning-fg",
  danger: "bg-chip-danger-bg border-chip-danger-border text-chip-danger-fg",
  accent: "bg-chip-accent-bg border-chip-accent-border text-chip-accent-fg",
  info: "bg-chip-info-bg border-chip-info-border text-chip-info-fg",
};

export interface StatusChipProps {
  status: ChipStatus;
  className?: string;
}

export function StatusChip({ status, className }: StatusChipProps) {
  const { tone, word, struck } = chipSpec(status);
  return (
    <span
      className={cx(
        "inline-flex items-center gap-1.5 whitespace-nowrap rounded-tight border px-2.5 py-0.5 text-sm font-semibold",
        tones[tone],
        className,
      )}
    >
      <span aria-hidden="true" className="size-1.5 shrink-0 rounded-round bg-current" />
      <span className={struck ? "line-through" : undefined}>{word}</span>
    </span>
  );
}
