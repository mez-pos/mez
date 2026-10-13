import { describe, expect, it } from "vitest";
import { ApiError, type ApiClient, type RequestOptions } from "../../api";
import type { RecoverOutcomeResponse, RecoverVenue } from "../../api-types";
import {
  alreadySetUp,
  askOutcome,
  awaitVenue,
  canBringBack,
  cleanOtp,
  cleanPhone,
  cloudLines,
  lastBackedUp,
  offeredCountryCode,
  onlyChoice,
  otpRefusal,
  pairThisBrowser,
  pairTries,
  phoneRefusal,
  photosLine,
  recoverClaimHeader,
  recoverRefusal,
  recoverStepIndex,
  recoverSteps,
  refusalField,
  restoreTimeoutMs,
  restoreVenue,
  signedOut,
  startRecovery,
  stillPairing,
  venuesLead,
  verifyRecovery,
  waiting,
  type PairDeps,
  type WaitDeps,
} from "./recover-logic";
import { setupTokenHeader } from "./setup-token";

const backedUp: RecoverVenue = { id: "ven_A1B2C3D4", name: "Corner Cafe", last_backup_at: "2026-10-12T16:02:30Z" };
const never: RecoverVenue = { id: "ven_E5F6G7H8", name: "Corner Cafe, Indiranagar" };

interface Sent {
  path: string;
  body: unknown;
  options: RequestOptions | undefined;
}

/** An API that answers what it is told to and remembers what it was sent. */
function fakeApi(answer: unknown): { api: ApiClient; sent: Sent[] } {
  const sent: Sent[] = [];
  const refuse = () => Promise.reject(new Error("not used"));
  const api: ApiClient = {
    get: refuse,
    put: refuse,
    del: refuse,
    post: <T>(path: string, body?: unknown, options?: RequestOptions) => {
      sent.push({ path, body, options });
      return Promise.resolve(answer as T);
    },
  };
  return { api, sent };
}

describe("the steps", () => {
  it("are four, and each knows its place", () => {
    expect(recoverSteps.map((s) => s.key)).toEqual(["phone", "code", "venue", "back"]);
    expect(recoverStepIndex("phone")).toBe(0);
    expect(recoverStepIndex("back")).toBe(3);
  });
});

describe("the phone number", () => {
  it("is sent as a plus and its digits, however it was typed", () => {
    expect(cleanPhone("+919876543210")).toBe("+919876543210");
    expect(cleanPhone("+91 98765 43210")).toBe("+919876543210");
    expect(cleanPhone(" +91-98765-43210 ")).toBe("+919876543210");
    expect(cleanPhone("+1 (415) 555.0134")).toBe("+14155550134");
  });

  it("is not one without its country code, or with anything but digits", () => {
    for (const typed of [
      "",
      "+",
      "9876543210",
      "919876543210",
      "0091 98765 43210",
      "+0919876543210",
      "+91 98765",
      "+1234567890123456",
      "+91 98765 4321O",
      "++919876543210",
    ]) {
      expect(cleanPhone(typed), typed).toBeNull();
    }
  });

  it("is refused in words that say what to type, before anything is sent", () => {
    expect(phoneRefusal("")).toMatch(/Type the phone number/);
    // The country code the field offers is not a number typed.
    expect(phoneRefusal(offeredCountryCode)).toMatch(/Type the phone number/);
    expect(phoneRefusal("98765 43210")).toMatch(/Start with \+ and the country code/);
    expect(phoneRefusal("+91 98765")).toMatch(/does not look like a phone number/);
    expect(phoneRefusal("+91 98765 43210")).toBeNull();
  });

  it("is offered India's code, which the server would otherwise refuse the number for want of", () => {
    expect(cleanPhone(`${offeredCountryCode}98765 43210`)).toBe("+919876543210");
  });
});

describe("the one-time code", () => {
  it("is six digits, with the spaces a messaging app shows taken away", () => {
    expect(cleanOtp("482913")).toBe("482913");
    expect(cleanOtp("482 913")).toBe("482913");
    expect(cleanOtp(" 482-913 ")).toBe("482913");
    expect(cleanOtp("000000")).toBe("000000");
    for (const typed of ["", "48291", "4829134", "48291O", "482.913"]) expect(cleanOtp(typed), typed).toBeNull();
  });

  it("is refused in words before anything is sent", () => {
    expect(otpRefusal(" ")).toMatch(/Type the code/);
    expect(otpRefusal("48291")).toMatch(/6 digits/);
    expect(otpRefusal("482 913")).toBeNull();
  });
});

describe("the account's venues", () => {
  it("can be brought back only where there is a backup", () => {
    expect(canBringBack(backedUp)).toBe(true);
    expect(canBringBack(never)).toBe(false);
  });

  it("start on the only one there is to choose, and on none where there is a choice", () => {
    expect(onlyChoice([backedUp])).toBe(backedUp.id);
    expect(onlyChoice([never, backedUp])).toBe(backedUp.id);
    expect(onlyChoice([backedUp, { ...backedUp, id: "ven_OTHER001" }])).toBeNull();
    expect(onlyChoice([never])).toBeNull();
    expect(onlyChoice([])).toBeNull();
  });

  it("each say when they were last backed up, in the time this computer keeps", () => {
    expect(lastBackedUp(backedUp, "en-GB", "Asia/Kolkata")).toBe("Last backed up 12 Oct 2026, 21:32");
    expect(lastBackedUp(backedUp, "en-GB", "UTC")).toBe("Last backed up 12 Oct 2026, 16:02");
    expect(lastBackedUp(never)).toMatch(/No backup with mez Cloud/);
    expect(lastBackedUp({ ...backedUp, last_backup_at: "nonsense" })).toBe("Backed up with mez Cloud.");
  });

  it("are led in by what the account has", () => {
    expect(venuesLead("Priya", [backedUp])).toMatch(/^Priya, this is the venue on your account/);
    expect(venuesLead("Priya", [backedUp, never])).toMatch(/choose the venue this computer is to be/);
    expect(venuesLead("Priya", [never])).toMatch(/none of this account's venues has a backup/);
    expect(venuesLead("Priya", [])).toMatch(/has no venues/);
    // Both dead ends say the way on.
    for (const venues of [[never], []]) expect(venuesLead("Priya", venues)).toMatch(/Set this computer up as a new venue instead/);
  });
});

describe("what is asked of the server", () => {
  it("is asked with the setup token before mez starts again", async () => {
    const { api, sent } = fakeApi({ attempt: "att_1" });
    const deps = { api, token: "0123456789abcdef0123456789abcdef" };
    await startRecovery(deps, "+919876543210");
    await verifyRecovery(deps, "att_1", "482913");
    await restoreVenue(deps, backedUp.id);
    expect(sent.map((s) => s.path)).toEqual(["/api/v1/setup/recover/start", "/api/v1/setup/recover/verify", "/api/v1/setup/recover/restore"]);
    expect(sent.map((s) => s.body)).toEqual([{ phone: "+919876543210" }, { attempt: "att_1", code: "482913" }, { venue_id: backedUp.id }]);
    for (const s of sent) expect(s.options?.headers).toEqual({ [setupTokenHeader]: deps.token });
  });

  it("gives a restore longer than the server gives itself, and nothing else that long", async () => {
    const { api, sent } = fakeApi({});
    await startRecovery({ api, token: null }, "+919876543210");
    await restoreVenue({ api, token: null }, backedUp.id);
    expect(sent[0]?.options?.timeoutMs).toBeUndefined();
    expect(sent[1]?.options?.timeoutMs).toBe(restoreTimeoutMs);
    expect(restoreTimeoutMs).toBeGreaterThan(30 * 60_000);
  });

  it("is asked with the claim after, and names the device only when a code is wanted", async () => {
    const { api, sent } = fakeApi({});
    await askOutcome(api, "the-claim");
    await askOutcome(api, "the-claim", { name: "Front counter", kind: "terminal" });
    expect(sent.map((s) => s.path)).toEqual(["/api/v1/setup/recover/outcome", "/api/v1/setup/recover/outcome"]);
    expect(sent[0]?.body).toEqual({});
    expect(sent[1]?.body).toEqual({ device_name: "Front counter", device_kind: "terminal" });
    for (const s of sent) expect(s.options?.headers).toEqual({ [recoverClaimHeader]: "the-claim" });
  });
});

describe("a refusal", () => {
  const host = "localhost:7777";

  it("is the server's sentence, which is already an owner's", () => {
    const err = new ApiError("recover_code_refused", "mez Cloud does not accept that code.", 409, "code");
    expect(recoverRefusal(err, host)).toBe("mez Cloud does not accept that code.");
    expect(refusalField(err)).toBe("code");
    expect(refusalField(new ApiError("recover_phone_invalid", "Type it with its country code.", 422, "phone"))).toBe("phone");
  });

  it("belongs to the step where it names no field of this page's", () => {
    expect(refusalField(new ApiError("recover_no_backup", "Nothing to bring back.", 409))).toBeNull();
    expect(refusalField(new ApiError("invalid_request", "bad", 400, "device_kind"))).toBeNull();
    expect(refusalField(new Error("boom"))).toBeNull();
  });

  it("has words of its own where the server said nothing, or nothing an owner can use", () => {
    expect(recoverRefusal(new ApiError("network", "fetch failed"), host)).toContain(host);
    expect(recoverRefusal(new ApiError("timeout", "the server did not answer in time"), host)).toMatch(/Nothing on this computer was changed/);
    expect(recoverRefusal(new ApiError("rate_limited", "too many requests; wait and try again", 429, null, 42_000), host)).toMatch(/Wait .*and try again/);
    expect(recoverRefusal(new ApiError("rate_limited", "mez Cloud is busy.", 429), host)).toMatch(/Wait a minute/);
    expect(recoverRefusal(new ApiError("setup_locked", "Open mez from the installer", 403), host)).toMatch(/can no longer set this computer up/);
    expect(recoverRefusal(new Error("boom"), host)).toBe("Something went wrong. Try again.");
  });

  it("is told apart where the page must go somewhere else", () => {
    expect(signedOut(new ApiError("recover_signed_out", "Ask for a new code.", 409))).toBe(true);
    expect(signedOut(new ApiError("recover_code_refused", "No.", 409))).toBe(false);
    expect(alreadySetUp(new ApiError("already_set_up", "This computer already has a venue.", 409))).toBe(true);
    expect(alreadySetUp(new Error("boom"))).toBe(false);
  });
});

/** A clock that only moves when the page sleeps, and a server that answers from a script. */
function waitWorld(script: readonly (RecoverOutcomeResponse | ApiError | Error)[]): { deps: WaitDeps; asked: () => number; slept: number[] } {
  let now = 0;
  let asked = 0;
  const slept: number[] = [];
  const deps: WaitDeps = {
    now: () => now,
    sleep: (ms) => {
      slept.push(ms);
      now += ms;
      return Promise.resolve();
    },
    ask: () => {
      const next = script[Math.min(asked, script.length - 1)];
      asked += 1;
      if (next === undefined) return Promise.reject(new Error("an empty script"));
      return next instanceof Error ? Promise.reject(next) : Promise.resolve(next);
    },
  };
  return { deps, asked: () => asked, slept };
}

const back: RecoverOutcomeResponse = { venue_id: "ven_A1B2C3D4", venue_name: "Corner Cafe", cloud: "pairing" };
const nothing = new ApiError("recover_nothing", "nothing to tell", 404);
const down = new ApiError("network", "fetch failed");

describe("waiting for mez to start again", () => {
  it("waits through the start that staged the venue, which has nothing to tell, and through nothing answering at all", async () => {
    const w = waitWorld([nothing, down, down, back]);
    expect(await awaitVenue(w.deps)).toEqual({ kind: "back", outcome: back });
    expect(w.asked()).toBe(4);
    expect(w.slept).toEqual([waiting.everyMs, waiting.everyMs, waiting.everyMs]);
  });

  it("is told at once when the venue is already back", async () => {
    const w = waitWorld([back]);
    expect(await awaitVenue(w.deps)).toEqual({ kind: "back", outcome: back });
    expect(w.slept).toEqual([]);
  });

  it("takes nothing to tell for an answer once the start has had its time", async () => {
    const w = waitWorld([nothing]);
    expect(await awaitVenue(w.deps)).toEqual({ kind: "untold" });
    // Not before: a second after the restore, the old start is still what answers.
    expect(w.slept.reduce((a, b) => a + b, 0)).toBeGreaterThanOrEqual(waiting.graceMs);
    expect(w.slept.reduce((a, b) => a + b, 0)).toBeLessThan(waiting.graceMs + 2 * waiting.everyMs);
  });

  it("gives up on a server that never answers, and says that and not that there is nothing to tell", async () => {
    const w = waitWorld([down]);
    expect(await awaitVenue(w.deps)).toEqual({ kind: "silent" });
    expect(w.slept.reduce((a, b) => a + b, 0)).toBeGreaterThanOrEqual(waiting.giveUpMs);
  });

  it("asks no faster than the server allows, and slower when it says so", async () => {
    const hurried = new ApiError("rate_limited", "too many requests", 429, null, 20_000);
    const w = waitWorld([hurried, new ApiError("rate_limited", "too many requests", 429, null, 200), back]);
    expect(await awaitVenue(w.deps)).toEqual({ kind: "back", outcome: back });
    expect(w.slept).toEqual([20_000, waiting.everyMs]);
  });

  it("never waits past the moment it would give up, whatever the server says", async () => {
    const w = waitWorld([new ApiError("rate_limited", "too many requests", 429, null, 10 * 60_000), down]);
    expect(await awaitVenue(w.deps)).toEqual({ kind: "silent" });
    expect(w.slept[0]).toBe(waiting.giveUpMs);
    expect(w.slept.reduce((a, b) => a + b, 0)).toBeLessThan(waiting.giveUpMs + 2 * waiting.everyMs);
  });

  it("waits through whatever else goes wrong while the server comes up", async () => {
    const w = waitWorld([new ApiError("bad_response", "not JSON", 502), new Error("boom"), back]);
    expect(await awaitVenue(w.deps)).toEqual({ kind: "back", outcome: back });
  });
});

describe("pairing the browser the owner is at", () => {
  const named: RecoverOutcomeResponse = { ...back, cloud: "paired", hostname: "corner-cafe.mezpos.com", pairing_code: "7K3M-9QX2" };

  function pairWorld(script: readonly (RecoverOutcomeResponse | Error)[], pairFails = false) {
    let asked = 0;
    const codes: string[] = [];
    const slept: number[] = [];
    const deps: PairDeps = {
      askNamed: () => {
        const next = script[Math.min(asked, script.length - 1)];
        asked += 1;
        if (next === undefined) return Promise.reject(new Error("an empty script"));
        return next instanceof Error ? Promise.reject(next) : Promise.resolve(next);
      },
      pair: (code) => {
        codes.push(code);
        return pairFails ? Promise.reject(new Error("wrong kind of device")) : Promise.resolve();
      },
      sleep: (ms) => {
        slept.push(ms);
        return Promise.resolve();
      },
    };
    return { deps, asked: () => asked, codes, slept };
  }

  it("pairs with the code the answer carries, and keeps that answer, which is the newer", async () => {
    const w = pairWorld([named]);
    expect(await pairThisBrowser(w.deps)).toEqual({ paired: true, outcome: named });
    expect(w.codes).toEqual(["7K3M-9QX2"]);
    expect(w.slept).toEqual([]);
  });

  it("asks again after a slip: the claim is still good and the code is the same one", async () => {
    const w = pairWorld([down, new ApiError("rate_limited", "too many requests", 429), named]);
    expect(await pairThisBrowser(w.deps)).toEqual({ paired: true, outcome: named });
    expect(w.asked()).toBe(pairTries);
    expect(w.slept).toEqual([waiting.everyMs, waiting.everyMs]);
  });

  it("gives up after three, and says the browser is not paired", async () => {
    const w = pairWorld([down]);
    expect(await pairThisBrowser(w.deps)).toEqual({ paired: false, outcome: null });
    expect(w.asked()).toBe(pairTries);
    expect(w.codes).toEqual([]);
  });

  it("does not ask again once the claim is no longer one", async () => {
    const w = pairWorld([nothing]);
    expect(await pairThisBrowser(w.deps)).toEqual({ paired: false, outcome: null });
    expect(w.asked()).toBe(1);
  });

  it("asks again for an answer that carried no code", async () => {
    const w = pairWorld([back, named]);
    expect((await pairThisBrowser(w.deps)).paired).toBe(true);
    expect(w.asked()).toBe(2);
  });

  it("does not try a code twice that would not pair", async () => {
    const w = pairWorld([named], true);
    expect(await pairThisBrowser(w.deps)).toEqual({ paired: false, outcome: named });
    expect(w.codes).toEqual(["7K3M-9QX2"]);
    expect(w.asked()).toBe(1);
  });
});

describe("the last card", () => {
  it("says what became of the pairing, and where the back office opens once it is paired", () => {
    expect(stillPairing("pairing")).toBe(true);
    for (const state of ["paired", "not_paired", "none"] as const) expect(stillPairing(state)).toBe(false);

    expect(cloudLines({ cloud: "pairing" })).toEqual(["Pairing it to your mez Cloud account again."]);
    expect(cloudLines({ cloud: "pairing" }, true)[0]).toMatch(/still being paired/);
    expect(cloudLines({ cloud: "paired", hostname: "corner-cafe.mezpos.com" })).toEqual([
      "It is paired to your mez Cloud account again. From a phone or a computer elsewhere, the back office opens at:",
      "https://corner-cafe.mezpos.com/backoffice/",
    ]);
    expect(cloudLines({ cloud: "paired" })).toEqual(["It is paired to your mez Cloud account again."]);
    expect(cloudLines({ cloud: "not_paired" })[0]).toMatch(/The till works all the same.*under Devices/);
    expect(cloudLines({ cloud: "none" })).toEqual([]);
  });

  it("speaks of the photos only when some did not come", () => {
    expect(photosLine({ photos: 12, photos_incomplete: false })).toBeNull();
    expect(photosLine({ photos: 0, photos_incomplete: false })).toBeNull();
    expect(photosLine({ photos: 9, photos_incomplete: true })).toMatch(/Not all of the menu's photos/);
    expect(photosLine({ photos: 0, photos_incomplete: true })).toMatch(/could not be brought back/);
  });
});
