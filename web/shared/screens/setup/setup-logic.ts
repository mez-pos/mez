// What the setup wizard knows, apart from how it is drawn
// (docs/design/screens/setup-wizard.md): its answers and their defaults, what
// each step checks before going on, and the stages that turn the answers into
// a venue this device is signed in on.

import { ApiError, type ApiClient } from "../../api";
import type { NumberingRequest, NumberingResponse, SetupRequest, SetupResponse } from "../../api-types";
import { formatWait } from "../../auth/format";
import { unreachableMessage } from "../../auth/messages";
import { setupHeaders } from "./setup-token";

export type TaxProfile = "none" | "gst_in" | "vat_uk" | "sales_tax_us";
export type Pricing = "inclusive" | "exclusive";

/** Everything the wizard asks, as typed. */
export interface Answers {
  readonly name: string;
  readonly legalName: string;
  readonly currency: string;
  readonly timezone: string;
  readonly deviceName: string;
  readonly taxProfile: TaxProfile;
  readonly taxRegistration: string;
  readonly pricing: Pricing;
  readonly fyStartMonth: number;
  readonly invoicePrefix: string;
  readonly ownerName: string;
  readonly pin: string;
  readonly pinAgain: string;
}

/**
 * A field as the server names it, so its refusals land where they belong.
 * `pin_again` and `invoice_prefix` are the wizard's own: the server's setup
 * takes neither.
 */
export type Field =
  | "name"
  | "legal_name"
  | "currency"
  | "timezone"
  | "device_name"
  | "tax_profile"
  | "tax_registration"
  | "tax_pricing"
  | "fy_start_month"
  | "invoice_prefix"
  | "owner_name"
  | "owner_pin"
  | "pin_again";

export type Problems = Partial<Record<Field, string>>;

export const steps = [
  { key: "venue", name: "Your venue" },
  { key: "taxes", name: "Taxes and invoices" },
  { key: "you", name: "You" },
] as const;

export type StepKey = (typeof steps)[number]["key"];

/**
 * Every step the wizard shows: the questions, then what is done once the
 * venue exists and the owner is signed in. Those are not questions: they act
 * through the API as they go, and each can be skipped.
 */
export const allSteps = [...steps, { key: "printer", name: "Printer" }, { key: "ready", name: "More devices" }] as const;

const fieldsOf: Readonly<Record<StepKey, readonly Field[]>> = {
  venue: ["name", "legal_name", "currency", "timezone", "device_name"],
  taxes: ["tax_profile", "tax_registration", "tax_pricing", "fy_start_month", "invoice_prefix"],
  you: ["owner_name", "owner_pin", "pin_again"],
};

/** The step a field is asked on, for sending a refusal back to it. */
export function stepOf(field: string): StepKey | null {
  for (const step of steps) {
    if ((fieldsOf[step.key] as readonly string[]).includes(field)) return step.key;
  }
  return null;
}

/** What is usual under each tax profile, and what it asks for. */
export interface ProfileFacts {
  readonly label: string;
  readonly blurb: string;
  readonly currency: string | null;
  readonly fyStartMonth: number | null;
  readonly pricing: Pricing;
  /** Whether the venue may choose how prices are quoted. */
  readonly pricingChoice: boolean;
  /** The registration number it needs, or null for none. */
  readonly registration: { readonly label: string; readonly hint: string; readonly required: boolean } | null;
}

export const profiles: Readonly<Record<TaxProfile, ProfileFacts>> = {
  none: {
    label: "No tax",
    blurb: "Nothing is added and no tax lines print.",
    currency: null,
    fyStartMonth: null,
    pricing: "inclusive",
    pricingChoice: false,
    registration: null,
  },
  gst_in: {
    label: "GST, India",
    blurb: "CGST and SGST on every bill, with your GSTIN.",
    currency: "INR",
    fyStartMonth: 4,
    pricing: "inclusive",
    pricingChoice: true,
    registration: { label: "GSTIN", hint: "15 characters. Its first two digits are your state.", required: true },
  },
  vat_uk: {
    label: "VAT, UK",
    blurb: "VAT shown on every receipt, with your VAT number.",
    currency: "GBP",
    fyStartMonth: 4,
    pricing: "inclusive",
    pricingChoice: true,
    registration: { label: "VAT number", hint: "As it is on your VAT certificate.", required: true },
  },
  sales_tax_us: {
    label: "Sales tax, US",
    blurb: "Tax added at the till. Your rates are set in Settings afterwards.",
    currency: "USD",
    fyStartMonth: 1,
    pricing: "exclusive",
    pricingChoice: false,
    registration: null,
  },
};

export const profileOrder: readonly TaxProfile[] = ["none", "gst_in", "vat_uk", "sales_tax_us"];

/** The currencies offered. The server takes any ISO code; these are the ones a first venue is likely to be in. */
export const currencies: readonly { code: string; name: string }[] = [
  { code: "INR", name: "Indian rupee" },
  { code: "GBP", name: "Pound sterling" },
  { code: "USD", name: "US dollar" },
  { code: "EUR", name: "Euro" },
  { code: "AED", name: "UAE dirham" },
  { code: "SGD", name: "Singapore dollar" },
  { code: "AUD", name: "Australian dollar" },
  { code: "CAD", name: "Canadian dollar" },
  { code: "LKR", name: "Sri Lankan rupee" },
  { code: "NPR", name: "Nepalese rupee" },
];

export const months = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"] as const;

/** The invoice prefix a venue starts with: the default series' code. */
export const defaultPrefix = "INV";

export const minOwnerPin = 6;
export const maxOwnerPin = 8;

/** The profile a venue in this time zone most likely needs. A guess to start from, shown and changeable. */
export function guessProfile(timezone: string): TaxProfile {
  if (timezone === "Asia/Kolkata" || timezone === "Asia/Calcutta") return "gst_in";
  if (timezone === "Europe/London") return "vat_uk";
  if (timezone.startsWith("America/") || timezone.startsWith("US/") || timezone === "Pacific/Honolulu") return "sales_tax_us";
  return "none";
}

/** Blank answers for a browser in `timezone`. */
export function startingAnswers(timezone: string): Answers {
  const taxProfile = guessProfile(timezone);
  const facts = profiles[taxProfile];
  return {
    name: "",
    legalName: "",
    currency: facts.currency ?? "INR",
    timezone,
    deviceName: "Counter 1",
    taxProfile,
    taxRegistration: "",
    pricing: facts.pricing,
    fyStartMonth: facts.fyStartMonth ?? 1,
    invoicePrefix: defaultPrefix,
    ownerName: "",
    pin: "",
    pinAgain: "",
  };
}

/**
 * The answers after choosing a tax profile. The currency, the financial year
 * and how prices are quoted follow the profile, except any the owner has
 * already set by hand (`touched`): choosing GST must not undo a currency
 * someone picked on purpose.
 */
export function withProfile(answers: Answers, profile: TaxProfile, touched: ReadonlySet<keyof Answers>): Answers {
  const facts = profiles[profile];
  return {
    ...answers,
    taxProfile: profile,
    currency: facts.currency !== null && !touched.has("currency") ? facts.currency : answers.currency,
    fyStartMonth: facts.fyStartMonth !== null && !touched.has("fyStartMonth") ? facts.fyStartMonth : answers.fyStartMonth,
    // Where the profile leaves no choice, the choice is not the owner's to keep.
    pricing: facts.pricingChoice && touched.has("pricing") ? answers.pricing : facts.pricing,
  };
}

const gstin = /^[0-9]{2}[0-9A-Za-z]{13}$/;

/**
 * What a step will not go on without. The server judges everything again:
 * this is here so that most refusals are said beside the field while the
 * owner is still on it, not after the last step.
 */
export function checkStep(step: StepKey, a: Answers, timezones: readonly string[] = []): Problems {
  const problems: Problems = {};
  if (step === "venue") {
    if (a.name.trim() === "") problems.name = "Give the venue a name.";
    else if ([...a.name.trim()].length > 60) problems.name = "Keep the name to 60 characters.";
    if ([...a.legalName.trim()].length > 120) problems.legal_name = "Keep the name on invoices to 120 characters.";
    if (!/^[A-Za-z]{3}$/.test(a.currency.trim())) problems.currency = "Choose a currency.";
    if (a.timezone.trim() === "") problems.timezone = "Choose the venue's time zone.";
    else if (timezones.length > 0 && !timezones.includes(a.timezone.trim())) problems.timezone = "Choose a time zone from the list, such as Asia/Kolkata.";
    if (a.deviceName.trim() === "") problems.device_name = "Give this till a name.";
  }
  if (step === "taxes") {
    const facts = profiles[a.taxProfile];
    const registration = a.taxRegistration.trim();
    if (a.taxProfile === "gst_in" && !gstin.test(registration)) {
      problems.tax_registration = "Enter the venue's 15 character GSTIN. It begins with two digits.";
    } else if (facts.registration?.required && registration === "") {
      problems.tax_registration = `Enter the venue's ${facts.registration.label}.`;
    }
    if (!Number.isInteger(a.fyStartMonth) || a.fyStartMonth < 1 || a.fyStartMonth > 12) problems.fy_start_month = "Choose a month.";
  }
  if (step === "you") {
    if (a.ownerName.trim() === "") problems.owner_name = "Enter your name.";
    if (!/^\d*$/.test(a.pin)) problems.owner_pin = "A PIN is digits only.";
    else if (a.pin.length < minOwnerPin || a.pin.length > maxOwnerPin) problems.owner_pin = `Choose a PIN of ${minOwnerPin} to ${maxOwnerPin} digits.`;
    else if (a.pinAgain !== a.pin) problems.pin_again = "The two PINs are not the same.";
  }
  return problems;
}

/** The first step with something wrong on it, and what, or null when the answers can be sent. */
export function firstProblem(a: Answers, timezones: readonly string[] = []): { step: StepKey; problems: Problems } | null {
  for (const step of steps) {
    const problems = checkStep(step.key, a, timezones);
    if (Object.keys(problems).length > 0) return { step: step.key, problems };
  }
  return null;
}

/** The answers as POST /api/v1/setup takes them. */
export function toRequest(a: Answers): SetupRequest {
  const facts = profiles[a.taxProfile];
  const legal = a.legalName.trim();
  const registration = facts.registration ? a.taxRegistration.trim() : "";
  return {
    name: a.name.trim(),
    ...(legal !== "" ? { legal_name: legal } : {}),
    currency: a.currency.trim().toUpperCase(),
    timezone: a.timezone.trim(),
    fy_start_month: a.fyStartMonth,
    tax_profile: a.taxProfile,
    ...(registration !== "" ? { tax_registration: registration } : {}),
    // Sent only where it was a choice: otherwise the profile's own stands.
    ...(facts.pricingChoice ? { tax_pricing: a.pricing } : {}),
    owner_name: a.ownerName.trim(),
    owner_pin: a.pin,
    device_name: a.deviceName.trim(),
    device_kind: "terminal",
  };
}

/** The stages of Create, in order. Each is done once; a failure resumes at its own stage. */
export type Stage = "create" | "pair" | "sign_in" | "prefix" | "done";

export const stageWords: Readonly<Record<Stage, string>> = {
  create: "Creating the venue",
  pair: "Pairing this till",
  sign_in: "Signing you in",
  prefix: "Setting the invoice prefix",
  done: "Done",
};

/** How far Create has got. `made` is what the server answered when it made the venue. */
export interface Progress {
  readonly stage: Stage;
  readonly made: SetupResponse | null;
}

export const notStarted: Progress = { stage: "create", made: null };

/** What Create needs of the app: the client, and the auth store's pairing and sign-in. */
export interface SetupDeps {
  readonly api: ApiClient;
  /** The setup token this tab holds, or null (ADR-270). */
  readonly token: string | null;
  pair(code: string): Promise<void>;
  signIn(staffId: string, pin: string): Promise<void>;
}

/**
 * What to do about a failed stage.
 *   field    a refused answer: go back to its step
 *   retry    nothing lasting went wrong: the same stage can be tried again
 *   taken    a venue already exists, and this wizard did not make it
 *   no_code  our venue exists but this device has no code that pairs it
 *   prefix   all done but the prefix, which the server refused
 *   locked   this browser does not hold the setup token: trying again cannot help
 */
export type FailureKind = "field" | "retry" | "taken" | "no_code" | "prefix" | "locked";

/** What to do when the wizard is open in a browser that does not hold the setup token. */
export const lockedWords =
  "For the venue's safety, setting up starts from the installer. Run the mez installer again and leave Open mez now ticked on its last page. If you started mez yourself, use the setup link it printed, or run mez setup-link.";

export interface Failure {
  readonly kind: FailureKind;
  readonly message: string;
  readonly field: Field | null;
}

export interface Outcome {
  readonly progress: Progress;
  readonly failure: Failure | null;
}

/**
 * Runs Create from wherever `from` says it had got to, reporting each stage
 * as it starts. It never throws: a failure comes back with the stage it
 * stopped at, so trying again makes no second venue and redeems no code twice.
 */
export async function finishSetup(
  deps: SetupDeps,
  answers: Answers,
  from: Progress,
  host: string,
  onStage: (p: Progress) => void = () => {},
): Promise<Outcome> {
  let progress = from;
  const at = (next: Progress) => {
    progress = next;
    onStage(next);
  };
  try {
    if (progress.stage === "create") {
      at(progress);
      const made = await deps.api.post<SetupResponse>("/api/v1/setup", toRequest(answers), { headers: setupHeaders(deps.token) });
      at({ stage: "pair", made });
    }
    const made = progress.made;
    if (!made) throw new Error("setup: a stage after create with no venue");
    if (progress.stage === "pair") {
      at(progress);
      await deps.pair(made.pairing_code);
      at({ stage: "sign_in", made });
    }
    if (progress.stage === "sign_in") {
      at(progress);
      await deps.signIn(made.owner_id, answers.pin);
      at({ stage: "prefix", made });
    }
    if (progress.stage === "prefix") {
      at(progress);
      await setPrefix(deps.api, answers.invoicePrefix);
      at({ stage: "done", made });
    }
    return { progress, failure: null };
  } catch (err) {
    return { progress, failure: failureOf(progress.stage, err, host) };
  }
}

/** Sets the invoice prefix, when it is not the one the venue has. Everything else about numbering is left as it stands. */
async function setPrefix(api: ApiClient, wanted: string): Promise<void> {
  const prefix = wanted.trim();
  if (prefix === "") return;
  const now = await api.get<NumberingResponse>("/api/v1/settings/numbering");
  if (now.invoice_prefix === prefix) return;
  const body: NumberingRequest = {
    invoice_series: now.invoice_series,
    credit_series: now.credit_series,
    invoice_prefix: prefix,
    credit_prefix: now.credit_prefix,
    template: now.template,
    financial_year_start_month: now.financial_year_start_month,
    cash_rounding_step: now.cash_rounding_step,
  };
  await api.put<NumberingResponse>("/api/v1/settings/numbering", body);
}

const noCode = "The venue is set up, but this till has no code to pair with. On this computer, run mez pair and enter the code it prints.";

/** A failed stage, in words for the owner, and what the wizard should offer. */
export function failureOf(stage: Stage, err: unknown, host: string): Failure {
  const retry = (message: string): Failure => ({ kind: "retry", message, field: null });
  if (!(err instanceof ApiError)) {
    // WrongDeviceKind, or anything unforeseen. After the venue exists, the way on is a fresh code.
    return stage === "pair" ? { kind: "no_code", message: noCode, field: null } : retry("Something went wrong. Try again.");
  }
  if (err.unreachable) return retry(unreachableMessage(host));
  if (err.code === "rate_limited") {
    return retry(
      err.retryAfterMs !== null ? `Too many tries. Wait ${formatWait(err.retryAfterMs)} and try again.` : "Too many tries. Wait a minute and try again.",
    );
  }
  switch (stage) {
    case "create":
      switch (err.code) {
        case "invalid_setup": {
          const field = err.field !== null && stepOf(err.field) !== null ? (err.field as Field) : null;
          return { kind: field ? "field" : "retry", message: sentence(err.message), field };
        }
        case "already_set_up":
          return { kind: "taken", message: "This venue has already been set up. Pair this device to use it.", field: null };
        case "setup_no_code":
          return { kind: "no_code", message: noCode, field: null };
        case "setup_locked":
          return { kind: "locked", message: lockedWords, field: null };
        case "setup_not_here":
          return retry("The venue is set up on the computer mez runs on. Open mez there, by its shortcut or at localhost.");
        default:
          return retry(`The server refused: ${err.message}`);
      }
    case "pair":
      // The code lasts ten minutes and works once: one that fails will not work on a second try.
      return err.code === "invalid_pairing_code" ? { kind: "no_code", message: noCode, field: null } : retry(`The server refused: ${err.message}`);
    case "prefix":
      return err.code === "invalid_setting" || err.code === "number_taken"
        ? { kind: "prefix", message: `The venue is ready, but the invoice prefix was not set. ${err.message} You can change it in Settings.`, field: null }
        : retry(`The venue is ready, but the invoice prefix was not set: ${err.message}`);
    default:
      return retry(`The server refused: ${err.message}`);
  }
}

/** The server's refusals begin in lower case, as Go errors do. Under a field they read as a sentence. */
function sentence(message: string): string {
  const text = message.trim();
  if (text === "") return "That cannot be used.";
  const first = text.charAt(0).toUpperCase() + text.slice(1);
  return /[.!?]$/.test(first) ? first : `${first}.`;
}

/** The IANA zones this browser knows, with `current` among them. Empty where the browser cannot say. */
export function timezoneList(current: string): readonly string[] {
  const intl = Intl as typeof Intl & { supportedValuesOf?: (key: string) => string[] };
  let zones: string[] = [];
  try {
    zones = intl.supportedValuesOf?.("timeZone") ?? [];
  } catch {
    zones = [];
  }
  if (zones.length === 0) return [];
  return zones.includes(current) || current === "" ? zones : [current, ...zones];
}

// Chrome still reports a few zones by the names they had before they were
// renamed. Both names work; the one an owner recognises is offered.
const renamedZones: Readonly<Record<string, string>> = {
  "Asia/Calcutta": "Asia/Kolkata",
  "Asia/Katmandu": "Asia/Kathmandu",
  "Asia/Rangoon": "Asia/Yangon",
  "Asia/Saigon": "Asia/Ho_Chi_Minh",
};

/** A zone by the name it has now. */
export function currentZoneName(zone: string): string {
  return renamedZones[zone] ?? zone;
}

/** The time zone this browser is in, or empty where it cannot say. */
export function browserTimezone(): string {
  try {
    return currentZoneName(Intl.DateTimeFormat().resolvedOptions().timeZone ?? "");
  } catch {
    return "";
  }
}
