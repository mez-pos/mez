// How the backup is doing: a chip, a sentence, the reason when something is
// wrong, and "Back up now" (backoffice-settings.md, "Backup"). It shows
// what is true now; where the backup goes is the card under it.

import { useState } from "react";
import type { BackupResponse, BackupState } from "../../../shared/api-types";
import { Button, Card, type ChipTone } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useAway } from "../../../shared/use-realtime";
import { clockText } from "../../../shared/venue-text";
import { backUpNowProblem, backupStanding, destinationText } from "./backup-logic";

// The status chip's tones (StatusChip.tsx). That chip takes a status from a
// vocabulary of the floor's and the kitchen's; this is neither.
const tones: Record<ChipTone, string> = {
  neutral: "bg-chip-neutral-bg border-chip-neutral-border text-chip-neutral-fg",
  success: "bg-chip-success-bg border-chip-success-border text-chip-success-fg",
  warning: "bg-chip-warning-bg border-chip-warning-border text-chip-warning-fg",
  danger: "bg-chip-danger-bg border-chip-danger-border text-chip-danger-fg",
  accent: "bg-chip-accent-bg border-chip-accent-border text-chip-accent-fg",
  info: "bg-chip-info-bg border-chip-info-border text-chip-info-fg",
};

export interface BackupStatusCardProps {
  backup: BackupResponse;
  /** Copies everything now, and resolves with the backup as it then stands. */
  onBackUpNow: () => Promise<BackupResponse>;
}

// What was last said about asking, and how the backup stood when it was
// said. The words are about the backup as it then was: once it stands
// otherwise (it recovers, it is turned off) they are no longer shown.
type Asked = { state: "idle" } | { state: "busy" } | { state: "done"; at: BackupState } | { state: "failed"; at: BackupState; message: string };

export function BackupStatusCard({ backup, onBackUpNow }: BackupStatusCardProps) {
  const [asked, setAsked] = useState<Asked>({ state: "idle" });
  // A backup is started by someone at the venue (ADR-308).
  const away = useAway();
  const standing = backupStanding(backup.status, (at) => clockText(at, null));
  const where = destinationText(backup);
  const now = backup.status.state;
  const running = now !== "off";

  const backUpNow = () => {
    setAsked({ state: "busy" });
    onBackUpNow().then(
      (next) => setAsked({ state: "done", at: next.status.state }),
      // A refusal brings no new status with it: it is about the one on show.
      (err: unknown) => setAsked({ state: "failed", at: now, message: backUpNowProblem(err) }),
    );
  };

  return (
    <Card padding="lg" className="min-w-0 md:p-8">
      <h2 className="font-serif text-xl">How the backup is doing</h2>
      <p className="mt-1 text-base text-fg-2">A second copy of everything the venue has sold, kept somewhere other than this computer.</p>

      <div className="mt-6 flex max-w-xl flex-col gap-4">
        <div role="status" className="flex flex-col gap-2">
          <span
            className={cx(
              "inline-flex items-center gap-1.5 self-start whitespace-nowrap rounded-pill border px-3 py-1 text-sm font-semibold",
              tones[standing.tone],
            )}
          >
            <span aria-hidden="true" className="size-1.5 shrink-0 rounded-round bg-current" />
            {standing.word}
          </span>
          <p className="text-base">{standing.sentence}</p>
        </div>

        {standing.reason && (
          <div className="rounded border border-line bg-sunken p-4">
            <p className="text-meta font-bold uppercase text-fg-faint">What mez was told</p>
            <p className="mt-1 wrap-break-word font-mono text-sm text-fg-2">{standing.reason}</p>
          </div>
        )}
        {standing.lastGood && <p className="text-base text-fg-2">{standing.lastGood}</p>}

        {where && (
          <p className="text-base text-fg-2">
            Saved destination: <span className="break-all font-mono text-fg">{where}</span>
          </p>
        )}

        {backup.restart_needed && (
          <div className="rounded border border-chip-warning-border bg-chip-warning-bg p-4 text-chip-warning-fg">
            <p className="text-base font-semibold">Restart mez to use the new destination.</p>
            <p className="mt-1 text-base">
              The destination below is saved and not yet in use. mez is still backing up to the one from before, and changes over when it is next restarted:
              restart the computer, or the mez service.
            </p>
          </div>
        )}
      </div>

      {running && !away && (
        <div className="mt-8 flex flex-wrap items-center gap-4 border-t border-line pt-6">
          <Button disabled={asked.state === "busy"} onClick={backUpNow}>
            {asked.state === "busy" ? "Backing up" : "Back up now"}
          </Button>
          {asked.state === "busy" && <p className="text-base text-fg-2">This can take up to half a minute.</p>}
          {asked.state === "done" && asked.at === now && now === "ok" && (
            <p role="status" className="text-base font-semibold text-accent-text">
              Everything is at the destination.
            </p>
          )}
          {asked.state === "failed" && asked.at === now && (
            <p role="alert" className="min-w-0 basis-full wrap-break-word text-sm text-danger">
              {asked.message}
            </p>
          )}
        </div>
      )}
    </Card>
  );
}
