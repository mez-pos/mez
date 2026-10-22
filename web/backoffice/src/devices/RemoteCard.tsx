// Back office from anywhere (docs/design/screens/backoffice-devices.md):
// whether the back office can be opened away from the venue, the address to
// open when it can, and what to do the first time. It changes nothing: the
// setting is in the venue's settings file and read when mez starts (ADR-306).
// It reads the same answer the card above it does, so the two agree. At its
// foot is mez Cloud: who the venue is paired to, and the code that pairs it
// (ADR-318, ADR-320).

import { useState } from "react";
import { useAway } from "../../../shared/use-realtime";
import { Button, StatusChip } from "../../../shared/ui";
import { Panel } from "../dashboard/Panel";
import { CloudPairModal } from "./CloudPairModal";
import { lastReportLine, pairedLine } from "./cloud-logic";
import { remoteState, remoteWords } from "./remote-logic";
import { useTunnel } from "./use-tunnel";

export function RemoteCard() {
  const tunnel = useTunnel();
  const away = useAway();
  const [pairing, setPairing] = useState(false);
  const data = tunnel.data;
  // Nothing while it loads or fails: the card above says both, about the
  // same request, and saying it twice is one time too many.
  if (!data) return null;
  const words = remoteWords(data, away);

  return (
    <Panel title="Back office from anywhere" aside={<StatusChip status={{ kind: "remote", status: remoteState(data) }} />}>
      <div className="flex flex-col gap-3">
        <p className="max-w-prose text-base text-fg-2">{words.line}</p>
        {words.address && <Address value={words.address} />}
        {words.steps.length > 0 && (
          <ol className="flex max-w-prose list-decimal flex-col gap-1 pl-5 text-base text-fg-2">
            {words.steps.map((step) => (
              <li key={step}>{step}</li>
            ))}
          </ol>
        )}
        {/* mez Cloud: who the venue is paired to, and the way to pair it. A
            code is typed at the venue, so from away there is the line and no
            button. */}
        {(data.paired_to || !away) && (
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line-soft pt-3">
            <p className="max-w-prose text-base text-fg-2">
              {data.paired_to ? pairedLine(data.paired_to) : "With a mez Cloud account, a code from it sets all of this up, and a backup with it."}
            </p>
            {!away && (
              <Button variant="secondary" onClick={() => setPairing(true)}>
                {data.paired_to ? "Pair again" : "Connect to mez Cloud"}
              </Button>
            )}
          </div>
        )}
        {/* The owner's WhatsApp, read only: what is sent is changed in the
            account area, which is where the link goes (ADR-364). */}
        {data.paired_to && lastReportLine(data.cloud, new Date()) && (
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="max-w-prose text-base text-fg-2">{lastReportLine(data.cloud, new Date())}</p>
            {data.cloud?.messages_url && (
              <a
                href={data.cloud.messages_url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex min-h-tap items-center rounded-tight text-base font-semibold text-attention-text underline-offset-2 outline-offset-2 outline-line-focus hover:underline focus-visible:outline-2"
              >
                Change what is sent
              </a>
            )}
          </div>
        )}
      </div>
      {pairing && <CloudPairModal onPair={tunnel.pairToCloud} onClose={() => setPairing(false)} />}
    </Panel>
  );
}

/** The address, in the numeral face so each character is read as it is, with a way to copy it. */
function Address({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    // Not every browser lets a page write to the clipboard, and a page on
    // plain http is one that may not. The address is on screen either way.
    void navigator.clipboard
      ?.writeText(value)
      .then(() => setCopied(true))
      .catch(() => setCopied(false));
  };
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-3">
      {/* An address has no spaces to break at, so it breaks anywhere: on a phone it is wider than the card. */}
      <a
        href={value}
        target="_blank"
        rel="noreferrer"
        className="max-w-full min-w-0 rounded-tight font-mono text-base font-semibold break-all text-attention-text outline-offset-2 outline-line-focus hover:underline focus-visible:outline-2"
      >
        {value}
      </a>
      <Button variant="secondary" onClick={copy}>
        {copied ? "Copied" : "Copy"}
      </Button>
    </div>
  );
}
