// The words for pairing a venue to a mez Cloud account (docs/design/screens/
// backoffice-devices.md, "Connect to mez Cloud"). Pure, so each is tested
// without a cloud to pair with. The server cleans and checks the code too
// (ADR-318); the check here only spares the owner a code spent on a typo.

import type { CloudPairResponse, TunnelPairing } from "../../../shared/api-types";
import type { Problem } from "../settings/problem";
import { problemWith } from "../settings/problem";
import { dateText } from "./devices-logic";

/** How long a pairing code is, and what it is made of: no 0, O, 1 or I. */
export const CODE_LENGTH = 8;
const CODE_ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";

/** A code as it is sent: capitals, without the spaces and hyphens a person adds; null if it could not be one. */
export function cleanCode(typed: string): string | null {
  const code = typed.toUpperCase().replace(/[\s-]/g, "");
  if (code.length !== CODE_LENGTH) return null;
  for (const ch of code) {
    if (!CODE_ALPHABET.includes(ch)) return null;
  }
  return code;
}

export type CloudField = "code";
const cloudFields: readonly CloudField[] = ["code"];

/** What can be refused in a code before asking: none typed, or one that could not be a code. */
export function codeProblem(typed: string): Problem<CloudField> | null {
  if (typed.trim() === "") return { field: "code", message: "Type the code from your mez Cloud account." };
  if (cleanCode(typed) === null) {
    return { field: "code", message: `A pairing code is ${CODE_LENGTH} letters and digits, with no 0, O, 1 or I. Check it against the one in your account.` };
  }
  return null;
}

/** A refused pairing as words, under the code when it is about the code. */
export const cloudProblemOf = (err: unknown): Problem<CloudField> => problemWith(err, cloudFields);

/** "Paired to Priya Menon's mez Cloud account since 12 Oct 2026." */
export function pairedLine(p: Pick<TunnelPairing, "owner" | "paired_at">): string {
  return `Paired to ${possessive(p.owner)} mez Cloud account since ${dateText(p.paired_at)}.`;
}

/** "Priya's", "Chris'": a name that ends in s takes the apostrophe alone. */
function possessive(name: string): string {
  return /s$/i.test(name) ? `${name}'` : `${name}'s`;
}

/** Where the back office opens from away, from the hostname a pairing gave. */
export function backOfficeAddress(hostname: string): string {
  return `https://${hostname}/backoffice/`;
}

/**
 * What a pairing did, a line each, for the modal that follows it. The first
 * says who the venue is paired to; the rest say what that changed here, and
 * what, if anything, is left for the owner to do.
 */
export function pairedLines(res: CloudPairResponse): string[] {
  const lines = [`This venue is now paired to ${possessive(res.owner)} mez Cloud account.`];
  switch (res.backup) {
    case "set":
      lines.push("A backup of the venue is now kept with mez Cloud, away from this computer.");
      break;
    case "kept":
      lines.push("Your backup is left where it was. To keep it with mez Cloud, turn the backup off under Settings and pair again.");
      break;
    case "failed":
      lines.push("The backup with mez Cloud could not be started. The venue is paired all the same; look under Settings, Backup.");
      break;
    case "none":
      break;
  }
  if (res.restart_needed) {
    lines.push("Restart mez to finish: part of this takes hold when it next starts.");
  }
  return lines;
}
