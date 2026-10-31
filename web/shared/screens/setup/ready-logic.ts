// The wizard's last step, apart from how it is drawn (setup-wizard.md,
// ADR-269, ADR-271): what a second device can be, where it opens mez, and the
// code that pairs it.

import { ApiError, type ApiClient } from "../../api";
import type { AddressAppResponse, AddressResponse, AddressesResponse, DeviceKind, NewCodeRequest, PairingCodeResponse } from "../../api-types";
import { unreachableMessage } from "../../auth/messages";

/** A kind of device that can be paired, in the owner's words. */
export interface SecondDevice {
  readonly kind: Exclude<DeviceKind, "guest">;
  readonly label: string;
  readonly blurb: string;
  /** What it is called until someone renames it. */
  readonly name: string;
  /**
   * Where its app is under the server's address. The server says the whole
   * address (`apps` of an address); this is only for telling an owner what to
   * type after an address the server could not work out.
   */
  readonly path: string;
}

export const secondDevices: readonly SecondDevice[] = [
  { kind: "terminal", label: "Another till", blurb: "A second counter, or a waiter's tablet.", name: "Counter 2", path: "" },
  { kind: "kitchen", label: "Kitchen display", blurb: "A screen at the pass that shows the orders.", name: "Kitchen", path: "/kitchen" },
  { kind: "back_office", label: "Back office", blurb: "A laptop for the menu, reports and settings.", name: "Office", path: "/backoffice/" },
];

/**
 * Where a new venue's owner goes from the wizard: the back office's menu, in
 * the same tab, which keeps the session. A till with no menu sells nothing
 * (SET-6).
 */
export const menuAddress = "/backoffice/menu";

export function secondDevice(kind: string): SecondDevice {
  return secondDevices.find((d) => d.kind === kind) ?? (secondDevices[0] as SecondDevice);
}

/** The address without its scheme, as a person types it into a browser's bar. */
export function typed(url: string): string {
  return url.replace(/^https?:\/\//, "");
}

/**
 * Where the server is on the venue's network: by the name it answers for, and
 * by each address. Never throws: with no answer there are none of either, and
 * the screen says so.
 */
export async function serverAddresses(api: ApiClient): Promise<AddressesResponse> {
  try {
    const found = await api.get<AddressesResponse>("/api/v1/devices/addresses");
    return { named: found.named ?? [], addresses: found.addresses ?? [] };
  } catch {
    return { named: [], addresses: [] };
  }
}

/** One place to open mez: its address for the kind of device, the code to scan, and the network it is on. */
export interface Place {
  readonly url: string;
  /** The address as a QR code, an SVG, or empty when the server drew none. */
  readonly svg: string;
  /** The adapter's name ("Wi-Fi"), or empty. */
  readonly adapter: string;
}

/**
 * The places to show for a kind of device. `main` are the ones to show large:
 * the real adapters', each as likely as the next, because which network a
 * tablet is on is not something the server can know. `others` are virtual
 * adapters' (a virtual machine's, a VPN's), which a tablet is unlikely to be
 * on, shown small. With no real adapter the virtual ones are all there is.
 */
export function placesFor(addresses: readonly AddressResponse[], kind: string): { main: Place[]; others: Place[] } {
  const place = (a: AddressResponse): Place | null => {
    const app: AddressAppResponse | undefined = a.apps.find((p) => p.kind === kind);
    return app ? { url: app.url, svg: app.svg, adapter: a.adapter } : null;
  };
  const of = (list: readonly AddressResponse[]) => list.map(place).filter((p): p is Place => p !== null);
  const real = of(addresses.filter((a) => !a.virtual));
  const virtual = of(addresses.filter((a) => a.virtual));
  return real.length > 0 ? { main: real, others: virtual } : { main: virtual, others: [] };
}

/**
 * Where a kind of device opens mez by the server's name, "mez.local:7777"
 * (ADR-379), or null where it announces none. Offered first: a tablet that
 * opens it keeps working when this computer's address changes (SET-1).
 */
export function namedPlaceFor(named: readonly AddressAppResponse[], kind: string): Place | null {
  const app = named.find((a) => a.kind === kind);
  return app ? { url: app.url, svg: app.svg, adapter: "" } : null;
}

/** An SVG as something an <img> can show. */
export function svgSrc(svg: string): string {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
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
