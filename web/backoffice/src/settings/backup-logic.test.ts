import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { BackupResponse } from "../../../shared/api-types";
import {
  BACKUP_KINDS,
  backUpNowProblem,
  backupAlarm,
  backupChanged,
  backupFormFrom,
  backupProblem,
  backupProblemOf,
  backupStanding,
  destinationText,
  secretKept,
  toBackupInput,
  type BackupForm,
} from "./backup-logic";

const noBucket = { bucket: "", path: "", region: "", endpoint: "", access_key_id: "", secret_set: false };

const off: BackupResponse = { kind: "", folder: "", s3: noBucket, status: { state: "off" }, restart_needed: false };
const toFolder: BackupResponse = { ...off, kind: "folder", folder: "D:\\mez-backup", status: { state: "ok", last_synced_at: "2026-10-07T08:32:00Z" } };
const toBucket: BackupResponse = {
  ...off,
  kind: "s3",
  s3: { bucket: "corner-cafe", path: "", region: "", endpoint: "https://abc.r2.cloudflarestorage.com", access_key_id: "AKID", secret_set: true },
  status: { state: "ok" },
};

const form = (from: BackupResponse, change: Partial<BackupForm> = {}): BackupForm => ({ ...backupFormFrom(from), ...change });
const when = (at: Date) => at.toISOString().slice(11, 16);

describe("the form and what is sent", () => {
  it("offers nowhere, a folder and a bucket, in that order", () => {
    expect(BACKUP_KINDS.map((k) => k.value)).toEqual(["", "folder", "s3"]);
  });

  it("reads the server's destination, and never has a secret to read", () => {
    expect(backupFormFrom(toBucket)).toEqual({
      kind: "s3",
      folder: "",
      bucket: "corner-cafe",
      path: "",
      endpoint: "https://abc.r2.cloudflarestorage.com",
      region: "",
      accessKeyId: "AKID",
      secret: "",
    });
  });

  it("sends a folder trimmed, and nothing of a bucket", () => {
    const sent = toBackupInput(form(toFolder, { folder: "  E:\\backups ", bucket: "left-over", secret: "typed-then-abandoned" }));
    expect(sent.kind).toBe("folder");
    expect(sent.folder).toBe("E:\\backups");
    expect(sent.s3).toEqual({ bucket: "", path: "", region: "", endpoint: "", access_key_id: "", secret_access_key: "" });
  });

  it("sends a bucket tidied, the secret as typed, and no folder", () => {
    const sent = toBackupInput(
      form(toBucket, {
        folder: "D:\\old",
        bucket: " corner-cafe ",
        path: " venues/cafe ",
        endpoint: " https://abc.example/ ",
        accessKeyId: " AKID ",
        secret: " s3cret ",
      }),
    );
    expect(sent).toEqual({
      kind: "s3",
      folder: "",
      s3: { bucket: "corner-cafe", path: "venues/cafe", region: "", endpoint: "https://abc.example", access_key_id: "AKID", secret_access_key: " s3cret " },
    });
  });

  it("sends nowhere as nothing at all", () => {
    expect(toBackupInput(form(toBucket, { kind: "", secret: "typed" }))).toEqual({
      kind: "",
      folder: "",
      s3: { bucket: "", path: "", region: "", endpoint: "", access_key_id: "", secret_access_key: "" },
    });
  });
});

describe("whether there is anything to save", () => {
  it("is unchanged as read, and by spaces or a slash on the end", () => {
    expect(backupChanged(form(toFolder), form(toFolder))).toBe(false);
    expect(backupChanged(form(toFolder, { folder: " D:\\mez-backup " }), form(toFolder))).toBe(false);
    expect(backupChanged(form(toBucket, { endpoint: "https://abc.r2.cloudflarestorage.com/" }), form(toBucket))).toBe(false);
  });

  it("is changed by another kind, another place, or a secret typed", () => {
    expect(backupChanged(form(toFolder, { kind: "" }), form(toFolder))).toBe(true);
    expect(backupChanged(form(toFolder, { folder: "E:\\elsewhere" }), form(toFolder))).toBe(true);
    expect(backupChanged(form(toBucket, { path: "venues/cafe" }), form(toBucket))).toBe(true);
    expect(backupChanged(form(toBucket, { secret: "new" }), form(toBucket))).toBe(true);
  });

  it("ignores the boxes of a kind that is not chosen", () => {
    expect(backupChanged(form(toFolder, { bucket: "typed-and-left", secret: "typed" }), form(toFolder))).toBe(false);
    expect(backupChanged(form(off, { folder: "D:\\typed-then-back-to-nowhere" }), form(off))).toBe(false);
  });
});

describe("the saved secret", () => {
  it("is kept while the bucket, where it is and the key id are as they were", () => {
    expect(secretKept(form(toBucket), form(toBucket), true)).toBe(true);
    expect(secretKept(form(toBucket, { path: "venues/cafe" }), form(toBucket), true)).toBe(true);
  });

  it("is not lent to another bucket, address, region or key", () => {
    for (const change of [{ bucket: "another" }, { endpoint: "https://elsewhere.example" }, { region: "eu-west-1" }, { accessKeyId: "OTHER" }]) {
      expect(secretKept(form(toBucket, change), form(toBucket), true)).toBe(false);
    }
  });

  it("is not there to keep when none is saved, or the saved destination is a folder", () => {
    expect(secretKept(form(toBucket), form(toBucket), false)).toBe(false);
    expect(secretKept(form(toFolder, { kind: "s3" }), form(toFolder), true)).toBe(false);
  });
});

describe("what can be refused without asking", () => {
  const check = (f: BackupForm, saved = toBucket) => backupProblem(f, form(saved), saved.s3.secret_set);

  it("accepts nowhere, a full folder, and a bucket as saved", () => {
    expect(check(form(off), off)).toBeNull();
    for (const folder of ["D:\\mez-backup", "d:/mez-backup", "\\\\nas\\backups\\cafe", "/mnt/backup"]) {
      expect(check(form(toFolder, { folder }), toFolder)).toBeNull();
    }
    expect(check(form(toBucket))).toBeNull();
    expect(check(form(toBucket, { endpoint: "", region: "ap-south-1", secret: "s" }))).toBeNull();
    expect(check(form(toBucket, { endpoint: "http://192.168.1.20:9000", secret: "s" }))).toBeNull();
  });

  it("refuses a folder that is empty or not written in full", () => {
    expect(check(form(toFolder, { folder: "  " }), toFolder)?.field).toBe("folder");
    expect(check(form(toFolder, { folder: "mez-backup" }), toFolder)?.field).toBe("folder");
    expect(check(form(toFolder, { folder: "..\\backup" }), toFolder)?.field).toBe("folder");
  });

  it.each([
    ["a bucket in capitals", { bucket: "CornerCafe" }, "bucket"],
    ["a bucket of two letters", { bucket: "cc" }, "bucket"],
    ["a bucket with two dots together", { bucket: "corner..cafe" }, "bucket"],
    ["a bucket ending in a hyphen", { bucket: "corner-" }, "bucket"],
    ["a path with a slash on the front", { path: "/venues" }, "path"],
    ["a path with a slash on the end", { path: "venues/" }, "path"],
    ["an address with no scheme", { endpoint: "abc.example", secret: "s" }, "endpoint"],
    ["an address with the bucket on the end", { endpoint: "https://abc.example/corner-cafe", secret: "s" }, "endpoint"],
    ["an address with a key in it", { endpoint: "https://AKID:secret@abc.example", secret: "s" }, "endpoint"],
    ["Amazon with no region", { endpoint: "", region: "", secret: "s" }, "region"],
    ["no key id", { accessKeyId: " ", secret: "s" }, "access_key_id"],
  ] as const)("refuses %s", (_name, change, field) => {
    expect(check(form(toBucket, change))?.field).toBe(field);
  });

  it("asks for the secret when there is none to keep, and says why when one was saved", () => {
    const first = backupProblem(form(off, { kind: "s3", bucket: "corner-cafe", region: "ap-south-1", accessKeyId: "AKID" }), form(off), false);
    expect(first).toEqual({ field: "secret_access_key", message: "Enter the secret access key." });

    const moved = check(form(toBucket, { bucket: "another-bucket" }));
    expect(moved?.field).toBe("secret_access_key");
    expect(moved?.message).toContain("again");
  });

  it("refuses a secret pasted with a space on either end", () => {
    expect(check(form(toBucket, { secret: "s3cret " }))?.field).toBe("secret_access_key");
    expect(check(form(toBucket, { secret: " s3cret" }))?.field).toBe("secret_access_key");
    expect(check(form(toBucket, { secret: "   " }))?.field).toBe("secret_access_key");
  });

  it("refuses a drive with no folder, and a share with no name", () => {
    expect(check(form(toFolder, { folder: "C:" }), toFolder)?.field).toBe("folder");
    expect(check(form(toFolder, { folder: "C:mez" }), toFolder)?.field).toBe("folder");
    expect(check(form(toFolder, { folder: "\\\\nas" }), toFolder)?.field).toBe("folder");
  });

  it("does not repeat an address with a key in it back", () => {
    expect(check(form(toBucket, { endpoint: "https://AKID:hunter2@abc.example", secret: "s" }))?.message).not.toContain("hunter2");
  });
});

describe("a refusal from the server", () => {
  it("goes under the field the server names", () => {
    const refused = backupProblemOf(new ApiError("invalid_setting", "The folder is inside the data directory.", 422, "folder"));
    expect(refused).toEqual({ field: "folder", message: "The folder is inside the data directory." });
    expect(backupProblemOf(new ApiError("invalid_setting", "Not both set.", 422, "secret_access_key")).field).toBe("secret_access_key");
  });

  it("says a destination that could not be written to at the foot, with its own reason", () => {
    const refused = backupProblemOf(new ApiError("backup_unusable", "mkdir E:\\backups: The system cannot find the path specified", 422));
    expect(refused.field).toBeNull();
    expect(refused.message).toBe(
      "mez could not write there, so nothing was changed. The reason it was given: mkdir E:\\backups: The system cannot find the path specified.",
    );
  });

  it("does not say a save failed when it only ran out of time", () => {
    expect(backupProblemOf(new ApiError("timeout", "the server did not answer in time")).message).toContain("may have gone through");
  });

  it("says the server could not be reached, and anything else plainly", () => {
    expect(backupProblemOf(new ApiError("network", "failed")).message).toContain("Can't reach");
    expect(backupProblemOf(new ApiError("forbidden", "no", 403)).message).toContain("Only someone who can change");
    expect(backupProblemOf(new Error("boom")).message).toBe("Something went wrong. Try again.");
  });
});

describe("back up now not going", () => {
  it("gives the reason", () => {
    expect(backUpNowProblem(new ApiError("backup_failed", "the destination did not answer in time", 502))).toBe(
      "The backup did not go. The reason it was given: the destination did not answer in time.",
    );
    expect(backUpNowProblem(new ApiError("backup_off", "No backup destination is set.", 409))).toContain("nowhere to back up to");
    expect(backUpNowProblem(new ApiError("timeout", "x"))).toContain("may still be going");
    expect(backUpNowProblem(new ApiError("network", "x"))).toContain("Can't reach");
  });
});

describe("where it goes, in one line", () => {
  it("names the folder, or the bucket and the folder in it", () => {
    expect(destinationText(off)).toBeNull();
    expect(destinationText(toFolder)).toBe("D:\\mez-backup");
    expect(destinationText(toBucket)).toBe("s3://corner-cafe/mez at https://abc.r2.cloudflarestorage.com");
    expect(destinationText({ ...toBucket, s3: { ...toBucket.s3, path: "venues/cafe", endpoint: "", region: "ap-south-1" } })).toBe(
      "s3://corner-cafe/venues/cafe",
    );
  });
});

describe("how it is doing", () => {
  it("is a word and a colour for every state", () => {
    // Off is a risk, in the warning tone (BO-11, ADR-381 item 7).
    expect(backupStanding({ state: "off" }, when)).toMatchObject({ tone: "warning", word: "Off", reason: null });
    expect(backupStanding({ state: "starting" }, when)).toMatchObject({ tone: "info", word: "Starting" });
    expect(backupStanding({ state: "ok" }, when)).toMatchObject({ tone: "success", word: "Backing up" });
    expect(backupStanding({ state: "failing" }, when)).toMatchObject({ tone: "danger", word: "Not backing up" });
  });

  it("says off as what it risks", () => {
    expect(backupStanding({ state: "off" }, when).sentence).toContain("disk fails");
  });

  it("raises the dashboard's band for nothing backed up and for a backup failing, and for nothing else (BO-11)", () => {
    expect(backupAlarm({ state: "off" }, when)).toMatchObject({ tone: "warning", sentence: expect.stringContaining("disk fails") });
    expect(backupAlarm({ state: "failing" }, when)).toMatchObject({ tone: "danger" });
    expect(backupAlarm({ state: "starting" }, when)).toBeNull();
    expect(backupAlarm({ state: "ok" }, when)).toBeNull();
  });

  it("says when everything was last there", () => {
    expect(backupStanding({ state: "ok", last_synced_at: "2026-10-07T08:32:00Z" }, when).sentence).toBe("Everything is at the destination, as of 08:32.");
    expect(backupStanding({ state: "ok" }, when).sentence).toBe("Everything is at the destination.");
    expect(backupStanding({ state: "ok", last_synced_at: "not a time" }, when).sentence).toBe("Everything is at the destination.");
  });

  it("says failing with the server's reason and when it last worked", () => {
    const failing = backupStanding({ state: "failing", error: "the bucket did not answer in 8s", last_synced_at: "2026-10-07T08:32:00Z" }, when);
    expect(failing.sentence).toContain("Sales carry on");
    expect(failing.reason).toBe("the bucket did not answer in 8s.");
    expect(failing.lastGood).toBe("It last had everything at 08:32.");

    const never = backupStanding({ state: "failing" }, when);
    expect(never.reason).toBeNull();
    expect(never.lastGood).toBe("It has not had everything since mez started.");
  });

  it("quotes a reason as it came, less the server's own prefix", () => {
    const quoted = (error: string) => backupStanding({ state: "failing", error }, when).reason;
    expect(quoted("backup: backing up now: sync database: mkdir E:\\x: Access is denied.")).toBe("sync database: mkdir E:\\x: Access is denied.");
    expect(quoted("photos: copying photo ab.jpg: disk full")).toBe("photos: copying photo ab.jpg: disk full.");
    expect(quoted("did not answer (context deadline exceeded)")).toBe("did not answer (context deadline exceeded)");
  });

  it("says nothing of when for a time it cannot read", () => {
    expect(backupStanding({ state: "failing", last_synced_at: "not a time" }, when).lastGood).toBe("It has not had everything since mez started.");
  });

  it("says a state it does not know as it came, and claims nothing of it", () => {
    const odd = backupStanding({ state: "paused" as never }, when);
    expect(odd.word).toBe("paused");
    expect(odd.tone).toBe("warning");
    expect(odd.sentence).toContain("does not know this state");
    expect(odd.sentence).not.toContain("not reaching");
  });
});
