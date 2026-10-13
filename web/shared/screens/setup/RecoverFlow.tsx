// "I had mez before" (setup-wizard.md, ADR-336): the wizard's other way. An
// owner whose computer is gone signs in to their mez Cloud account with the
// phone it is under, chooses their venue, and it is brought back from its
// backup onto this one. mez then starts again by itself, and this page waits
// through that and pairs the browser it is in (ADR-333 to ADR-335).
//
// The words and the waiting are recover-logic.ts's. This draws them.

import { ArrowRightIcon } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import type { ApiClient } from "../../api";
import type { RecoverOutcomeResponse, RecoverRestoreResponse, RecoverVerifyResponse } from "../../api-types";
import { Button } from "../../ui";
import { cx } from "../../ui/cx";
import { TextField } from "./fields";
import {
  alreadySetUp,
  askOutcome,
  awaitVenue,
  canBringBack,
  cleanOtp,
  cleanPhone,
  cloudLines,
  lastBackedUp,
  needsCode,
  offeredCountryCode,
  onlyChoice,
  otpRefusal,
  pairThisBrowser,
  phoneRefusal,
  photosLine,
  recoverRefusal,
  refusalField,
  restoreVenue,
  signedOut,
  silent,
  startRecovery,
  stillPairing,
  untold,
  verifyRecovery,
  venuesLead,
  waiting,
  type RecoverStepKey,
} from "./recover-logic";

export interface RecoverFlowProps {
  api: ApiClient;
  token: string | null;
  /** Pairs this browser with a device's code, as first run does (AuthStore.pair). */
  pair: (code: string) => Promise<void>;
  /** Which step is in hand, for the frame's numbered rounds. */
  onStep: (step: RecoverStepKey) => void;
  /** The venue is staged: from here the server's "no setup needed" is this page's own doing. */
  onBegun: () => void;
  /** Back to setting up a new venue. */
  onNewVenue: () => void;
  /** Out of the wizard, to the till. */
  onLeave: () => void;
}

type Stage =
  | { at: "phone" }
  | { at: "code"; phone: string; attempt: string }
  | { at: "venue"; phone: string; account: RecoverVerifyResponse }
  | { at: "bringing"; name: string }
  | { at: "starting"; restored: RecoverRestoreResponse }
  | { at: "back"; restored: RecoverRestoreResponse; outcome: RecoverOutcomeResponse; paired: boolean }
  | { at: "lost"; title: string; body: string };

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

export function RecoverFlow({ api, token, pair, onStep, onBegun, onNewVenue, onLeave }: RecoverFlowProps) {
  const host = window.location.host;
  const deps = { api, token };
  const [stage, setStage] = useState<Stage>({ at: "phone" });
  const [phone, setPhone] = useState(offeredCountryCode);
  const [otp, setOtp] = useState("");
  const [chosen, setChosen] = useState<string | null>(null);
  const [deviceName, setDeviceName] = useState("Counter 1");
  const [problem, setProblem] = useState<{ field: "phone" | "code" | "device" | null; message: string } | null>(null);
  const [busy, setBusy] = useState(false);
  // The claim is this page's alone and is kept in its memory alone (ADR-334).
  const claim = useRef<string | null>(null);

  const go = (next: Stage) => {
    setProblem(null);
    setStage(next);
    onStep(next.at === "phone" || next.at === "code" || next.at === "venue" ? next.at : "back");
  };
  const refuse = (err: unknown) => setProblem({ field: refusalField(err), message: recoverRefusal(err, host) });

  const sendCode = async (number: string): Promise<boolean> => {
    setBusy(true);
    setProblem(null);
    try {
      const started = await startRecovery(deps, number);
      setOtp("");
      go({ at: "code", phone: number, attempt: started.attempt });
      return true;
    } catch (err) {
      refuse(err);
      return false;
    } finally {
      setBusy(false);
    }
  };

  const submitPhone = (event: FormEvent) => {
    event.preventDefault();
    if (busy) return;
    const refused = phoneRefusal(phone);
    const number = cleanPhone(phone);
    if (refused !== null || number === null) {
      setProblem({ field: "phone", message: refused ?? "Type the phone number of your mez Cloud account." });
      return;
    }
    void sendCode(number);
  };

  const submitCode = async (event: FormEvent) => {
    event.preventDefault();
    if (busy || stage.at !== "code") return;
    const refused = otpRefusal(otp);
    const code = cleanOtp(otp);
    if (refused !== null || code === null) {
      setProblem({ field: "code", message: refused ?? "Type the code that was sent to your phone." });
      return;
    }
    setBusy(true);
    setProblem(null);
    try {
      const account = await verifyRecovery(deps, stage.attempt, code);
      setChosen(onlyChoice(account.venues));
      go({ at: "venue", phone: stage.phone, account });
    } catch (err) {
      refuse(err);
    } finally {
      setBusy(false);
    }
  };

  const bringBack = async (event: FormEvent) => {
    event.preventDefault();
    if (busy || stage.at !== "venue") return;
    const venue = stage.account.venues.find((v) => v.id === chosen && canBringBack(v));
    if (!venue) {
      setProblem({ field: null, message: "Choose the venue to bring back." });
      return;
    }
    const name = deviceName.trim();
    if (name === "") {
      setProblem({ field: "device", message: "Give this till a name." });
      return;
    }
    const from = stage;
    setBusy(true);
    go({ at: "bringing", name: venue.name });
    let restored: RecoverRestoreResponse;
    try {
      restored = await restoreVenue(deps, venue.id);
    } catch (err) {
      setBusy(false);
      if (signedOut(err)) {
        // The sign-in ran out while they chose: back to the number, with why.
        go({ at: "phone" });
        setProblem({ field: null, message: recoverRefusal(err, host) });
        return;
      }
      if (alreadySetUp(err)) {
        go({ at: "lost", title: "Already set up", body: recoverRefusal(err, host) });
        return;
      }
      go(from);
      refuse(err);
      return;
    }
    // Staged. mez is starting again on the venue as this runs.
    claim.current = restored.claim;
    onBegun();
    go({ at: "starting", restored });
    const held = restored.claim;
    const arrival = await awaitVenue({ ask: () => askOutcome(api, held), sleep, now: () => Date.now() });
    if (arrival.kind !== "back") {
      setBusy(false);
      go({ at: "lost", title: arrival.kind === "untold" ? "Open mez again" : "Can't reach mez", body: arrival.kind === "untold" ? untold : silent(host) });
      return;
    }
    // The venue is in place. This browser is no device of it yet.
    const browser = await pairThisBrowser({ askNamed: () => askOutcome(api, held, { name, kind: "terminal" }), pair, sleep });
    setBusy(false);
    go({ at: "back", restored, outcome: browser.outcome ?? arrival.outcome, paired: browser.paired });
  };

  // Once the venue is back, whether it is paired to its account is asked for a
  // while longer: with no internet it takes some seconds to be given up on.
  const backOutcome = stage.at === "back" ? stage.outcome : null;
  const [gaveUp, setGaveUp] = useState(false);
  useEffect(() => {
    if (!backOutcome || !stillPairing(backOutcome.cloud) || !claim.current) return;
    const held = claim.current;
    let live = true;
    const began = Date.now();
    void (async () => {
      while (live) {
        await sleep(waiting.everyMs);
        if (!live) return;
        if (Date.now() - began > waiting.pairingMs) {
          setGaveUp(true);
          return;
        }
        try {
          const next = await askOutcome(api, held);
          if (!live) return;
          if (!stillPairing(next.cloud)) {
            setStage((s) => (s.at === "back" ? { ...s, outcome: next } : s));
            return;
          }
        } catch {
          // Asked again next time round.
        }
      }
    })();
    return () => {
      live = false;
    };
  }, [api, backOutcome]);

  if (stage.at === "phone") {
    return (
      <form onSubmit={submitPhone} noValidate className="flex flex-1 flex-col">
        <Head title="Bring your venue back">
          If this venue was on another computer and was paired to mez Cloud, it can be brought back here from its backup: the menu, the staff and their PINs,
          and every sale up to the last backup. You need the phone your mez Cloud account is under.
        </Head>
        {problem && problem.field === null && <Alert>{problem.message}</Alert>}
        <div className="mt-5 max-w-md">
          <TextField
            label="Phone number of your mez Cloud account"
            type="tel"
            inputMode="tel"
            autoComplete="tel"
            value={phone}
            onChange={(v) => {
              setPhone(v);
              setProblem(null);
            }}
            error={problem?.field === "phone" ? problem.message : undefined}
            hint={
              busy
                ? "Asking mez Cloud to send a code."
                : "With its country code. A 6-digit code is sent to it on WhatsApp or by SMS, if it has a mez Cloud account."
            }
            className="font-mono"
            maxLength={24}
            autoFocus
          />
        </div>
        <Foot>
          <Button disabled={busy} onClick={onNewVenue}>
            Set up a new venue instead
          </Button>
          <Button type="submit" variant="primary" className="min-w-44" disabled={busy}>
            {busy ? "Sending" : "Send the code"}
            {!busy && <ArrowRightIcon aria-hidden="true" size={18} strokeWidth={2.2} />}
          </Button>
        </Foot>
      </form>
    );
  }

  if (stage.at === "code") {
    return (
      <form onSubmit={(event) => void submitCode(event)} noValidate className="flex flex-1 flex-col">
        <Head title="Enter the code">
          If <span className="font-mono">{stage.phone}</span> has a mez Cloud account, a code is on its way to it, on WhatsApp or by SMS. It can take a minute.
        </Head>
        {problem && problem.field !== "code" && <Alert>{problem.message}</Alert>}
        <div className="mt-5 max-w-xs">
          <TextField
            label="Code from your phone"
            inputMode="numeric"
            autoComplete="one-time-code"
            value={otp}
            onChange={(v) => {
              setOtp(v.replace(/[^\d\s-]/g, ""));
              setProblem(null);
            }}
            error={problem?.field === "code" ? problem.message : undefined}
            hint={busy ? "Checking with mez Cloud." : "6 digits. It works once, for a few minutes."}
            className="font-mono text-lg tracking-widest"
            maxLength={9}
            autoFocus
          />
        </div>
        <Foot>
          <Button disabled={busy} onClick={() => go({ at: "phone" })}>
            Change the number
          </Button>
          <Button disabled={busy} onClick={() => void sendCode(stage.phone)}>
            Send another code
          </Button>
          <Button type="submit" variant="primary" className="min-w-44" disabled={busy}>
            {busy ? "Checking" : "Continue"}
            {!busy && <ArrowRightIcon aria-hidden="true" size={18} strokeWidth={2.2} />}
          </Button>
        </Foot>
      </form>
    );
  }

  if (stage.at === "venue") {
    const { owner, venues } = stage.account;
    const any = venues.some(canBringBack);
    return (
      <form onSubmit={(event) => void bringBack(event)} noValidate className="flex flex-1 flex-col">
        <Head title={any ? "Choose your venue" : "Nothing to bring back"}>{venuesLead(owner, venues)}</Head>
        {problem && problem.field === null && <Alert>{problem.message}</Alert>}
        {venues.length > 0 && (
          <div role="radiogroup" aria-label="Venues on this account" className="mt-5 flex flex-col gap-3">
            {venues.map((venue) => {
              const able = canBringBack(venue);
              const picked = able && venue.id === chosen;
              return (
                <button
                  key={venue.id}
                  type="button"
                  role="radio"
                  aria-checked={picked}
                  disabled={!able}
                  onClick={() => {
                    setChosen(venue.id);
                    setProblem(null);
                  }}
                  className={cx(
                    "flex min-h-tap flex-col items-start gap-1 rounded-card border px-4 py-3 text-left outline-offset-2 outline-attention focus-visible:outline-3",
                    picked ? "border-accent bg-accent-tint" : "border-line-strong bg-raised",
                    able ? "hover:border-accent" : "opacity-70",
                  )}
                >
                  <span className="text-base font-semibold">{venue.name}</span>
                  <span className={cx("text-sm", able ? "text-fg-2" : "text-fg-muted")}>{lastBackedUp(venue)}</span>
                </button>
              );
            })}
          </div>
        )}
        {any && (
          <div className="mt-5 max-w-md">
            <TextField
              label="What this till is called"
              value={deviceName}
              onChange={(v) => {
                setDeviceName(v);
                setProblem(null);
              }}
              error={problem?.field === "device" ? problem.message : undefined}
              hint="This computer's name on reports and in the list of devices. The old computer keeps its own in that list."
              maxLength={40}
            />
          </div>
        )}
        <Foot>
          <Button onClick={onNewVenue}>Set up a new venue instead</Button>
          {any && (
            <Button type="submit" variant="primary" className="min-w-44">
              Bring it back
              <ArrowRightIcon aria-hidden="true" size={18} strokeWidth={2.2} />
            </Button>
          )}
        </Foot>
      </form>
    );
  }

  if (stage.at === "bringing") {
    return (
      <Waiting title={`Bringing ${stage.name} back`} status="Downloading the backup and checking it">
        This can take a few minutes on a slow connection. Leave this page open: nothing on this computer is changed until the whole backup is here and checked.
      </Waiting>
    );
  }

  if (stage.at === "starting") {
    return (
      <Waiting title={`${stage.restored.venue_name} is here`} status="mez is starting again on your venue">
        The backup is down and checked. mez is starting again to put it in place, which takes a few seconds. Leave this page open.
      </Waiting>
    );
  }

  if (stage.at === "lost") {
    return (
      <div className="flex flex-1 flex-col items-start justify-center gap-3 py-6">
        <Arrived>{stage.title}</Arrived>
        <p role="alert" className="max-w-xl text-base text-fg">
          {stage.body}
        </p>
        <div className="mt-3">
          <Button variant="primary" onClick={onLeave}>
            Open mez
          </Button>
        </div>
      </div>
    );
  }

  const photos = photosLine(stage.restored);
  const [cloudSays, address] = cloudLines(stage.outcome, gaveUp);
  return (
    <div className="flex flex-1 flex-col">
      <Arrived>{stage.restored.venue_name} is back</Arrived>
      <div role="status" className="mt-5 flex max-w-2xl flex-col gap-3" data-testid="recover-back">
        <p className="text-base">Its menu, its staff and its sales are on this computer, as they were at its last backup.</p>
        {photos && <p className="text-base">{photos}</p>}
        {cloudSays && <p className="text-base">{cloudSays}</p>}
        {address && <p className="font-mono text-lg font-semibold break-all">{address}</p>}
        <p className="text-base">{stage.paired ? "Sign in with your PIN, as you did before. Look at the last invoices before you sell on it." : needsCode}</p>
      </div>
      <Foot>
        <Button variant="primary" className="min-w-44" onClick={onLeave}>
          {stage.paired ? "Sign in" : "Open mez"}
          <ArrowRightIcon aria-hidden="true" size={18} strokeWidth={2.2} />
        </Button>
      </Foot>
    </div>
  );
}

/**
 * The heading of a card that took the place of what the owner was pressing.
 * It takes the focus as it arrives: the button that was pressed is gone, and
 * a screen reader left on nothing would never say the venue is back.
 */
function Arrived({ children }: { children: ReactNode }) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    heading.current?.focus();
  }, [children]);
  return (
    <h2 ref={heading} tabIndex={-1} className="font-serif text-xl outline-none">
      {children}
    </h2>
  );
}

function Head({ title, children }: { title: string; children: ReactNode }) {
  return (
    <>
      <h2 className="font-serif text-xl">{title}</h2>
      <p className="mt-1 max-w-2xl text-sm text-fg-muted">{children}</p>
    </>
  );
}

function Alert({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="mt-4 max-w-2xl rounded border border-danger-line bg-danger-tint px-4 py-3 text-sm text-danger">
      {children}
    </p>
  );
}

function Foot({ children }: { children: ReactNode }) {
  return <div className="mt-auto flex flex-wrap items-center justify-end gap-3 pt-6">{children}</div>;
}

/** A step that is the server's to do: what is happening, and that the page is to be left open. */
function Waiting({ title, status, children }: { title: string; status: string; children: ReactNode }) {
  return (
    <div className="flex flex-1 flex-col items-start justify-center gap-3 py-6">
      <Arrived>{title}</Arrived>
      <p className="max-w-xl text-base text-fg">{children}</p>
      <p role="status" className="text-sm text-fg-muted">
        {status}
      </p>
    </div>
  );
}
