// "Tax and numbering": the tax profile, the tax classes and invoice numbering,
// one below the other, each saving on its own (backoffice-settings.md).

import { useCallback } from "react";
import { EmptyState, InboxIcon } from "../../../shared/ui";
import { NumberingCard } from "./NumberingCard";
import { problemOf } from "./settings-logic";
import { TaxClassesCard } from "./TaxClassesCard";
import { TaxProfileCard } from "./TaxProfileCard";
import { useNumbering, useTax } from "./use-tax";

export interface TaxSectionProps {
  /** Told, by card, whenever its draft starts or stops differing from what is saved. */
  onDirty: (key: string, dirty: boolean) => void;
}

export function TaxSection({ onDirty }: TaxSectionProps) {
  const tax = useTax();
  const numbering = useNumbering();
  const profileDirty = useCallback((d: boolean) => onDirty("tax-profile", d), [onDirty]);
  const numberingDirty = useCallback((d: boolean) => onDirty("tax-numbering", d), [onDirty]);

  const failed = tax.error ?? numbering.error;
  if (!tax.loading && !numbering.loading && failed !== null && failed !== undefined) {
    return (
      <EmptyState
        icon={<InboxIcon />}
        line={problemOf(failed).message}
        action={{
          label: "Try again",
          onClick: () => {
            tax.refetch();
            numbering.refetch();
          },
        }}
      />
    );
  }
  if (!tax.data || !numbering.data) return <p className="text-base text-fg-muted">Loading the tax settings</p>;

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-6">
      <TaxProfileCard tax={tax.data} onSave={tax.save} onDirty={profileDirty} />
      <TaxClassesCard classes={tax.data.classes} api={tax} />
      <NumberingCard numbering={numbering.data} api={numbering} onDirty={numberingDirty} />
    </div>
  );
}
