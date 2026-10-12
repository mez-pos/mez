// The wizard's mez Cloud step (setup-wizard.md, ADR-322): an owner who has a
// mez Cloud account types the code from it, and the venue is paired before
// its first sale. It comes once the venue exists and the owner is signed in,
// because pairing is the owner's to do (ADR-320), and it can be skipped: the
// till needs none of it.

import { ArrowRightIcon } from "lucide-react";
import { useState, type FormEvent } from "react";
import type { ApiClient } from "../../api";
import type { CloudPairResponse } from "../../api-types";
import { backOfficeAddress, cleanCode, codeRefusal, pairRefusal, pairToCloud, pairedLines } from "../../cloud-pair";
import { Button } from "../../ui";
import { TextField } from "./fields";

export function CloudStep({ api, onDone }: { api: ApiClient; onDone: () => void }) {
  const [typed, setTyped] = useState("");
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [paired, setPaired] = useState<CloudPairResponse | null>(null);

  const connect = async (event: FormEvent) => {
    event.preventDefault();
    if (busy) return;
    const refused = codeRefusal(typed);
    const code = cleanCode(typed);
    if (refused !== null || code === null) {
      setProblem(refused);
      return;
    }
    setBusy(true);
    setProblem(null);
    try {
      setPaired(await pairToCloud(api, code));
    } catch (err) {
      setProblem(pairRefusal(err));
    } finally {
      setBusy(false);
    }
  };

  if (paired) {
    return (
      <div className="flex flex-1 flex-col">
        <h2 className="font-serif text-xl">Connected to mez Cloud</h2>
        <div role="status" className="mt-5 flex flex-col gap-3" data-testid="cloud-paired">
          {pairedLines(paired).map((line) => (
            <p key={line} className="text-base">
              {line}
            </p>
          ))}
          <p className="text-base text-fg-2">From a phone or a computer elsewhere, the back office opens at:</p>
          <p className="font-mono text-lg font-semibold break-all">{backOfficeAddress(paired.hostname)}</p>
        </div>
        <div className="mt-auto flex flex-wrap items-center justify-end gap-3 pt-6">
          <Button variant="primary" className="min-w-44" onClick={onDone}>
            Continue
            <ArrowRightIcon aria-hidden="true" size={18} strokeWidth={2.2} />
          </Button>
        </div>
      </div>
    );
  }

  return (
    <form onSubmit={(event) => void connect(event)} noValidate className="flex flex-1 flex-col">
      <h2 className="font-serif text-xl">Connect to mez Cloud</h2>
      <p className="mt-1 text-sm text-fg-muted">
        If you have a mez Cloud account, a code from it joins this venue to it: the back office opens from anywhere, guests can order from their phones, and a
        backup is kept away from this computer. The till works the same without it, and you can do this any time later, in the back office under Devices.
      </p>
      <div className="mt-5 max-w-md">
        <TextField
          label="Pairing code"
          value={typed}
          onChange={(v) => {
            setTyped(v);
            setProblem(null);
          }}
          error={problem ?? undefined}
          hint={
            busy
              ? "Connecting, and starting the backup. This can take up to twenty seconds."
              : "From your mez Cloud account, under this venue. It works once, for an hour."
          }
          // Eight characters, and room for the spaces or the hyphen a person types between them.
          maxLength={16}
          autoCapitalize="characters"
          className="font-mono tracking-widest uppercase"
        />
      </div>
      <div className="mt-auto flex flex-wrap items-center justify-end gap-3 pt-6">
        <Button disabled={busy} onClick={onDone}>
          Skip for now
        </Button>
        <Button type="submit" variant="primary" className="min-w-44" disabled={busy}>
          {busy ? "Connecting" : "Connect"}
        </Button>
      </div>
    </form>
  );
}
