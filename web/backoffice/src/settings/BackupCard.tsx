// Where the venue's backup goes: nowhere, a folder, or a bucket
// (backoffice-settings.md, "Backup"; ADR-276). Saving tries the place
// first, so a save here can take twenty seconds and says so.

import type { BackupRequest, BackupResponse } from "../../../shared/api-types";
import { Choice, Field, inputClass } from "../FormFields";
import { BACKUP_KINDS, backupChanged, backupFormFrom, backupProblem, backupProblemOf, secretKept, toBackupInput, type BackupForm } from "./backup-logic";
import { useAway } from "../../../shared/use-realtime";
import { SectionCard } from "./SectionCard";
import { useSectionDraft } from "./use-section-draft";

export interface BackupCardProps {
  backup: BackupResponse;
  /** Saves, and resolves with the backup as the server now holds it. */
  onSave: (input: BackupRequest) => Promise<BackupResponse>;
  onDirty: (dirty: boolean) => void;
}

const mono = `${inputClass} font-mono`;
// What a path, a name or a key must not have done to it by a keyboard.
const verbatim = { autoComplete: "off", autoCapitalize: "none", autoCorrect: "off", spellCheck: false } as const;

export function BackupCard({ backup, onSave, onDirty }: BackupCardProps) {
  const away = useAway();
  const saved = backupFormFrom(backup);
  const secretSet = backup.s3.secret_set;
  const d = useSectionDraft({
    saved,
    changed: backupChanged,
    check: (f) => backupProblem(f, saved, secretSet),
    send: (f) => onSave(toBackupInput(f)).then(backupFormFrom),
    problemOf: backupProblemOf,
    onDirty,
  });
  const { form, busy } = d;
  const set = (patch: Partial<BackupForm>) => d.update((f) => ({ ...f, ...patch }));
  const kept = secretKept(form, saved, secretSet);

  return (
    <SectionCard
      readOnly={away}
      title="Where the backup goes"
      saves="backup"
      description="Saving tries the place first: mez writes a small note there, and keeps the setting only if it could."
      dirty={d.dirty}
      busy={busy}
      saved={d.done}
      error={d.footerError}
      onSave={d.save}
      onDiscard={d.discard}
    >
      <Choice label="Back up to" value={form.kind} options={BACKUP_KINDS} onChange={(kind) => set({ kind })} error={d.on("kind")} />

      {form.kind === "" && saved.kind !== "" && (
        <div className="rounded border border-chip-warning-border bg-chip-warning-bg p-4 text-chip-warning-fg">
          <p className="text-base font-semibold">Saving this stops the backup.</p>
          <p className="mt-1 text-base">What is already at the destination stays there. The venue's only up-to-date copy will be this computer's disk.</p>
        </div>
      )}

      {form.kind === "folder" && (
        <Field
          label="Folder"
          hint="Written in full, like D:\mez-backup or \\nas\backups\cafe. On another disk, a USB drive or a network share: a folder on this computer's own disk is lost with it."
          error={d.on("folder")}
        >
          {(id, aria) => (
            <input id={id} {...aria} className={mono} value={form.folder} disabled={busy} {...verbatim} onChange={(e) => set({ folder: e.target.value })} />
          )}
        </Field>
      )}

      {form.kind === "s3" && (
        <>
          <p className="text-base text-fg-2">Amazon S3, Cloudflare R2, or any other service that speaks S3.</p>
          <Field label="Bucket" hint="The bucket's name." error={d.on("bucket")}>
            {(id, aria) => (
              <input id={id} {...aria} className={mono} value={form.bucket} disabled={busy} {...verbatim} onChange={(e) => set({ bucket: e.target.value })} />
            )}
          </Field>
          <Field label="Folder in the bucket" hint="Leave empty to use mez." error={d.on("path")}>
            {(id, aria) => (
              <input
                id={id}
                {...aria}
                className={mono}
                value={form.path}
                placeholder="mez"
                disabled={busy}
                {...verbatim}
                onChange={(e) => set({ path: e.target.value })}
              />
            )}
          </Field>
          <Field
            label="Endpoint"
            hint="For Cloudflare R2 and other services, their address, like https://account.r2.cloudflarestorage.com. Leave empty for Amazon S3."
            error={d.on("endpoint")}
          >
            {(id, aria) => (
              <input
                id={id}
                {...aria}
                className={mono}
                value={form.endpoint}
                disabled={busy}
                inputMode="url"
                {...verbatim}
                onChange={(e) => set({ endpoint: e.target.value })}
              />
            )}
          </Field>
          <Field label="Region" hint="Amazon S3 needs it, like ap-south-1. Others can leave it empty." error={d.on("region")}>
            {(id, aria) => (
              <input id={id} {...aria} className={mono} value={form.region} disabled={busy} {...verbatim} onChange={(e) => set({ region: e.target.value })} />
            )}
          </Field>
          <Field label="Access key ID" error={d.on("access_key_id")}>
            {(id, aria) => (
              <input
                id={id}
                {...aria}
                className={mono}
                value={form.accessKeyId}
                disabled={busy}
                {...verbatim}
                onChange={(e) => set({ accessKeyId: e.target.value })}
              />
            )}
          </Field>
          <Field
            label="Secret access key"
            hint={
              kept
                ? "One is saved, and is not shown here or anywhere. Leave this empty to keep it."
                : "It is sent to mez once and not shown again, here or anywhere."
            }
            error={d.on("secret_access_key")}
          >
            {(id, aria) => (
              <input
                id={id}
                {...aria}
                type="password"
                className={mono}
                value={form.secret}
                placeholder={kept ? "Saved. Leave empty to keep it." : undefined}
                disabled={busy}
                // Not a sign-in: a password manager should neither fill it nor offer to keep it.
                autoComplete="new-password"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
                onChange={(e) => set({ secret: e.target.value })}
              />
            )}
          </Field>
        </>
      )}

      {busy && (
        <p role="status" className="text-base text-fg-2">
          Trying the destination. This can take up to twenty seconds.
        </p>
      )}
    </SectionCard>
  );
}
