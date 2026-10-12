// Pairing a venue to a mez Cloud account, as both places that offer it need
// it: the back office's Devices screen and the setup wizard (ADR-318,
// ADR-320, ADR-322). Pure but for pairToCloud, so each sentence is tested
// without a cloud to pair with. The server cleans and checks the code too;
// the check here only spares the owner a code spent on a slip.

import { ApiError, type ApiClient } from "./api";
import type { CloudPairRequest, CloudPairResponse } from "./api-types";

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

/** What is wrong with a code before asking the server: none typed, or one that could not be a code. Null when it can be sent. */
export function codeRefusal(typed: string): string | null {
  if (typed.trim() === "") return "Type the code from your mez Cloud account.";
  if (cleanCode(typed) === null) {
    return `A pairing code is ${CODE_LENGTH} letters and digits, with no 0, O, 1 or I. Check it against the one in your account.`;
  }
  return null;
}

/** "Priya's", "Chris'": a name that ends in s takes the apostrophe alone. */
export function possessive(name: string): string {
  return /s$/i.test(name) ? `${name}'` : `${name}'s`;
}

/** Where the back office opens from away, from the hostname a pairing gave. */
export function backOfficeAddress(hostname: string): string {
  return `https://${hostname}/backoffice/`;
}

/**
 * What a pairing did, a line each, for what follows it. The first says who
 * the venue is paired to; the rest say what that changed here, and what, if
 * anything, is left for the owner to do.
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

/** Exchanges a code for a pairing. The code is sent as it is given: clean it first. */
export function pairToCloud(api: ApiClient, code: string): Promise<CloudPairResponse> {
  return api.post<CloudPairResponse>("/api/v1/cloud/pair", { code } satisfies CloudPairRequest);
}

/** Why a venue was not paired, in words for the owner. The server's own say whether the code is still good. */
export function pairRefusal(err: unknown): string {
  if (!(err instanceof ApiError)) return "Something went wrong. Try again; the code may still be good.";
  if (err.unreachable) return "Can't reach the mez server. Check this computer's connection and try again; the code is still good.";
  if (err.code === "forbidden") return "Only someone who can change the venue's settings can pair it.";
  return err.message;
}
