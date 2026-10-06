// The wizard's last step (setup-wizard.md, ADR-269): where a second device
// opens mez, and a code to pair it, both in type large enough to read from
// across a counter. The owner is signed in, so the code is the one the back
// office's Devices screen would make.

import { ArrowRightIcon } from "lucide-react";
import { useEffect, useState } from "react";
import type { ApiClient } from "../../api";
import type { PairingCodeResponse } from "../../api-types";
import { Button } from "../../ui";
import { CardChoice } from "./fields";
import { codeError, makeCode, openAt, secondDevice, secondDevices, serverAddresses, typed, untilWords } from "./ready-logic";

export function ReadyStep({ api, onDone }: { api: ApiClient; onDone: () => void }) {
  const [urls, setUrls] = useState<readonly string[] | null>(null);
  const [kind, setKind] = useState<string>(secondDevices[0]?.kind ?? "terminal");
  const [code, setCode] = useState<PairingCodeResponse | null>(null);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const device = secondDevice(kind);

  useEffect(() => {
    let live = true;
    void serverAddresses(api).then((found) => {
      if (live) setUrls(found);
    });
    return () => {
      live = false;
    };
  }, [api]);

  const choose = (next: string) => {
    setKind(next);
    // A code pairs the kind it was made for: the one on screen is not for this.
    setCode(null);
    setProblem(null);
  };

  const show = async () => {
    setBusy(true);
    setProblem(null);
    try {
      setCode(await makeCode(api, device));
    } catch (err) {
      setProblem(codeError(err, window.location.host));
    } finally {
      setBusy(false);
    }
  };

  const [first, ...others] = urls ?? [];

  return (
    <div className="flex flex-1 flex-col">
      <h2 className="font-serif text-xl">Add another device</h2>
      <p className="mt-1 text-sm text-fg-muted">
        This computer is ready to sell. To add a tablet, a kitchen screen or a laptop now, do these three things on it. You can also do it any time later.
      </p>

      <div className="mt-5 flex flex-col gap-6">
        <CardChoice
          label="1. What is it?"
          value={kind}
          onChange={choose}
          options={secondDevices.map((d) => ({ value: d.kind, label: d.label, blurb: d.blurb }))}
          min="14rem"
        />

        <div className="flex flex-col gap-1.5">
          <p className="text-base font-semibold">2. On that device, connect to this network and open</p>
          {urls === null && <p className="text-base text-fg-2">Finding this computer&rsquo;s address</p>}
          {urls !== null && !first && (
            <p className="rounded border border-line bg-sunken px-4 py-3 text-base text-fg-2">
              mez could not tell its own address on this network. On this computer, open Windows&rsquo; network settings to find its IP address, and open that
              address on the other device, followed by :{window.location.port || "80"}
              {device.path}.
            </p>
          )}
          {first && (
            <>
              <p className="break-all rounded-card bg-sunken px-5 py-4 font-mono text-total font-semibold tabular-nums text-fg-heading md:text-display">
                {typed(openAt(first, device))}
              </p>
              {others.length > 0 && (
                <p className="text-sm text-fg-muted">
                  If that does not open, try <span className="font-mono tabular-nums">{others.map((u) => typed(openAt(u, device))).join(" or ")}</span>.
                </p>
              )}
              <p className="text-sm text-fg-muted">
                Both must be on the same network. If it still does not open, Windows may be treating this network as Public: set it to Private in Windows&rsquo;
                network settings.
              </p>
            </>
          )}
        </div>

        <div className="flex flex-col gap-1.5">
          <p className="text-base font-semibold">3. Enter this code there</p>
          {code ? (
            <>
              <p
                aria-label={`Pairing code ${code.code.split("").join(" ")}`}
                className="rounded-card bg-sunken px-5 py-4 font-mono text-total font-semibold tracking-[.12em] text-fg-heading md:text-display"
              >
                {code.code}
              </p>
              <p role="status" className="text-sm text-fg-muted">
                For one {device.label.toLowerCase()}. It works once, {untilWords(code.expires_at)}. More codes are made in the back office, under Devices.
              </p>
            </>
          ) : (
            <div className="flex flex-wrap items-center gap-3">
              <Button variant="attention" disabled={busy} onClick={() => void show()}>
                {busy ? "Making a code" : "Show a pairing code"}
              </Button>
              <p className="text-sm text-fg-muted">A code lasts 10 minutes, so make it when the device is in your hand.</p>
            </div>
          )}
          {problem && (
            <p role="alert" className="text-base text-danger">
              {problem}
            </p>
          )}
        </div>
      </div>

      <div className="mt-auto flex flex-wrap items-center gap-3 pt-6">
        {code && (
          <Button disabled={busy} onClick={() => void show()}>
            Another code
          </Button>
        )}
        <Button variant="primary" className="ml-auto min-w-44" onClick={onDone}>
          Start selling
          <ArrowRightIcon aria-hidden="true" size={18} strokeWidth={2.2} />
        </Button>
      </div>
    </div>
  );
}
