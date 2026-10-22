// The words for pairing a venue to a mez Cloud account on the Devices screen
// (docs/design/screens/backoffice-devices.md, "Connect to mez Cloud"). What
// the setup wizard says too is in shared/cloud-pair.ts; this is what is the
// back office's own: a refusal placed under its field, and the line that
// says who the venue is paired to.

import type { TunnelCloud, TunnelPairing } from "../../../shared/api-types";
import { codeRefusal, possessive } from "../../../shared/cloud-pair";
import type { Problem } from "../settings/problem";
import { problemWith } from "../settings/problem";
import { dateText, timeText } from "./devices-logic";

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

/**
 * When the owner's last close-of-day message went (ADR-364): "Last report:
 * today 23:04", "yesterday 23:04", or "12 Oct 2026, 23:04". With none yet,
 * "No close-of-day report has gone yet."; null when the cloud said nothing
 * of reports, as a relay of the venue's own does not.
 */
export function lastReportLine(cloud: Pick<TunnelCloud, "last_report" | "messages_url"> | undefined, now: Date): string | null {
  if (!cloud || (!cloud.last_report && !cloud.messages_url)) return null;
  if (!cloud.last_report) return "No close-of-day report has gone yet.";
  const at = new Date(cloud.last_report);
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  const when =
    at.toDateString() === now.toDateString()
      ? `today ${timeText(cloud.last_report)}`
      : at.toDateString() === yesterday.toDateString()
        ? `yesterday ${timeText(cloud.last_report)}`
        : `${dateText(cloud.last_report)}, ${timeText(cloud.last_report)}`;
  return `Last report: ${when}.`;
}
