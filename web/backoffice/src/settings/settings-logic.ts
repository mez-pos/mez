// The venue settings screen's logic (docs/design/screens/backoffice-settings.md):
// which sections there are, the venue details form and its checks, and how a
// refusal reads. No React here, so all of it is tested directly.

import type { SettingsVenueResponse, VenueDetailsRequest } from "../../../shared/api-types";
import { problemWith, type Problem as FieldProblem } from "./problem";

export type SettingsSectionId = "venue" | "tax" | "shifts" | "stations" | "backup";

export interface SettingsSection {
  readonly id: SettingsSectionId;
  readonly label: string;
  readonly path: string;
  /** False until its task lands: shown as Soon, not a link. */
  readonly built: boolean;
}

/** The sections, in the order the rail shows them. */
export const SETTINGS_SECTIONS: readonly SettingsSection[] = [
  { id: "venue", label: "Venue", path: "/settings/venue", built: true },
  { id: "tax", label: "Tax and numbering", path: "/settings/tax", built: true },
  { id: "shifts", label: "Shift, tips and receipt", path: "/settings/shifts", built: true },
  { id: "stations", label: "Stations and courses", path: "/settings/stations", built: false },
  { id: "backup", label: "Backup", path: "/settings/backup", built: false },
];

/** Where `/settings` and a path that is no section go. */
export const DEFAULT_SECTION = SETTINGS_SECTIONS[0]!;

/** The built section a path names, or null: an unbuilt one is not somewhere to be. */
export function builtSectionFor(path: string): SettingsSection | null {
  return SETTINGS_SECTIONS.find((s) => s.built && s.path === path) ?? null;
}

// The server's limits (internal/service/settings): a name is a label on a
// receipt, not prose. Checked here as well so the refusal is instant; the
// server checks again and its words win.
export const MAX_NAME = 60;
export const MAX_LEGAL_NAME = 120;

/** The venue details as typed. */
export interface VenueForm {
  name: string;
  legalName: string;
  timezone: string;
}

export function venueFormFrom(v: SettingsVenueResponse): VenueForm {
  return { name: v.name, legalName: v.legal_name, timezone: v.timezone };
}

/** What Save sends: trimmed, as the server trims it. */
export function toVenueInput(form: VenueForm): VenueDetailsRequest {
  return { name: form.name.trim(), legal_name: form.legalName.trim(), timezone: form.timezone.trim() };
}

/** Whether the draft differs from what the server has, ignoring stray spaces. */
export function venueChanged(draft: VenueForm, saved: VenueForm): boolean {
  const a = toVenueInput(draft);
  const b = toVenueInput(saved);
  return a.name !== b.name || a.legal_name !== b.legal_name || a.timezone !== b.timezone;
}

export type VenueField = "name" | "legal_name" | "timezone";

export type Problem = FieldProblem<VenueField>;

/** What can be refused without asking the server. */
export function venueProblem(form: VenueForm): Problem | null {
  const input = toVenueInput(form);
  const count = (s: string) => [...s].length;
  if (input.name === "") return { field: "name", message: "Give the venue a name." };
  if (count(input.name) > MAX_NAME) return { field: "name", message: `Keep the name to ${MAX_NAME} characters.` };
  if (count(input.legal_name) > MAX_LEGAL_NAME) {
    return { field: "legal_name", message: `Keep the legal name to ${MAX_LEGAL_NAME} characters.` };
  }
  if (input.timezone === "") return { field: "timezone", message: "Choose a timezone, like Asia/Kolkata." };
  return null;
}

const venueFields: readonly VenueField[] = ["name", "legal_name", "timezone"];

/** A failed request as words, and the field to put them under. */
export const problemOf = (err: unknown): Problem => problemWith(err, venueFields);

/** Zone names for the timezone field's suggestions: the browser's own list when it has one. */
export function timezoneNames(intl: { supportedValuesOf?: (key: "timeZone") => string[] } = Intl as never): string[] {
  try {
    const names = intl.supportedValuesOf?.("timeZone") ?? [];
    if (names.length > 0) return names;
  } catch {
    // An older browser: fall through to the short list.
  }
  return FALLBACK_ZONES;
}

const FALLBACK_ZONES = [
  "Asia/Kolkata",
  "Asia/Dubai",
  "Asia/Singapore",
  "Asia/Colombo",
  "Asia/Kathmandu",
  "Europe/London",
  "Europe/Paris",
  "America/New_York",
  "America/Chicago",
  "America/Los_Angeles",
  "Australia/Sydney",
  "UTC",
];
