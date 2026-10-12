// "Backup": how the venue's backup is doing, and where it goes, one below
// the other (backoffice-settings.md; ADR-277). The first is read again every
// few seconds; the second is a form that saves on its own.

import { useCallback } from "react";
import { OnAMirror } from "../../../shared/shell/OnAMirror";
import { Card, EmptyState, InboxIcon } from "../../../shared/ui";
import { useOnMirror } from "../../../shared/use-mirror";
import { BackupCard } from "./BackupCard";
import { BackupStatusCard } from "./BackupStatusCard";
import { problemOf } from "./settings-logic";
import { useBackup } from "./use-backup";

export interface BackupSectionProps {
  /** Told, by card, whenever its draft starts or stops differing from what is saved. */
  onDirty: (key: string, dirty: boolean) => void;
}

export function BackupSection({ onDirty }: BackupSectionProps) {
  // A mirror is made from the backup and has none of its own: asked, it
  // answers that there is no backup here (404), which is not what is true
  // of the venue. So it is not asked (ADR-330).
  return useOnMirror() ? <MirrorBackup /> : <VenueBackup onDirty={onDirty} />;
}

function MirrorBackup() {
  return (
    <Card padding="lg" className="min-w-0 flex-1 md:p-8">
      <h2 className="font-serif text-xl">Backup</h2>
      <p className="mt-1 max-w-prose text-base text-fg-2">
        What you are reading is the venue's backup. The time at the top of the page is when the venue last added to it.
      </p>
      <OnAMirror className="mt-6">Where the backup goes, and how it is doing, is seen and changed at the venue.</OnAMirror>
    </Card>
  );
}

function VenueBackup({ onDirty }: BackupSectionProps) {
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
