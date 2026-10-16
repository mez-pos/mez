// First run: redeem a pairing code for a device token (docs/phases/phase-05.md,
// Task 6). The same screen pairs a terminal on ink and a back office on paper.
//
// A venue that is not made yet has no codes to redeem. Then the terminal on
// the server's own machine goes to the setup wizard, and anything else is told
// where the wizard is (ADR-267).

import { useState, type FormEvent } from "react";
import { Redirect } from "wouter";
import { formatPairingCode, isCompletePairingCode } from "../auth/format";
import type { AuthStatus } from "../auth/auth-store";
import { pairingError } from "../auth/messages";
import type { AppKind } from "../auth/storage";
import { useAuth, useAuthStore } from "../auth/use-auth";
import { Button, Logo, Meta } from "../ui";
import { setupPath } from "./setup/setup-path";
import { useSetupStatus } from "./setup/use-setup-status";
import { Splash } from "./Splash";

const appNames: Record<AppKind, string> = { terminal: "terminal", backoffice: "back office", kitchen: "kitchen display" };

type Notice = NonNullable<Extract<AuthStatus, { kind: "unpaired" }>["notice"]>;

const notices: Record<Notice, string> = {
  revoked: "This device was unpaired from the back office. Pair it again to keep using it.",
  unrecognised: "The server no longer recognises this device. Pair it again to keep using it.",
  // A session mez Cloud handed this tab (ADR-355): opened again from there,
  // not by pairing, which is for the venue's own devices.
  cloud_ended: "The back office you opened from mez Cloud has closed. Open it again from mez Cloud.",
  cloud_signed_out: "You signed out of the back office you opened from mez Cloud.",
};

export function PairScreen({ app }: { app: AppKind }) {
  const setup = useSetupStatus();
  // The first answer is waited for, so the wizard's own machine never sees
  // this form flash by. A server that does not answer is not waited on: the
  // form says it cannot be reached when a code is tried.
  if (setup.isPending && setup.failureCount === 0) return <Splash />;
  if (setup.data?.needed) {
    // Only the terminal has the wizard among its routes; the others link to it.
    if (setup.data.here && app === "terminal") return <Redirect to={setupPath} replace />;
    return <NotSetUp here={setup.data.here} onCheck={() => void setup.refetch()} />;
  }
  return <PairForm app={app} />;
}

/** What a device sees while the venue has not been made. */
function NotSetUp({ here, onCheck }: { here: boolean; onCheck: () => void }) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-canvas p-6 text-fg">
      <div className="flex w-full max-w-md flex-col gap-6">
        <div>
          <Logo className="mb-6 h-12" />
          <Meta items={["First run"]} />
          <h1 className="mt-3 font-serif text-total">This venue is not set up yet</h1>
          <p className="mt-3 text-base text-fg-2">
            {here
              ? "It takes a few minutes, and it is done once, here."
              : "Setting up is done on the computer mez is installed on. Open mez there, from its shortcut or at localhost in a browser, then pair this device with a code from it."}
          </p>
        </div>
        {here ? (
          // A plain link: the wizard is the terminal's, outside this app's own routes.
          <a
            href={setupPath}
            className="inline-flex min-h-tap items-center justify-center rounded-pill bg-accent px-5 text-base font-semibold text-accent-fg shadow-action outline-offset-2 outline-attention hover:bg-accent-hover focus-visible:outline-3"
          >
            Set up this venue
          </a>
        ) : (
          <Button onClick={onCheck} className="w-full">
            Check again
          </Button>
        )}
        <Meta items={["Server", window.location.host]} />
      </div>
    </main>
  );
}

function PairForm({ app }: { app: AppKind }) {
  const status = useAuth();
  const store = useAuthStore();
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const host = window.location.host;
  const notice = status.kind === "unpaired" && status.notice ? notices[status.notice] : null;
  const back = status.kind === "unpaired" ? (status.back ?? null) : null;
  const ready = isCompletePairingCode(code) && !busy;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!ready) return;
    setBusy(true);
    setError(null);
    try {
      await store.pair(code);
    } catch (err) {
      setError(pairingError(err, host));
      setBusy(false);
    }
  };

  return (
    <main className="flex min-h-screen items-center justify-center bg-canvas p-6 text-fg">
      <form onSubmit={submit} className="flex w-full max-w-md flex-col gap-6" noValidate>
        <div>
          <Logo className="mb-6 h-12" />
          <Meta items={[appNames[app], "Pairing"]} />
          <h1 className="mt-3 font-serif text-total">Pair this {appNames[app]}</h1>
          <p className="mt-3 text-base text-fg-2">
            Enter the eight-character code a manager gives you. They make one in the back office, under Devices. A code lasts 10 minutes and works once.
          </p>
        </div>

        {notice && (
          <p role="status" className="rounded border border-chip-warning-border bg-chip-warning-bg px-4 py-3 text-base text-chip-warning-fg">
            {notice}
            {back && (
              <>
                {" "}
                <a href={back} className="font-semibold underline underline-offset-2">
                  Back to mez Cloud
                </a>
              </>
            )}
          </p>
        )}

        <label className="flex flex-col gap-2">
          <span className="text-base font-semibold">Pairing code</span>
          <input
            value={code}
            onChange={(event) => {
              setCode(formatPairingCode(event.target.value));
              setError(null);
            }}
            placeholder="XXXX-XXXX"
            autoComplete="one-time-code"
            autoCapitalize="characters"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="go"
            aria-invalid={error !== null}
            aria-describedby={error ? "pair-error" : undefined}
            className="h-14 rounded border border-line-strong bg-raised px-4 text-center font-mono text-xl tracking-widest text-fg uppercase placeholder:text-fg-faint outline-accent-tint focus-visible:border-accent focus-visible:outline-3"
            autoFocus
          />
        </label>

        {error && (
          <p id="pair-error" role="alert" className="text-base text-danger">
            {error}
          </p>
        )}

        <Button type="submit" variant="primary" disabled={!ready} className="w-full">
          {busy ? "Pairing\u2026" : "Pair"}
        </Button>

        <Meta items={["Server", host]} />
      </form>
    </main>
  );
}
