// First run: redeem a pairing code for a device token (docs/phases/phase-05.md,
// Task 6). The same screen pairs a terminal on ink and a back office on paper.

import { useState, type FormEvent } from "react";
import { formatPairingCode, isCompletePairingCode } from "../auth/format";
import { pairingError } from "../auth/messages";
import type { AppKind } from "../auth/storage";
import { useAuth, useAuthStore } from "../auth/use-auth";
import { Button, Meta } from "../ui";

const appNames: Record<AppKind, string> = { terminal: "terminal", backoffice: "back office", kitchen: "kitchen display" };

const notices = {
  revoked: "This device was unpaired from the back office. Pair it again to keep using it.",
  unrecognised: "The server no longer recognises this device. Pair it again to keep using it.",
} as const;

export function PairScreen({ app }: { app: AppKind }) {
  const status = useAuth();
  const store = useAuthStore();
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const host = window.location.host;
  const notice = status.kind === "unpaired" && status.notice ? notices[status.notice] : null;
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
          <Meta items={["Nofiltr", appNames[app], "Pairing"]} />
          <h1 className="mt-3 font-serif text-total">Pair this {appNames[app]}</h1>
          <p className="mt-3 text-base text-fg-2">
            Enter the eight-character code from <span className="font-mono">nofiltr setup</span>, or ask a manager for one. A code lasts 10
            minutes and works once.
          </p>
        </div>

        {notice && (
          <p role="status" className="rounded border border-chip-warning-border bg-chip-warning-bg px-4 py-3 text-base text-chip-warning-fg">
            {notice}
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
            className="h-14 rounded border border-line-strong bg-raised-high px-4 text-center font-mono text-xl tracking-widest text-fg uppercase placeholder:text-fg-faint outline-offset-2 outline-line-focus focus-visible:outline-2"
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
