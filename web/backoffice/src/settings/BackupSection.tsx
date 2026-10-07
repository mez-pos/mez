// "Backup": how the venue's backup is doing, and where it goes, one below
// the other (backoffice-settings.md; ADR-277). The first is read again every
// few seconds; the second is a form that saves on its own.

import { useCallback } from "react";
import { EmptyState, InboxIcon } from "../../../shared/ui";
import { BackupCard } from "./BackupCard";
import { BackupStatusCard } from "./BackupStatusCard";
import { problemOf } from "./settings-logic";
import { useBackup } from "./use-backup";

export interface BackupSectionProps {
  /** Told, by card, whenever its draft starts or stops differing from what is saved. */
  onDirty: (key: string, dirty: boolean) => void;
}

export function BackupSection({ onDirty }: BackupSectionProps) {
  const backup = useBackup();
  const dirty = useCallback((d: boolean) => onDirty("backup", d), [onDirty]);

  // A read that fails with the cards already drawn (the fifteen-second one,
  // with the server restarting) leaves them drawn: what is typed is not
  // thrown away for a status that will be read again.
  if (!backup.data) {
    if (backup.loading) return <p className="text-base text-fg-muted">Loading the backup settings</p>;
    return <EmptyState icon={<InboxIcon />} line={problemOf(backup.error).message} action={{ label: "Try again", onClick: backup.refetch }} />;
  }

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-6">
      <BackupStatusCard backup={backup.data} onBackUpNow={backup.backUpNow} />
      <BackupCard backup={backup.data} onSave={backup.save} onDirty={dirty} />
    </div>
  );
}
