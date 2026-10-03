// The tax half of "Tax and numbering" (docs/design/screens/backoffice-settings.md):
// the venue's tax profile and its tax classes, as forms. The server checks
// everything again, and for a regime's own rules (does this GSTIN fit this
// state?) its words win; what is checked here is only what costs nothing.
// No React in this file, so all of it is tested directly.

import type { JurisdictionRate, TaxClassRequest, TaxClassResponse, TaxSettingsRequest, TaxSettingsResponse } from "../../../shared/api-types";
import { problemWith, type Problem } from "./problem";

export type ProfileId = "none" | "gst_in" | "vat_uk" | "sales_tax_us";

export const PROFILES: readonly { value: ProfileId; label: string }[] = [
  { value: "none", label: "No tax" },
  { value: "gst_in", label: "GST (India)" },
  { value: "vat_uk", label: "VAT (UK)" },
  { value: "sales_tax_us", label: "US sales tax" },
];

export type Pricing = "inclusive" | "exclusive";

export const PRICINGS: readonly { value: Pricing; label: string }[] = [
  { value: "inclusive", label: "Prices include tax" },
  { value: "exclusive", label: "Tax is added on top" },
];

/** What the registration number is called under a profile, or null when it has none. */
export function registrationLabel(profile: ProfileId): string | null {
  switch (profile) {
    case "gst_in":
      return "GSTIN";
    case "vat_uk":
      return "VAT number";
    default:
      return null;
  }
}

/** A tax rate is thousandths of a percent (5000 is 5%); the owner types percent. */
export const MAX_RATE = 100_000;

/** "7.25", "5%" or "12" as a rate, or null for anything that is not 0% to 100% with up to three decimals. */
export function parsePercent(text: string): number | null {
  const m = /^(\d{1,3})(?:\.(\d{1,3}))?\s*%?$/.exec(text.trim());
  if (!m) return null;
  const rate = Number(m[1]) * 1000 + Number((m[2] ?? "").padEnd(3, "0"));
  return rate <= MAX_RATE ? rate : null;
}

/** A rate as the owner types it: 2500 is "2.5", 5000 is "5". */
export function percentInput(rate: number): string {
  const rest = String(rate % 1000).padStart(3, "0").replace(/0+$/, "");
  return rest === "" ? String(Math.trunc(rate / 1000)) : `${Math.trunc(rate / 1000)}.${rest}`;
}

/** A rate for reading: "5%", "2.5%". */
export function percentText(rate: number): string {
  return `${percentInput(rate)}%`;
}

// ---- the tax profile -----------------------------------------------------

export interface JurisdictionRow {
  name: string;
  /** Percent, as typed. */
  rate: string;
}

export interface TaxForm {
  profile: ProfileId;
  registrationId: string;
  pricing: Pricing;
  stateCode: string;
  jurisdiction: JurisdictionRow[];
}

const profileIds: readonly string[] = PROFILES.map((p) => p.value);

export function taxFormFrom(t: TaxSettingsResponse): TaxForm {
  return {
    profile: profileIds.includes(t.profile) ? (t.profile as ProfileId) : "none",
    registrationId: t.registration_id,
    pricing: t.pricing === "exclusive" ? "exclusive" : "inclusive",
    stateCode: t.state_code,
    jurisdiction: t.jurisdiction.map((j: JurisdictionRate) => ({ name: j.name, rate: percentInput(j.rate) })),
  };
}

/** A registration number as the server keeps it: capitals, no spaces. */
const normalRegistration = (s: string): string => s.replace(/\s+/g, "").toUpperCase();

/** What a profile actually uses of the form, so a field it ignores never counts as a change. */
function effective(f: TaxForm): unknown {
  switch (f.profile) {
    case "none":
      return { profile: f.profile };
    case "gst_in":
      return { profile: f.profile, r: normalRegistration(f.registrationId), p: f.pricing, s: f.stateCode.trim() };
    case "vat_uk":
      return { profile: f.profile, r: normalRegistration(f.registrationId), p: f.pricing };
    case "sales_tax_us":
      return { profile: f.profile, j: f.jurisdiction.map((j) => [j.name.trim(), parsePercent(j.rate) ?? j.rate.trim()]) };
  }
}

export function taxChanged(draft: TaxForm, saved: TaxForm): boolean {
  return JSON.stringify(effective(draft)) !== JSON.stringify(effective(saved));
}

export type TaxField = "profile" | "registration_id" | "pricing" | "state_code" | "jurisdiction";
const taxFields: readonly TaxField[] = ["profile", "registration_id", "pricing", "state_code", "jurisdiction"];

export const taxProblemOf = (err: unknown): Problem<TaxField> => problemWith(err, taxFields);

/** What can be refused without asking the server, in the order the form reads. */
export function taxProblem(f: TaxForm): Problem<TaxField> | null {
  if ((f.profile === "gst_in" || f.profile === "vat_uk") && normalRegistration(f.registrationId) === "") {
    return {
      field: "registration_id",
      message: f.profile === "gst_in" ? "Enter the venue's GSTIN: invoices print it." : "Enter the venue's VAT number: receipts print it.",
    };
  }
  if (f.profile === "sales_tax_us") {
    if (f.jurisdiction.length === 0) return { field: "jurisdiction", message: "Add the rate where the venue is, like CA at 7.25%." };
    if (f.jurisdiction.some((j) => j.name.trim() === "" || parsePercent(j.rate) === null)) {
      return { field: "jurisdiction", message: "Each rate needs a name and a rate from 0% to 100%." };
    }
  }
  return null;
}

/** What Save sends. US sales tax has no inclusive pricing, and the server clears what a profile does not use. */
export function toTaxInput(f: TaxForm): TaxSettingsRequest {
  return {
    profile: f.profile,
    registration_id: registrationLabel(f.profile) ? f.registrationId.trim() : "",
    pricing: f.profile === "none" || f.profile === "sales_tax_us" ? "" : f.pricing,
    state_code: f.profile === "gst_in" ? f.stateCode.trim() : "",
    jurisdiction:
      f.profile === "sales_tax_us"
        ? f.jurisdiction.map((j) => ({ name: j.name.trim(), rate: parsePercent(j.rate) ?? 0 }))
        : [],
  };
}

// ---- the items a profile still owes --------------------------------------

/** Where an item opens in the menu editor. */
export const menuItemPath = (id: number): string => `/menu?item=${id}`;

/**
 * Whether to list the items without an HSN: only while the form is still the
 * profile the server holds, since the list is what that profile owes. A draft
 * of another profile has no list until it is saved.
 */
export function showWithoutHSN(tax: Pick<TaxSettingsResponse, "profile" | "needs_hsn" | "items_without_hsn">, draft: ProfileId): boolean {
  return tax.needs_hsn && tax.items_without_hsn.length > 0 && draft === tax.profile;
}

/** The items to name on screen, and how many more there are. */
export function namedItems<T>(items: readonly T[], max = 8): { shown: T[]; more: number } {
  return { shown: items.slice(0, max), more: Math.max(0, items.length - max) };
}

// ---- tax classes -----------------------------------------------------------

export interface ClassForm {
  name: string;
  /** Percent, as typed. Ignored when exempt. */
  rate: string;
  exempt: boolean;
}

export const emptyClassForm = (): ClassForm => ({ name: "", rate: "", exempt: false });

export function classFormFrom(c: TaxClassResponse): ClassForm {
  return { name: c.name, rate: c.exempt ? "" : percentInput(c.rate), exempt: c.exempt };
}

export type ClassField = "name" | "rate";
const classFields: readonly ClassField[] = ["name", "rate"];

export const classProblemOf = (err: unknown): Problem<ClassField> => problemWith(err, classFields);

export const MAX_CLASS_NAME = 40;

export function classProblem(f: ClassForm): Problem<ClassField> | null {
  const name = f.name.trim();
  if (name === "") return { field: "name", message: "Give the tax class a name, like Food or Alcohol." };
  if ([...name].length > MAX_CLASS_NAME) return { field: "name", message: `Keep the name to ${MAX_CLASS_NAME} characters.` };
  if (!f.exempt && parsePercent(f.rate) === null) return { field: "rate", message: "Enter a rate from 0% to 100%." };
  return null;
}

export function toClassInput(f: ClassForm): TaxClassRequest {
  return { name: f.name.trim(), rate: f.exempt ? 0 : (parsePercent(f.rate) ?? 0), exempt: f.exempt };
}

/** A class as a line: "Food, 5%" or "Water, exempt". */
export function classRateText(c: Pick<TaxClassResponse, "rate" | "exempt">): string {
  return c.exempt ? "Exempt" : percentText(c.rate);
}
