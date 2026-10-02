// Close the day (terminal-close.md): count the drawer blind, then the
// reveal, the reason when one is owed, and close. The expected figure is
// never on this screen until the server's answer to the count brings it.

import { useState } from "react";
import type { ShiftResponse } from "../../../shared/api-types";
import { formatMoney } from "../../../shared/money";
import { Button, Confirm, Meta, Money, useToast } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { ClosingStock } from "./ClosingStock";
import { stockCountOwed } from "./closing-stock-logic";
import { DrawerCounter, emptyCount, type Counted } from "./DrawerCounter";
import { useClosingStock } from "./use-closing-stock";
import { countRequest, maxReason, reasonOwed, varianceWords } from "./shift-logic";
import type { ShiftCommands } from "./use-shift";

interface CloseDayScreenProps {
  shift: ShiftResponse;
  currency: string;
  commands: ShiftCommands;
  /** Back to selling: offered only before the count, after which the drawer is shut. */
  onBack: () => void;
  /** Told the number of the Z-report the close issued. */
  onClosed: (zReport: number) => void;
}

export function CloseDayScreen({ shift, currency, commands, onBack, onClosed }: CloseDayScreenProps) {
  const [counted, setCounted] = useState<Counted>(emptyCount);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();
  const stock = useClosingStock(shift.id);

  const run = async (what: () => Promise<unknown>, done?: string) => {
    setBusy(true);
    setError(null);
    try {
      await what();
      if (done) toast(done, { tone: "success" });
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const request = countRequest(currency, counted.tally, counted.typed);
  const rec = shift.reconciliation;

  return (
    <div className="flex min-w-0 flex-1">
      <section aria-label="Close the day" className="flex min-w-0 flex-1 justify-center overflow-y-auto px-6 py-5">
        {rec ? (
          <div className="flex w-full max-w-xl flex-col gap-10">
            <Reveal shift={shift} busy={busy} onExplain={(reason) => void run(() => commands.explain(shift.id, reason))} />
            {stock.sheet.data && <ClosingStock sheet={stock.sheet.data} onRecord={stock.record} />}
          </div>
        ) : (
          <DrawerCounter title="Count the drawer" currency={currency} value={counted} onChange={setCounted} disabled={busy} />
        )}
      </section>

      <aside aria-label="The shift" className="flex w-104 shrink-0 flex-col border-l border-line bg-chrome">
        <header className="border-b border-line px-5 py-4">
          <Meta items={["Close the day", shift.trading_day]} />
          <p className="mt-1 font-mono text-base text-fg-2">
            {shift.public_id} · opened {new Date(shift.opened_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
          </p>
        </header>
        <div className="min-h-0 flex-1" />
        <footer className="flex flex-col gap-tap-gap border-t border-line px-5 pb-5 pt-4">
          {error && (
            <p role="alert" className="text-base text-danger">
              {error}
            </p>
          )}
          {rec ? (
            <Button
              variant="primary"
              size="charge"
              disabled={busy || reasonOwed(rec, shift.reason !== undefined) !== null || stockCountOwed(stock.sheet.data)}
              onClick={() =>
                void run(async () => {
                  const closed = await commands.close(shift.id);
                  onClosed(closed.z_report ?? 0);
                }, "Shift closed.")
              }
            >
              Close the shift
            </Button>
          ) : (
            <>
              <div className="flex items-baseline justify-between">
                <span className="font-serif text-xl">Counted</span>
                <Money amount={request.total} variant="total" />
              </div>
              <Button variant="primary" size="charge" disabled={busy} onClick={() => setConfirming(true)}>
                Submit count
              </Button>
              <Button variant="ghost" disabled={busy} onClick={onBack}>
                Back to the register
              </Button>
            </>
          )}
        </footer>
      </aside>

      <Confirm
        open={confirming}
        title="Submit the count"
        confirmLabel="Submit count"
        onClose={() => setConfirming(false)}
        onConfirm={() => {
          setConfirming(false);
          void run(() => commands.count(shift.id, request));
        }}
      >
        <p className="text-base">The count can't be changed once it's in. Submit {formatMoney(request.total)}?</p>
      </Confirm>
    </div>
  );
}

interface RevealProps {
  shift: ShiftResponse;
  busy: boolean;
  onExplain: (reason: string) => void;
}

/** How the drawer came out, and the reason for it. */
function Reveal({ shift, busy, onExplain }: RevealProps) {
  const [reason, setReason] = useState("");
  const rec = shift.reconciliation;
  if (!rec) return null;
  const words = varianceWords(rec);
  const owed = reasonOwed(rec, shift.reason !== undefined);

  return (
    <section aria-label="The count" className="flex w-full max-w-xl flex-col gap-6">
      <h2 className="font-serif text-xl">The drawer</h2>
      <dl className="flex flex-col">
        <Figure label="Expected" amount={rec.expected} />
        <Figure label="Counted" amount={rec.counted} />
      </dl>
      <p
        className={cx(
          "flex items-baseline justify-between border-t border-line pt-3 font-serif text-xl",
          words.tone === "danger" && "text-danger",
          words.tone === "warning" && "text-warning",
          words.tone === "success" && "text-success",
        )}
      >
        <span>{words.text}</span>
      </p>

      {shift.reason ? (
        <blockquote className="border-l-2 border-line pl-4 text-base text-fg-2">
          <p>{shift.reason.reason}</p>
          <footer className="mt-1 text-sm text-fg-muted">Saved with the count. It can't be changed.</footer>
        </blockquote>
      ) : (
        <form
          className="flex flex-col gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (reason.trim()) onExplain(reason);
          }}
        >
          {owed && <p className="text-base text-fg-2">{owed}</p>}
          <label className="flex flex-col gap-1 text-sm text-fg-muted">
            {owed ? "Why?" : "A note on the count, if there is one"}
            <textarea
              value={reason}
              maxLength={maxReason}
              rows={3}
              onChange={(e) => setReason(e.target.value)}
              className="rounded border border-line-strong bg-raised px-4 py-3 text-base text-fg outline-accent-tint focus-visible:border-accent focus-visible:outline-3"
            />
          </label>
          <Button type="submit" disabled={busy || reason.trim() === ""} className="self-start">
            Save reason
          </Button>
        </form>
      )}
    </section>
  );
}

function Figure({ label, amount }: { label: string; amount: ShiftResponse["float"] }) {
  return (
    <div className="flex items-baseline justify-between py-1 text-base text-fg-2">
      <dt>{label}</dt>
      <dd>
        <Money amount={amount} />
      </dd>
    </div>
  );
}
