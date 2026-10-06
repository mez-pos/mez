// The wizard's last step, apart from how it is drawn (setup-wizard.md,
// ADR-269): what a second device can be, where it opens mez, and the code
// that pairs it.

import { ApiError, type ApiClient } from "../../api";
import type { AddressesResponse, DeviceKind, NewCodeRequest, PairingCodeResponse } from "../../api-types";
import { unreachableMessage } from "../../auth/messages";

/** A kind of device that can be paired, in the owner's words. */
export interface SecondDevice {
  readonly kind: Exclude<DeviceKind, "guest">;
  readonly label: string;
  readonly blurb: string;
  /** What it is called until someone renames it. */
  readonly name: string;
  /** Where its app is, under the server's address. */
  readonly path: string;
}

export const secondDevices: readonly SecondDevice[] = [
  { kind: "terminal", label: "Another till", blurb: "A second counter, or a waiter's tablet.", name: "Counter 2", path: "" },
  { kind: "kitchen", label: "Kitchen display", blurb: "A screen at the pass that shows the orders.", name: "Kitchen", path: "/kitchen" },
  { kind: "back_office", label: "Back office", blurb: "A laptop for the menu, reports and settings.", name: "Office", path: "/backoffice/" },
];

export function secondDevice(kind: string): SecondDevice {
  return secondDevices.find((d) => d.kind === kind) ?? (secondDevices[0] as SecondDevice);
}

/** The address a device of this kind opens: the server's, and its app's path. */
export function openAt(url: string, device: SecondDevice): string {
  return url.replace(/\/+$/, "") + device.path;
}

/** The address without its scheme, as a person types it into a browser's bar. */
export function typed(url: string): string {
  return url.replace(/^https?:\/\//, "");
}

/** Where the server is on the venue's network. Never throws: with no answer there are none, and the screen says so. */
export async function serverAddresses(api: ApiClient): Promise<readonly string[]> {
  try {
    return (await api.get<AddressesResponse>("/api/v1/devices/addresses")).urls;
  } catch {
    return [];
  }
}

/** A pairing code for one device of this kind. */
export async function makeCode(api: ApiClient, device: SecondDevice): Promise<PairingCodeResponse> {
  const body: NewCodeRequest = { name: device.name, kind: device.kind };
  return api.post<PairingCodeResponse>("/api/v1/devices/codes", body);
}

/** When a code stops working, as a time of day on this computer's clock. */
export function untilWords(expiresAt: string, locale?: string): string {
  const at = new Date(expiresAt);
  if (Number.isNaN(at.getTime())) return "for 10 minutes";
  return `until ${at.toLocaleTimeString(locale, { hour: "numeric", minute: "2-digit" })}`;
}

/** Why a code could not be made, in words for the owner. */
export function codeError(err: unknown, host: string): string {
  if (!(err instanceof ApiError)) return "Something went wrong. Try again.";
  if (err.unreachable) return unreachableMessage(host);
  if (err.code === "forbidden") return "You are not allowed to pair devices. A manager can make a code in the back office, under Devices.";
  return `The server refused: ${err.message}`;
}
