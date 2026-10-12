// The Devices screen's logic (docs/design/screens/backoffice-devices.md): how a
// device is described, who is in service and who was, and how a refusal reads.
// No React in this file, so all of it is tested directly.

import type { DeviceKind, DeviceRecordResponse } from "../../../shared/api-types";
import type { DeviceState } from "../../../shared/ui/status";
import { agoText } from "../printers/printers-logic";
import { problemWith, type Problem } from "../settings/problem";

/** A device's kind in the words staff use, never the code. */
export const kindWords: Record<DeviceKind, string> = {
  terminal: "Terminal",
  kitchen: "Kitchen display",
  back_office: "Back office",
  guest: "Guest",
};

/** The kinds a device can be paired as: a guest is a phone, not a paired device. */
export const PAIRABLE: readonly { value: Exclude<DeviceKind, "guest">; label: string }[] = [
  { value: "terminal", label: kindWords.terminal },
  { value: "kitchen", label: kindWords.kitchen },
  { value: "back_office", label: kindWords.back_office },
];

/** The server's limit on a device's name (internal/auth/devices.go). */
export const MAX_DEVICE_NAME = 64;

const DAY_MS = 24 * 60 * 60 * 1000;

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"] as const;

/** A date as people write it, "3 Oct 2026", on the browser's clock. Written out, not asked of the locale: its abbreviations differ between browsers. */
export function dateText(iso: string): string {
  const d = new Date(iso);
  return `${d.getDate()} ${MONTHS[d.getMonth()]} ${d.getFullYear()}`;
}

/** When a device was last heard from, in words: "Seen 12 min ago", or that it never was. */
export function lastSeenText(d: Pick<DeviceRecordResponse, "last_seen_at">, now: Date): string {
  if (!d.last_seen_at) return "Not heard from since it was paired";
  // From a day on, the date: agoText would ask the locale for it.
  if (now.getTime() - Date.parse(d.last_seen_at) >= DAY_MS) return `Seen ${dateText(d.last_seen_at)}`;
  return `Seen ${agoText(d.last_seen_at, now)}`;
}

/** A device heard from, but not for a day: a till in a drawer, or one that is off. */
export function isStale(d: Pick<DeviceRecordResponse, "last_seen_at">, now: Date): boolean {
  return d.last_seen_at !== undefined && now.getTime() - Date.parse(d.last_seen_at) >= DAY_MS;
}

export const revokedText = (d: Pick<DeviceRecordResponse, "revoked_at">): string => (d.revoked_at ? `Revoked ${dateText(d.revoked_at)}` : "Revoked");

/** The devices in service, then the revoked ones, each in the order the server sent. */
export function split(devices: readonly DeviceRecordResponse[]): { inService: DeviceRecordResponse[]; revoked: DeviceRecordResponse[] } {
  return {
    inService: devices.filter((d) => !d.revoked_at),
    revoked: devices.filter((d) => d.revoked_at),
  };
}

/** The list as the table shows it: the devices in service, then the revoked. */
export function listed(devices: readonly DeviceRecordResponse[]): DeviceRecordResponse[] {
  const { inService, revoked } = split(devices);
  return [...inService, ...revoked];
}

/** Where a device stands, for its chip: revoked, quiet for a day, or in service. */
export function stateOf(d: Pick<DeviceRecordResponse, "last_seen_at" | "revoked_at">, now: Date): DeviceState {
  if (d.revoked_at) return "revoked";
  return isStale(d, now) ? "quiet" : "in_service";
}

/** The line under the title: "4 in service", with how many are quiet and how many revoked when any are. */
export function devicesSummary(devices: readonly Pick<DeviceRecordResponse, "last_seen_at" | "revoked_at">[], now: Date): string {
  const count = (state: DeviceState) => devices.filter((d) => stateOf(d, now) === state).length;
  const quiet = count("quiet");
  const revoked = count("revoked");
  return [
    `${count("in_service") + quiet} in service`,
    ...(quiet > 0 ? [`${quiet} quiet for a day or more`] : []),
    ...(revoked > 0 ? [`${revoked} revoked`] : []),
  ].join(", ");
}

/** "runs out in 7 min", "runs out in under a minute", or that it has run out. */
export function runsOutText(expiresAt: string, now: Date): string {
  const ms = Date.parse(expiresAt) - now.getTime();
  if (ms <= 0) return "has run out";
  const minutes = Math.ceil(ms / 60000);
  return ms < 60000 ? "runs out in under a minute" : `runs out in ${minutes} min`;
}

/** A time of day, "17:42", for when a code stops working. */
export function timeText(iso: string): string {
  return new Date(iso).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", hour12: false });
}

export type PairField = "name" | "kind";
const pairFields: readonly PairField[] = ["name", "kind"];

export const pairProblemOf = (err: unknown): Problem<PairField> => problemWith(err, pairFields);

/** What can be refused in a name before asking: none given, or too long. */
export function nameProblem(name: string): Problem<"name"> | null {
  const n = name.trim();
  if (n === "" || [...n].length > MAX_DEVICE_NAME) {
    return { field: "name", message: `A name of 1 to ${MAX_DEVICE_NAME} characters is needed.` };
  }
  return null;
}

/** A failed request as words, for the line under a card's heading. */
export const actionProblemOf = (err: unknown): string => problemWith(err, []).message;

/**
 * What a device is, under its name. The one mez Cloud signs the owner in on
 * (ADR-323) is a back office nobody holds, and saying "Back office" of it
 * would send an owner looking for a laptop.
 */
export function kindText(d: Pick<DeviceRecordResponse, "kind" | "cloud">): string {
  return d.cloud ? "Your mez Cloud account's way in. Nobody holds it" : kindWords[d.kind];
}

/** What a row's menu can offer, in the order shown. */
export type DeviceAction = "rename" | "revoke";

/**
 * A row's menu: none for a revoked device. No Revoke on the device in use;
 * no Rename from away (ADR-308), and none for mez Cloud's, which is known
 * by its name on this screen and in the guide. None at all on a mirror, which
 * changes nothing (ADR-330).
 */
export function deviceActions(d: Pick<DeviceRecordResponse, "revoked_at" | "this" | "cloud">, away: boolean, mirror = false): DeviceAction[] {
  if (d.revoked_at || mirror) return [];
  return [...(away || d.cloud ? [] : (["rename"] as const)), ...(d.this ? [] : (["revoke"] as const))];
}

/** What revoking a device does, said before it is done, and the word on the button. */
export function revokeWords(d: Pick<DeviceRecordResponse, "cloud">): { body: string; confirm: string } {
  return d.cloud
    ? {
        body: "The back office can no longer be opened from your mez Cloud account, and anyone signed in that way is signed out at once. Guests' orders and the backup carry on. Pairing to mez Cloud again, here at the venue, turns it back on.",
        confirm: "Shut mez Cloud's sign-in",
      }
    : {
        body: "It stops working at once and shows the pairing screen. Orders it took stay as they are. To use it again it needs a new code.",
        confirm: "Revoke device",
      };
}

/** What is said once a device has been revoked. */
export function revokedToast(d: Pick<DeviceRecordResponse, "name" | "cloud">): string {
  return d.cloud ? "mez Cloud's sign-in is shut." : `${d.name} revoked.`;
}
