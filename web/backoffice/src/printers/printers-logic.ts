// What the printers screen says about a printer (backoffice-printers.md).
// The settings are the server's; this words them, and turns the form into
// the request the server checks.

import { ApiError } from "../../../shared/api";
import type { PrinterResponse, PrinterSettings, PrintersResponse, PrinterTransport, StationPrinterResponse } from "../../../shared/api-types";
import { venueDate } from "../../../shared/venue-clock";

/** How the server reaches it: the address with its port, or the Windows printer's name. */
export function connectionText(p: Pick<PrinterResponse, "transport" | "address">): string {
  if (p.transport === "windows") return `USB: ${p.address}`;
  const hasPort = /^\[.*\]:\d+$/.test(p.address) || (/^[^:]+:\d+$/.test(p.address) && !p.address.startsWith("["));
  if (hasPort) return p.address;
  return p.address.includes(":") ? `[${p.address}]:9100` : `${p.address}:9100`;
}

/** The line under the title: "2 printers", or "3 printers, 1 retired". */
export function printersSummary(printers: readonly Pick<PrinterResponse, "active">[]): string {
  if (printers.length === 0) return "None yet";
  const retired = printers.filter((p) => !p.active).length;
  const count = printers.length === 1 ? "1 printer" : `${printers.length} printers`;
  return retired === 0 ? count : `${count}, ${retired} retired`;
}

/** What a printer prints, as its row says it: receipts, and each station's tickets. */
export function printsText(p: Pick<PrinterResponse, "is_receipt" | "stations">, stations: readonly StationPrinterResponse[]): string {
  const names = new Map(stations.map((s) => [s.code, s.name]));
  const parts = [...(p.is_receipt ? ["Receipts"] : []), ...p.stations.map((code) => `${names.get(code) ?? code} tickets`)];
  return parts.length === 0 ? "Nothing yet" : parts.join(", ");
}

/** "just now", "12 min ago", "3 h ago", or the venue's date, "2 Oct 2026". */
export function agoText(iso: string, now: Date, timezone: string | null): string {
  const minutes = Math.floor((now.getTime() - Date.parse(iso)) / 60000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes} min ago`;
  if (minutes < 24 * 60) return `${Math.floor(minutes / 60)} h ago`;
  return venueDate(iso, timezone);
}

/** What a new printer is for: receipts, one station's tickets, or nothing yet (BO-15). */
export type PrintsFor = "receipts" | "later" | `station:${string}`;

/** The choices for a new printer: receipts, each station's tickets, and deciding later. */
export function printsOptions(stations: readonly Pick<StationPrinterResponse, "code" | "name">[]): { value: PrintsFor; label: string }[] {
  return [
    { value: "receipts", label: "Receipts" },
    ...stations.map((s) => ({ value: `station:${s.code}` as const, label: `${s.name} tickets` })),
    { value: "later", label: "Decide later" },
  ];
}

/**
 * What a new printer starts on: receipts while nothing prints them, then the
 * first station with no printer, else nothing yet. A retired printer prints
 * nothing.
 */
export function defaultPrints(data: Pick<PrintersResponse, "printers" | "stations">): PrintsFor {
  const active = new Set(data.printers.filter((p) => p.active).map((p) => p.id));
  if (!data.printers.some((p) => p.active && p.is_receipt)) return "receipts";
  const bare = data.stations.find((s) => s.printer_id === undefined || !active.has(s.printer_id));
  return bare ? `station:${bare.code}` : "later";
}

/** The printer an add made: the one in the answer that was not there before, or null. */
export function addedPrinterId(before: PrintersResponse | undefined, after: PrintersResponse): number | null {
  const had = new Set((before?.printers ?? []).map((p) => p.id));
  const fresh = after.printers.filter((p) => !had.has(p.id)).map((p) => p.id);
  return fresh.length === 0 ? null : Math.max(...fresh);
}

export interface PrinterForm {
  name: string;
  transport: PrinterTransport;
  address: string;
  paperMM: 80 | 58;
}

export function emptyForm(): PrinterForm {
  return { name: "", transport: "tcp", address: "", paperMM: 80 };
}

export function formFrom(p: PrinterResponse): PrinterForm {
  return { name: p.name, transport: p.transport, address: p.address, paperMM: p.paper_mm === 58 ? 58 : 80 };
}

export function toSettings(f: PrinterForm): PrinterSettings {
  return { name: f.name.trim(), transport: f.transport, address: f.address.trim(), paper_mm: f.paperMM };
}

/** A refusal as the form shows it: under its field, or over the form. */
export interface Problem {
  field: "name" | "transport" | "address" | "paper_mm" | null;
  message: string;
}

const fields = ["name", "transport", "address", "paper_mm"] as const;

export function problemOf(err: unknown): Problem {
  if (err instanceof ApiError) {
    if (err.unreachable) return { field: null, message: "Can't reach the mez server. Check this computer's connection." };
    const field = (fields as readonly string[]).includes(err.field ?? "") ? (err.field as Problem["field"]) : null;
    if (err.code === "forbidden") return { field: null, message: "Only someone who can change the venue's settings can do this." };
    return { field, message: err.message };
  }
  return { field: null, message: "Something went wrong. Try again." };
}
