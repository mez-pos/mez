// Where the venue's backup goes and how it is doing, as a form and as words
// (backoffice-settings.md, "Backup"; ADR-276, ADR-277). No React here. The
// server checks everything again, and tries the destination before it keeps
// it; these checks only spare a round trip that can take twenty seconds.

import { ApiError } from "../../../shared/api";
import type { BackupKind, BackupRequest, BackupResponse, BackupState } from "../../../shared/api-types";
import type { ChipTone } from "../../../shared/ui";
import { problemWith, type Problem } from "./problem";

export interface BackupForm {
  kind: BackupKind;
  folder: string;
  bucket: string;
  /** The folder inside the bucket. */
  path: string;
  endpoint: string;
  region: string;
  accessKeyId: string;
  /** Typed this visit. Never filled from the server, which never sends it. */
  secret: string;
}

/** The server's names for the parts of a destination, as its refusals give them. */
export type BackupField = "kind" | "folder" | "bucket" | "path" | "endpoint" | "region" | "access_key_id" | "secret_access_key";

const fields: readonly BackupField[] = ["kind", "folder", "bucket", "path", "endpoint", "region", "access_key_id", "secret_access_key"];

/** The three places a backup can go, in the order the card offers them. */
export const BACKUP_KINDS: readonly { value: BackupKind; label: string }[] = [
  { value: "", label: "Nowhere" },
  { value: "folder", label: "A folder" },
  { value: "s3", label: "A bucket" },
];

export function backupFormFrom(b: BackupResponse): BackupForm {
  return {
    kind: b.kind,
    folder: b.folder,
    bucket: b.s3.bucket,
    path: b.s3.path,
    endpoint: b.s3.endpoint,
    region: b.s3.region,
    accessKeyId: b.s3.access_key_id,
    secret: "",
  };
}

/** An endpoint as the server keeps it: no spaces round it and no slash on the end. */
function tidyEndpoint(endpoint: string): string {
  return endpoint.trim().replace(/\/+$/, "");
}

/**
 * The form as a request. Only the chosen kind's fields are sent: the others
 * are ignored by the server, and a secret typed for a bucket the owner then
 * moved away from has no business leaving the browser.
 */
export function toBackupInput(f: BackupForm): BackupRequest {
  const none = { bucket: "", path: "", region: "", endpoint: "", access_key_id: "", secret_access_key: "" };
  if (f.kind === "folder") return { kind: "folder", folder: f.folder.trim(), s3: none };
  if (f.kind === "s3") {
    return {
      kind: "s3",
      folder: "",
      s3: {
        bucket: f.bucket.trim(),
        path: f.path.trim(),
        region: f.region.trim(),
        endpoint: tidyEndpoint(f.endpoint),
        access_key_id: f.accessKeyId.trim(),
        // Not trimmed: what a secret starts and ends with is not ours to say.
        secret_access_key: f.secret,
      },
    };
  }
  return { kind: "", folder: "", s3: none };
}

/** Whether the draft differs from what is saved, looking only at the kind chosen. */
export function backupChanged(draft: BackupForm, saved: BackupForm): boolean {
  if (draft.kind !== saved.kind) return true;
  if (draft.kind === "folder") return draft.folder.trim() !== saved.folder.trim();
  if (draft.kind === "s3") return bucketMoved(draft, saved) || draft.path.trim() !== saved.path.trim() || draft.secret !== "";
  return false;
}

/**
 * Whether the bucket, where it is, or the key id differ from what is saved.
 * These are what a saved secret was given for: change one and the server
 * will not lend the old secret to the new place (ADR-276).
 */
function bucketMoved(draft: BackupForm, saved: BackupForm): boolean {
  return (
    draft.bucket.trim() !== saved.bucket.trim() ||
    tidyEndpoint(draft.endpoint) !== tidyEndpoint(saved.endpoint) ||
    draft.region.trim() !== saved.region.trim() ||
    draft.accessKeyId.trim() !== saved.accessKeyId.trim()
  );
}

/**
 * Whether a secret left empty will be kept by the server: one is saved, the
 * saved destination is a bucket, and nothing it was given for has changed.
 */
export function secretKept(draft: BackupForm, saved: BackupForm, secretSet: boolean): boolean {
  return secretSet && saved.kind === "s3" && !bucketMoved(draft, saved);
}

/** A full path as Windows or anything else writes one: a drive, a share, or a leading slash. */
const FULL_PATH = /^([A-Za-z]:[\\/]|\\\\[^\\/]+[\\/][^\\/]+|\/)/;

/** A bucket's name by the rule S3 and its copies share. */
const BUCKET = /^[a-z0-9][a-z0-9.-]*[a-z0-9]$/;

export function backupProblem(f: BackupForm, saved: BackupForm, secretSet: boolean): Problem<BackupField> | null {
  if (f.kind === "folder") {
    const folder = f.folder.trim();
    if (folder === "") return { field: "folder", message: "Enter the folder to back up to." };
    if (!FULL_PATH.test(folder)) {
      return { field: "folder", message: "Write the folder in full, starting from the drive or the share: D:\\mez-backup, not mez-backup." };
    }
    return null;
  }
  if (f.kind !== "s3") return null;

  const bucket = f.bucket.trim();
  if (bucket.length < 3 || bucket.length > 63 || !BUCKET.test(bucket) || /\.\.|\.-|-\./.test(bucket)) {
    return { field: "bucket", message: "Enter the bucket's name: 3 to 63 small letters, digits, dots and hyphens." };
  }
  const path = f.path.trim();
  if (path.startsWith("/") || path.endsWith("/") || path.includes("\\")) {
    return { field: "path", message: "Write the folder like venues/cafe, with no slash at either end." };
  }
  const endpoint = tidyEndpoint(f.endpoint);
  if (endpoint.includes("@")) {
    return { field: "endpoint", message: "The address must not have a key in it. The key goes in the two boxes below." };
  }
  if (endpoint !== "" && !/^https?:\/\/[^/\s]+$/.test(endpoint)) {
    return { field: "endpoint", message: "Enter the service's address, starting with https://, and nothing after its name." };
  }
  if (endpoint === "" && f.region.trim() === "") {
    return { field: "region", message: "Amazon S3 needs the bucket's region, like ap-south-1. For another service, enter its address above." };
  }
  if (f.accessKeyId.trim() === "") return { field: "access_key_id", message: "Enter the access key ID." };
  // No secret these services issue has a space in it, and one pasted with
  // a space on the end is refused by the bucket in words that do not say so.
  if (f.secret !== f.secret.trim()) {
    return { field: "secret_access_key", message: "The secret has a space at its start or end. Paste it again without one." };
  }
  if (f.secret === "" && !secretKept(f, saved, secretSet)) {
    return {
      field: "secret_access_key",
      message: secretSet && saved.kind === "s3" ? "Type the secret again: it was saved for a different bucket or key." : "Enter the secret access key.",
    };
  }
  return null;
}

/**
 * A refused save, as words and the field to put them under. A destination
 * that was written correctly and could not be written to belongs to no
 * field: it is said at the foot, with the destination's own reason.
 */
export function backupProblemOf(err: unknown): Problem<BackupField> {
  if (err instanceof ApiError && err.code === "backup_unusable") {
    return { field: null, message: `mez could not write there, so nothing was changed. The reason it was given: ${reasonText(err.message)}` };
  }
  if (err instanceof ApiError && err.code === "timeout") {
    return { field: null, message: "mez did not answer in time. Look at how the backup is doing, above, before trying again: the save may have gone through." };
  }
  return problemWith(err, fields);
}

/** "Back up now" not going, as words. */
export function backUpNowProblem(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.code === "backup_off") return "No destination is set, so there is nowhere to back up to.";
    if (err.code === "backup_failed") return `The backup did not go. The reason it was given: ${reasonText(err.message)}`;
    if (err.code === "timeout") return "mez did not answer in time. The backup may still be going: look again in a minute.";
  }
  return problemWith(err, []).message;
}

/**
 * A reason as the destination or the system gave it. It is quoted, not
 * rewritten: "mkdir E:\\backups: ..." with a capital M is nobody's sentence,
 * and the words are what someone helping will search for. Only the server's
 * own prefix is taken off, and a full stop put on.
 */
function reasonText(reason: string): string {
  const text = reason.trim().replace(/^backup: (backing up now: )?/, "");
  if (text === "") return "none.";
  return /[.!?)]$/.test(text) ? text : `${text}.`;
}

/** Where the saved backup goes, as one line, or null when it goes nowhere. */
export function destinationText(b: BackupResponse): string | null {
  if (b.kind === "folder") return b.folder;
  if (b.kind === "s3") {
    const where = `s3://${b.s3.bucket}/${b.s3.path || "mez"}`;
    return b.s3.endpoint === "" ? where : `${where} at ${b.s3.endpoint}`;
  }
  return null;
}

export interface BackupStanding {
  /** Colour, with the word: never one without the other (tokens.md). */
  readonly tone: ChipTone;
  readonly word: string;
  readonly sentence: string;
  /** The server's own reason, while something is wrong. */
  readonly reason: string | null;
  /** When it last had everything, said under a reason. */
  readonly lastGood: string | null;
}

const words: Readonly<Record<BackupState, { tone: ChipTone; word: string }>> = {
  off: { tone: "neutral", word: "Off" },
  starting: { tone: "info", word: "Starting" },
  ok: { tone: "success", word: "Backing up" },
  failing: { tone: "danger", word: "Not backing up" },
};

/**
 * How the backup is doing, for the owner. `when` turns the time it last had
 * everything into the venue's words.
 */
export function backupStanding(status: BackupResponse["status"], when: (at: Date) => string): BackupStanding {
  const at = status.last_synced_at === undefined ? null : new Date(status.last_synced_at);
  const last = at !== null && !Number.isNaN(at.getTime()) ? when(at) : null;
  // A state from a newer server than this screen is said as it came, in the colour of something to look at.
  const { tone, word } = words[status.state] ?? { tone: "warning" as const, word: status.state };

  switch (status.state) {
    case "off":
      return {
        tone,
        word,
        sentence: "Nothing is being backed up. If this computer's disk fails, the venue's sales, menu and settings go with it.",
        reason: null,
        lastGood: null,
      };
    case "starting":
      return { tone, word, sentence: "Reaching the destination for the first time since mez started.", reason: null, lastGood: null };
    case "ok":
      return {
        tone,
        word,
        sentence: last === null ? "Everything is at the destination." : `Everything is at the destination, as of ${last}.`,
        reason: null,
        lastGood: null,
      };
    case "failing":
      return {
        tone,
        word,
        sentence: "The backup is not reaching its destination. Sales carry on, and are copied when it does.",
        reason: status.error ? reasonText(status.error) : null,
        lastGood: last === null ? "It has not had everything since mez started." : `It last had everything at ${last}.`,
      };
    default:
      // A state this screen was not built to know: said as it came, with
      // whatever the server says of it, and nothing claimed either way.
      return {
        tone,
        word,
        sentence: "This screen does not know this state. Reload it; if it still says so, mez is newer than this screen.",
        reason: status.error ? reasonText(status.error) : null,
        lastGood: last === null ? null : `It last had everything at ${last}.`,
      };
  }
}
