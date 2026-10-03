// "Shift, tips and receipt": the shift and tip rules, and the receipt template,
// one below the other, each saving on its own (backoffice-settings.md).

import { useCallback } from "react";
import { EmptyState, InboxIcon } from "../../../shared/ui";
import { ReceiptCard } from "./ReceiptCard";
import { RulesCard } from "./RulesCard";
import { problemOf } from "./settings-logic";
import { useReceipt, useRules } from "./use-shifts";

export interface ShiftsSectionProps {
  /** Told, by card, whenever its draft starts or stops differing from what is saved. */
  onDirty: (key: string, dirty: boolean) => void;
}

export function ShiftsSection({ onDirty }: ShiftsSectionProps) {
  const rules = useRules();
  const receipt = useReceipt();
  const rulesDirty = useCallback((d: boolean) => onDirty("shift-rules", d), [onDirty]);
  const receiptDirty = useCallback((d: boolean) => onDirty("receipt", d), [onDirty]);

  const failed = rules.error ?? receipt.error;
  if (!rules.loading && !receipt.loading && failed !== null && failed !== undefined) {
    return (
      <EmptyState
        icon={<InboxIcon />}
        line={problemOf(failed).message}
        action={{
          label: "Try again",
          onClick: () => {
            rules.refetch();
            receipt.refetch();
          },
        }}
      />
    );
  }
  if (!rules.data || !receipt.data) return <p className="text-base text-fg-muted">Loading the shift and receipt settings</p>;

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-6">
      <RulesCard rules={rules.data} onSave={rules.save} onDirty={rulesDirty} />
      <ReceiptCard receipt={receipt.data} api={receipt} onDirty={receiptDirty} />
    </div>
  );
}
