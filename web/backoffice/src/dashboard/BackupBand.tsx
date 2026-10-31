// The dashboard's word on the backup (BO-11, ADR-381 item 7): a band over
// the day's figures, for whoever may change settings, while nothing is being
// backed up or the backup is failing. Settings says the rest.

import { TriangleAlertIcon } from "lucide-react";
import { Link } from "wouter";
import { cx } from "../../../shared/ui/cx";
import type { BackupStanding } from "../settings/backup-logic";

export function BackupBand({ standing }: { standing: BackupStanding }) {
  const failing = standing.tone === "danger";
  return (
    <div
      role="status"
      className={cx(
        "flex flex-wrap items-center gap-x-4 gap-y-2 rounded-card border px-5 py-3",
        failing ? "border-chip-danger-border bg-chip-danger-bg text-chip-danger-fg" : "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg",
      )}
    >
      <TriangleAlertIcon aria-hidden="true" size={20} className="shrink-0" />
      <p className="min-w-0 flex-1 text-base">{standing.sentence}</p>
      <Link
        href="/settings/backup"
        className="inline-flex min-h-tap items-center rounded-tight text-base font-semibold underline underline-offset-2 outline-offset-2 outline-focus focus-visible:outline-3"
      >
        {failing ? "See why" : "Set up the backup"}
      </Link>
    </div>
  );
}
