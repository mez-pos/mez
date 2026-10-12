// The words for pairing a venue to a mez Cloud account on the Devices screen
// (docs/design/screens/backoffice-devices.md, "Connect to mez Cloud"). What
// the setup wizard says too is in shared/cloud-pair.ts; this is what is the
// back office's own: a refusal placed under its field, and the line that
// says who the venue is paired to.

import type { TunnelPairing } from "../../../shared/api-types";
import { codeRefusal, possessive } from "../../../shared/cloud-pair";
import type { Problem } from "../settings/problem";
import { problemWith } from "../settings/problem";
import { dateText } from "./devices-logic";

export { CODE_LENGTH, backOfficeAddress, cleanCode, pairedLines } from "../../../shared/cloud-pair";

export type CloudField = "code";
const cloudFields: readonly CloudField[] = ["code"];

/** What can be refused in a code before asking: none typed, or one that could not be a code. */
export function codeProblem(typed: string): Problem<CloudField> | null {
  const message = codeRefusal(typed);
  return message === null ? null : { field: "code", message };
}

/** A refused pairing as words, under the code when it is about the code. */
export const cloudProblemOf = (err: unknown): Problem<CloudField> => problemWith(err, cloudFields);

/** "Paired to Priya Menon's mez Cloud account since 12 Oct 2026." */
export function pairedLine(p: Pick<TunnelPairing, "owner" | "paired_at">): string {
  return `Paired to ${possessive(p.owner)} mez Cloud account since ${dateText(p.paired_at)}.`;
}
