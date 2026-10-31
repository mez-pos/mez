import { describe, expect, it } from "vitest";
import { ApiError, type ApiClient } from "../../api";
import type { NumberingResponse, SetupResponse } from "../../api-types";
import { WrongDeviceKind } from "../../auth/auth-store";
import {
  allSteps,
  checkStep,
  currentZoneName,
  failureOf,
  finishSetup,
  firstProblem,
  guessProfile,
  indexOfAfter,
  nextAfter,
  profileOrder,
  profiles,
  notStarted,
  startingAnswers,
  stepOf,
  timezoneList,
  zoneOptions,
  toRequest,
  withProfile,
  type Answers,
  type Progress,
  type SetupDeps,
  type Stage,
} from "./setup-logic";

const filled: Answers = {
  ...startingAnswers("Asia/Kolkata"),
  name: "Filter Coffee House",
  taxRegistration: "29ABCDE1234F1Z5",
  ownerName: "Priya",
  pin: "246813",
  pinAgain: "246813",
};

const made: SetupResponse = { venue_id: "ven_1", owner_id: "stf_1", pairing_code: "ABCD-2345", expires_at: "2026-10-07T04:40:00Z" };

const numbering: NumberingResponse = {
  invoice_series: "INV",
  credit_series: "CN",
  invoice_prefix: "INV",
  credit_prefix: "CN",
  template: "{prefix}/{fy:short}/{seq:06d}",
  financial_year_start_month: 4,
  cash_rounding_step: 1,
  currency: "INR",
  next_invoice_no: "INV/26-27/000001",
  next_credit_no: "CN/26-27/000001",
  number_limit: 16,
  fy_locked: false,
  max_rounding_step: 100,
};

const refusal = (
  code: ConstructorParameters<typeof ApiError>[0],
  message: string,
  status: number,
  field: string | null = null,
  retryAfterMs: number | null = null,
) => new ApiError(code, message, status, field, retryAfterMs);

/** Deps that record what was asked, and fail where `fail` says. */
function deps(fail: Partial<Record<string, unknown>> = {}) {
  const calls: string[] = [];
  const bodies: Record<string, unknown> = {};
  const step = async <T>(name: string, value: T, body?: unknown): Promise<T> => {
    if (name in fail) {
      const err = fail[name];
      delete fail[name]; // a failure happens once, so a retry can pass
      throw err;
    }
    // Only what the server did is counted: a call that failed did nothing.
    calls.push(name);
    if (body !== undefined) bodies[name] = body;
    return value;
  };
  const api: ApiClient = {
    get: <T>(path: string) => step(`GET ${path}`, numbering as T),
    post: <T>(path: string, body?: unknown) => step(`POST ${path}`, made as T, body),
    put: <T>(path: string, body?: unknown) => step(`PUT ${path}`, numbering as T, body),
    del: <T>(path: string) => step(`DELETE ${path}`, undefined as T),
  };
  const d: SetupDeps = {
    api,
    token: null,
    pair: (code) => step(`pair ${code}`, undefined),
    signIn: (id, pin) => step(`signIn ${id} ${pin.length} digits`, undefined),
  };
  return { d, calls, bodies };
}

describe("startingAnswers and guessProfile", () => {
  it.each([
    ["Asia/Kolkata", "gst_in", "INR", 4],
    ["Asia/Calcutta", "gst_in", "INR", 4],
    ["Europe/London", "vat_uk", "GBP", 4],
    ["America/Chicago", "sales_tax_us", "USD", 1],
    ["Europe/Paris", "none", "INR", 1],
    ["", "none", "INR", 1],
  ])("in %s starts as %s", (zone, profile, currency, month) => {
    expect(guessProfile(zone)).toBe(profile);
    const a = startingAnswers(zone);
    expect([a.taxProfile, a.currency, a.fyStartMonth, a.timezone]).toEqual([profile, currency, month, zone]);
  });

  it("starts with a till name and the default prefix, and no PIN", () => {
    const a = startingAnswers("Asia/Kolkata");
    expect([a.deviceName, a.invoicePrefix, a.pin, a.name]).toEqual(["Counter 1", "INV", "", ""]);
  });
});

describe("withProfile", () => {
  const none = new Set<keyof Answers>();

  it("takes the profile's currency, year and pricing", () => {
    const a = withProfile(startingAnswers("Asia/Kolkata"), "sales_tax_us", none);
    expect([a.taxProfile, a.currency, a.fyStartMonth, a.pricing]).toEqual(["sales_tax_us", "USD", 1, "exclusive"]);
  });

  it("keeps a currency and a year the owner set by hand", () => {
    const start = { ...startingAnswers("Asia/Kolkata"), currency: "AED", fyStartMonth: 7 };
    const a = withProfile(start, "vat_uk", new Set<keyof Answers>(["currency", "fyStartMonth"]));
    expect([a.currency, a.fyStartMonth]).toEqual(["AED", 7]);
  });

  it("keeps a chosen pricing only where the profile lets it be chosen", () => {
    const start = { ...startingAnswers("Asia/Kolkata"), pricing: "exclusive" as const };
    const touched = new Set<keyof Answers>(["pricing"]);
    expect(withProfile(start, "vat_uk", touched).pricing).toBe("exclusive");
    expect(withProfile({ ...start, pricing: "inclusive" }, "sales_tax_us", touched).pricing).toBe("exclusive");
  });

  it("leaves the currency alone under No tax, and does not change the answers it was given", () => {
    const start = startingAnswers("Europe/London");
    const a = withProfile(start, "none", none);
    expect([a.currency, a.fyStartMonth]).toEqual(["GBP", 4]);
    expect(start.taxProfile).toBe("vat_uk");
  });
});

describe("checkStep", () => {
  const zones = ["Asia/Kolkata", "Europe/London"];

  it.each<[string, Parameters<typeof checkStep>[0], Partial<Answers>, string | null]>([
    ["a filled venue", "venue", {}, null],
    ["no name", "venue", { name: "  " }, "name"],
    ["a name of 61 characters", "venue", { name: "a".repeat(61) }, "name"],
    ["a name of 60 characters in Devanagari", "venue", { name: "क".repeat(60) }, null],
    ["a legal name of 121 characters", "venue", { legalName: "a".repeat(121) }, "legal_name"],
    ["no currency", "venue", { currency: "" }, "currency"],
    ["a currency that is a word", "venue", { currency: "rupees" }, "currency"],
    ["no time zone", "venue", { timezone: "" }, "timezone"],
    ["a time zone not on the list", "venue", { timezone: "Mars/Olympus" }, "timezone"],
    ["no till name", "venue", { deviceName: " " }, "device_name"],
    ["GST with its GSTIN", "taxes", {}, null],
    ["GST with a GSTIN in small letters", "taxes", { taxRegistration: "29abcde1234f1z5" }, null],
    ["GST with no GSTIN", "taxes", { taxRegistration: "" }, "tax_registration"],
    ["GST with a short GSTIN", "taxes", { taxRegistration: "29ABC" }, "tax_registration"],
    ["GST with a GSTIN that names no state", "taxes", { taxRegistration: "ABABCDE1234F1Z5" }, "tax_registration"],
    ["the composition scheme with its GSTIN", "taxes", { taxProfile: "gst_in_composition" }, null],
    ["the composition scheme with no GSTIN", "taxes", { taxProfile: "gst_in_composition", taxRegistration: "" }, "tax_registration"],
    ["the composition scheme with a VAT number", "taxes", { taxProfile: "gst_in_composition", taxRegistration: "GB123456789" }, "tax_registration"],
    ["VAT with no number", "taxes", { taxProfile: "vat_uk", taxRegistration: " " }, "tax_registration"],
    ["no tax needs no number", "taxes", { taxProfile: "none", taxRegistration: "" }, null],
    ["US sales tax needs no number", "taxes", { taxProfile: "sales_tax_us", taxRegistration: "" }, null],
    ["a financial year starting in month 13", "taxes", { fyStartMonth: 13 }, "fy_start_month"],
    ["a filled owner", "you", {}, null],
    ["no owner name", "you", { ownerName: "" }, "owner_name"],
    ["a PIN of five digits", "you", { pin: "24681", pinAgain: "24681" }, "owner_pin"],
    ["a PIN of nine digits", "you", { pin: "246813579", pinAgain: "246813579" }, "owner_pin"],
    ["a PIN with a letter", "you", { pin: "24681a", pinAgain: "24681a" }, "owner_pin"],
    ["a PIN typed differently the second time", "you", { pinAgain: "246814" }, "pin_again"],
  ])("%s", (_name, step, change, field) => {
    const problems = checkStep(step, { ...filled, ...change }, zones);
    expect(Object.keys(problems)).toEqual(field ? [field] : []);
  });

  it("takes any time zone where the browser has no list to check against", () => {
    expect(checkStep("venue", { ...filled, timezone: "Asia/Calcutta" })).toEqual({});
  });

  it("finds the first step with a problem, or none", () => {
    expect(firstProblem(filled)).toBeNull();
    expect(firstProblem({ ...filled, taxRegistration: "", ownerName: "" })?.step).toBe("taxes");
  });

  it("knows which step asks each field", () => {
    expect([stepOf("timezone"), stepOf("tax_registration"), stepOf("owner_pin"), stepOf("nonsense")]).toEqual(["venue", "taxes", "you", null]);
  });
});

describe("toRequest", () => {
  it("sends the answers trimmed, as a terminal", () => {
    expect(toRequest({ ...filled, name: " Filter Coffee House ", currency: "inr", legalName: " " })).toEqual({
      name: "Filter Coffee House",
      currency: "INR",
      timezone: "Asia/Kolkata",
      fy_start_month: 4,
      tax_profile: "gst_in",
      tax_registration: "29ABCDE1234F1Z5",
      tax_pricing: "inclusive",
      owner_name: "Priya",
      owner_pin: "246813",
      device_name: "Counter 1",
      device_kind: "terminal",
    });
  });

  it("sends a legal name only when there is one", () => {
    expect(toRequest({ ...filled, legalName: "Filter Coffee House Pvt Ltd" }).legal_name).toBe("Filter Coffee House Pvt Ltd");
  });

  it("sends no registration and no pricing where the profile has neither", () => {
    const req = toRequest({ ...filled, taxProfile: "sales_tax_us", taxRegistration: "left over from GST", pricing: "inclusive" });
    expect("tax_registration" in req || "tax_pricing" in req).toBe(false);
  });

  it("sends the composition scheme's GSTIN and no pricing: nothing is added to a price (ADR-401)", () => {
    const req = toRequest({ ...filled, taxProfile: "gst_in_composition", pricing: "exclusive" });
    expect(req.tax_profile).toBe("gst_in_composition");
    expect(req.tax_registration).toBe("29ABCDE1234F1Z5");
    expect("tax_pricing" in req).toBe(false);
  });

  it("offers the composition scheme after GST", () => {
    expect(profileOrder.indexOf("gst_in_composition")).toBe(profileOrder.indexOf("gst_in") + 1);
    expect(profiles.gst_in_composition).toMatchObject({ label: "GST composition, India", currency: "INR", fyStartMonth: 4, pricingChoice: false });
  });

  it("never sends the wizard's own fields", () => {
    expect(Object.keys(toRequest(filled)).some((k) => k === "pin_again" || k === "invoice_prefix")).toBe(false);
  });
});

describe("finishSetup", () => {
  const host = "localhost:7777";

  it("makes the venue, pairs, signs in, and leaves a default prefix alone", async () => {
    const { d, calls, bodies } = deps();
    const stages: Stage[] = [];
    const out = await finishSetup(d, filled, notStarted, host, (p) => stages.push(p.stage));

    expect(out).toEqual({ progress: { stage: "done", made }, failure: null });
    expect(calls).toEqual(["POST /api/v1/setup", "pair ABCD-2345", "signIn stf_1 6 digits", "GET /api/v1/settings/numbering"]);
    expect(bodies["POST /api/v1/setup"]).toEqual(toRequest(filled));
    expect(stages).toEqual(["create", "pair", "pair", "sign_in", "sign_in", "prefix", "prefix", "done"]);
  });

  it("sets a prefix that was changed, and nothing else about numbering", async () => {
    const { d, calls, bodies } = deps();
    await finishSetup(d, { ...filled, invoicePrefix: " FCH " }, notStarted, host);

    expect(calls.at(-1)).toBe("PUT /api/v1/settings/numbering");
    expect(bodies["PUT /api/v1/settings/numbering"]).toEqual({
      invoice_series: "INV",
      credit_series: "CN",
      invoice_prefix: "FCH",
      credit_prefix: "CN",
      template: "{prefix}/{fy:short}/{seq:06d}",
      financial_year_start_month: 4,
      cash_rounding_step: 1,
    });
  });

  it("asks nothing about numbering when the prefix is left empty", async () => {
    const { d, calls } = deps();
    await finishSetup(d, { ...filled, invoicePrefix: "  " }, notStarted, host);
    expect(calls).toHaveLength(3);
  });

  it.each<[string, string, Stage]>([
    ["making the venue", "POST /api/v1/setup", "create"],
    ["pairing", "pair ABCD-2345", "pair"],
    ["signing in", "signIn stf_1 6 digits", "sign_in"],
    ["reading the numbering", "GET /api/v1/settings/numbering", "prefix"],
  ])("stops where %s fails, and trying again carries on from there", async (_name, call, stage) => {
    const { d, calls } = deps({ [call]: new ApiError("network", "the server could not be reached") });
    const first = await finishSetup(d, filled, notStarted, host);

    expect(first.progress.stage).toBe(stage);
    expect(first.failure?.kind).toBe("retry");
    expect(first.progress.made).toEqual(stage === "create" ? null : made);

    const again = await finishSetup(d, filled, first.progress, host);
    expect(again).toEqual({ progress: { stage: "done", made }, failure: null });
    // The venue is made once and the code redeemed once, however many tries it took.
    expect(calls.filter((c) => c === "POST /api/v1/setup")).toHaveLength(1);
    expect(calls.filter((c) => c.startsWith("pair "))).toHaveLength(1);
    expect(calls.filter((c) => c.startsWith("signIn "))).toHaveLength(1);
  });

  it("does nothing when everything is already done", async () => {
    const { d, calls } = deps();
    const done: Progress = { stage: "done", made };
    expect(await finishSetup(d, filled, done, host)).toEqual({ progress: done, failure: null });
    expect(calls).toEqual([]);
  });

  it("fails, and does not pair, when asked to resume with no venue", async () => {
    const { d, calls } = deps();
    const out = await finishSetup(d, filled, { stage: "pair", made: null }, host);
    expect(out.failure).not.toBeNull();
    expect(calls).toEqual([]);
  });
});

describe("failureOf", () => {
  const host = "localhost:7777";

  it("sends a refused answer back to its field, as a sentence", () => {
    const f = failureOf("create", refusal("invalid_setup", 'timezone "Mars/Olympus": unknown time zone', 422, "timezone"), host);
    expect(f).toEqual({ kind: "field", field: "timezone", message: 'Timezone "Mars/Olympus": unknown time zone.' });
  });

  it("offers a retry for a refused answer that names no field the wizard asks", () => {
    expect(failureOf("create", refusal("invalid_setup", "the first device: too long", 422, "device_kind"), host).kind).toBe("retry");
  });

  it.each<[string, Stage, unknown, string]>([
    ["a venue someone else made", "create", refusal("already_set_up", "This venue is already set up.", 409), "taken"],
    ["a venue made without its code", "create", refusal("setup_no_code", "no code", 500), "no_code"],
    ["a browser that is not at the machine", "create", refusal("setup_not_here", "not here", 403), "retry"],
    ["a browser without the setup token", "create", refusal("setup_locked", "locked", 403), "locked"],
    ["too many tries", "create", refusal("rate_limited", "slow down", 429, null, 42_000), "retry"],
    ["a server that cannot be reached", "pair", new ApiError("timeout", "no answer"), "retry"],
    ["a code that has expired", "pair", refusal("invalid_pairing_code", "not valid", 400, "code"), "no_code"],
    ["a code for another kind of device", "pair", new WrongDeviceKind("kitchen", "terminal"), "no_code"],
    ["a sign-in the server refused", "sign_in", refusal("internal", "internal error", 500), "retry"],
    ["a prefix the server will not take", "prefix", refusal("invalid_setting", "Use capitals and digits.", 422, "invoice_prefix"), "prefix"],
    ["a prefix that could not be saved", "prefix", refusal("internal", "internal error", 500), "retry"],
    ["something that is not the server's", "create", new TypeError("x is undefined"), "retry"],
  ])("%s", (_name, stage, err, kind) => {
    const f = failureOf(stage, err, host);
    expect(f.kind).toBe(kind);
    expect(f.message).not.toBe("");
  });

  it("says how long to wait, and where the server is", () => {
    expect(failureOf("create", refusal("rate_limited", "slow down", 429, null, 42_000), host).message).toContain("0:42");
    expect(failureOf("create", new ApiError("network", "unreachable"), host).message).toContain(host);
  });

  it("says the venue is ready when only the prefix failed", () => {
    expect(failureOf("prefix", refusal("invalid_setting", "Use capitals and digits.", 422), host).message).toMatch(/venue is ready.*Use capitals and digits\./);
  });
});

describe("currentZoneName", () => {
  it("gives a renamed zone its present name and leaves the rest alone", () => {
    expect([currentZoneName("Asia/Calcutta"), currentZoneName("Europe/London"), currentZoneName("")]).toEqual(["Asia/Kolkata", "Europe/London", ""]);
  });
});

describe("zoneOptions", () => {
  const at = new Date("2026-01-15T12:00:00Z");

  it("names a zone by its city, its time's name and its offset, the IANA name kept as the value (SET-8)", () => {
    const [kolkata] = zoneOptions(["Asia/Kolkata"], at);
    expect(kolkata?.value).toBe("Asia/Kolkata");
    expect(kolkata?.label).toMatch(/^Kolkata · .+ \(GMT\+5:30\)$/);
    expect(zoneOptions(["America/Argentina/Buenos_Aires"], at)[0]?.label).toMatch(/^Buenos Aires · /);
  });

  it("lists by city, so typing a city's first letters in the list finds it", () => {
    expect(zoneOptions(["Europe/London", "Asia/Kolkata", "America/New_York"], at).map((o) => o.value)).toEqual([
      "Asia/Kolkata",
      "Europe/London",
      "America/New_York",
    ]);
  });

  it("names a zone the browser does not know by its city alone", () => {
    expect(zoneOptions(["Not/A_Zone"], at)).toEqual([{ value: "Not/A_Zone", label: "A Zone" }]);
  });
});

describe("timezoneList", () => {
  it("holds the browser's own zone, even one the list spells another way", () => {
    const zones = timezoneList("Asia/Calcutta");
    if (zones.length > 0) expect(zones).toContain("Asia/Calcutta");
    expect(timezoneList("")).not.toContain("");
  });
});

describe("the steps after the venue is made", () => {
  it("are the printer, mez Cloud and more devices, in that order, and the last leaves", () => {
    expect(allSteps.map((s) => s.name)).toEqual(["Your venue", "Taxes and invoices", "You", "Printer", "mez Cloud", "More devices"]);
    expect(nextAfter("printer")).toBe("cloud");
    expect(nextAfter("cloud")).toBe("ready");
    expect(nextAfter("ready")).toBeNull();
  });

  it("are numbered after the questions", () => {
    expect(indexOfAfter("printer")).toBe(3);
    expect(indexOfAfter("cloud")).toBe(4);
    expect(indexOfAfter("ready")).toBe(5);
  });
});
