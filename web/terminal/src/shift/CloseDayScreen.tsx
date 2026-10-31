// Close the day (D23): two steps. Count the drawer, blind; then the cash
// reconciliation, the note when one is owed, the closing stock, and close.
// The Z-report the close issues is the screen after this one, with its slip.
//
// The design shows the expected figure above the count. Here the count is
// blind (ADR-093): the expected figure is never on this screen until the
// server's answer to the count brings it, so step one has no figure to copy.
// Its slip of the Z-report is not drawn beside the steps: there is no report
// until the shift is closed.
//
// The count holds every till at the close, so step one says first what is
// still open, with the way to it, and the confirm says it again (TBS-2).
// Open orders are not closed with the shift: they carry into the next one,
// and the Z-report says how many (ADR-384).

import { LockIcon, TriangleAlertIcon } from "lucide-react";
import { useState } from "react";
import type { ShiftResponse } from "../../../shared/api-types";
import { formatMoney } from "../../../shared/money";
import { Button, Card, Confirm, Money, useToast } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useDayOrders } from "../bills/use-bills";
import { ClosingStock } from "./ClosingStock";
import { stockCountOwed } from "./closing-stock-logic";
import { DrawerCounter, emptyCount, type Counted } from "./DrawerCounter";
import { carryWords, openOrders } from "./open-orders-logic";
import { OpenOrdersNote } from "./OpenOrdersNote";
import { useClosingStock } from "./use-closing-stock";
import { countRequest, maxReason, reasonOwed, toleranceText, varianceWords } from "./shift-logic";
import type { ShiftCommands } from "./use-shift";

interface CloseDayScreenProps {
  shift: ShiftResponse;
  currency: string;
  commands: ShiftCommands;
  /** Back to selling: offered only before the count, after which the drawer is shut. */
  onBack: () => void;
  /** The floor's names for its tables, to say where the open orders are. */
  names: ReadonlyMap<number, string>;
  /** To the Orders screen on its Open tab: offered, like Back, only before the count. */
  onSeeOpenOrders: () => void;
  /** Told the number of the Z-report the close issued. */
  onClosed: (zReport: number) => void;
}

export function CloseDayScreen({ shift, currency, commands, onBack, names, onSeeOpenOrders, onClosed }: CloseDayScreenProps) {
  const [counted, setCounted] = useState<Counted>(emptyCount);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();
  const stock = useClosingStock(shift.id);
  const day = useDayOrders();
  const open = day.data ? openOrders(day.data.orders) : null;
  const carry = carryWords(open ? open.length : null);

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
  const blocked = rec ? reasonOwed(rec, shift.reason !== undefined) !== null || stockCountOwed(stock.sheet.data) : false;

  return (
    <div className="flex min-w-0 flex-1">
      {/* The scroll keeps clear of the sticky foot: a row the counter scrolls into sight
          lands above Submit count, not under it (TBS-15). */}
      <section aria-label="Close the day" className="flex min-w-0 flex-1 scroll-pb-28 justify-center overflow-y-auto px-6 py-6 short:py-4">
        <div className={cx("flex w-full flex-col gap-5 short:gap-3", rec ? "max-w-2xl" : "max-w-3xl")}>
          <header>
            <p className="text-meta font-bold uppercase text-attention-text">Step {rec ? 2 : 1} of 2</p>
            <h1 className="mt-2 font-serif text-total short:mt-1">Close the day</h1>
            {/* The rail beside the steps holds the shift's facts; without room for it they are said here. */}
            <p className="mt-1 font-mono text-sm tabular-nums text-fg-muted lg:hidden">
              {shift.trading_day} · {shift.public_id}
            </p>
            <p className="mt-1 text-base text-fg-muted">
              {rec
                ? "Check the drawer against what was expected, then close. The Z-report follows."
                : "Count the drawer. What was expected shows once the count is in."}
            </p>
          </header>

          {!rec && <OpenOrdersNote open={open} failed={day.isError} names={names} onSee={onSeeOpenOrders} />}

          {rec ? (
            <>
              <Reconciliation shift={shift} busy={busy} onExplain={(reason) => void run(() => commands.explain(shift.id, reason))} />
              {stock.sheet.data && <ClosingStock sheet={stock.sheet.data} onRecord={stock.record} />}
            </>
          ) : (
            <DrawerCounter title="Count the drawer" currency={currency} value={counted} onChange={setCounted} disabled={busy} totalBeside />
          )}

          {/* Kept in sight under a long closing count: the way on, and what went wrong, stay at the foot. */}
          <div className="sticky bottom-0 -mx-1 flex flex-col gap-2 bg-canvas px-1 pb-1 pt-3">
            {error && (
              <p role="alert" className="text-base text-danger">
                {error}
              </p>
            )}
            <div className="flex flex-wrap gap-3">
              {rec ? (
                <Button
                  variant="primary"
                  size="charge"
                  className="flex-1"
                  disabled={busy || blocked}
                  onClick={() =>
                    void run(async () => {
                      const closed = await commands.close(shift.id);
                      onClosed(closed.z_report ?? 0);
                    }, "Shift closed.")
                  }
                >
                  <LockIcon aria-hidden="true" size={19} />
                  Close the day
                </Button>
              ) : (
                <>
                  <Button size="charge" disabled={busy} onClick={onBack}>
                    Back to the order screen
                  </Button>
                  <Button variant="primary" size="charge" className="flex-1" disabled={busy} onClick={() => setConfirming(true)}>
                    Submit count
                  </Button>
                </>
              )}
            </div>
          </div>
        </div>
      </section>

      <aside aria-label="The shift" className="hidden w-rail shrink-0 flex-col gap-4 border-l border-line bg-chrome p-5 lg:flex">
        <p className="text-meta font-bold uppercase text-fg-faint">The shift</p>
        <dl className="grid gap-3 text-base">
          <Fact label="Trading day" value={shift.trading_day} />
          <Fact label="Opened" value={new Date(shift.opened_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} />
          <Fact label="Shift" value={shift.public_id} />
          <div className="flex items-baseline justify-between gap-3">
            <dt className="text-fg-2">Opening float</dt>
            <dd>
              <Money amount={shift.float} />
            </dd>
          </div>
          {!rec && (
            <div className="flex items-baseline justify-between gap-3 border-t border-line pt-3">
              <dt className="font-serif text-xl">Counted</dt>
              <dd>
                <Money amount={request.total} variant="total" />
              </dd>
            </div>
          )}
        </dl>
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
        {carry && <p className="mb-2 text-base font-semibold">{carry}</p>}
        <p className="text-base">The count can't be changed once it's in. Submit {formatMoney(request.total)}?</p>
      </Confirm>
    </div>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <dt className="text-fg-2">{label}</dt>
      <dd className="font-mono tabular-nums">{value}</dd>
    </div>
  );
}

interface ReconciliationProps {
  shift: ShiftResponse;
  busy: boolean;
  onExplain: (reason: string) => void;
}

const tones = { danger: "bg-danger-tint text-danger", warning: "bg-chip-warning-bg text-chip-warning-fg", success: "bg-accent-tint text-accent-text" } as const;

/** The cash reconciliation card (D23): what was expected, what was counted, how it came out, and the note for it. */
function Reconciliation({ shift, busy, onExplain }: ReconciliationProps) {
  const [reason, setReason] = useState("");
  const rec = shift.reconciliation;
  if (!rec) return null;
  const words = varianceWords(rec);
  const owed = reasonOwed(rec, shift.reason !== undefined);

  return (
    <Card padding="lg" role="region" aria-label="Cash reconciliation">
      <h2 className="text-meta font-bold uppercase text-fg-faint">Cash reconciliation</h2>
      <dl className="mt-3 grid gap-2.5 text-base">
        <Figure label="Opening float" amount={shift.float} />
        <div className="flex items-baseline justify-between border-t border-line pt-2.5 font-bold">
          <dt>Expected in drawer</dt>
          <dd>
            <Money amount={rec.expected} />
          </dd>
        </div>
        <Figure label="Counted" amount={rec.counted} />
      </dl>

      <div className={cx("mt-4 flex items-start gap-3 rounded p-3.5", tones[words.tone])}>
        {words.tone !== "success" && <TriangleAlertIcon aria-hidden="true" size={20} className="mt-0.5 shrink-0" />}
        <div>
          <p className="text-base font-bold">{words.text}</p>
          <p className="text-sm text-fg">{owed ?? toleranceText(rec, shift.reason !== undefined)}</p>
        </div>
      </div>

      {shift.reason ? (
        <blockquote className="mt-4 border-l-2 border-line pl-4 text-base text-fg-2">
          <p>{shift.reason.reason}</p>
          <footer className="mt-1 text-sm text-fg-muted">Saved with the count. It can't be changed.</footer>
        </blockquote>
      ) : (
        <form
          className="mt-4 flex flex-col gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (reason.trim()) onExplain(reason);
          }}
        >
          <label className="flex flex-col gap-1.5 text-sm font-semibold">
            {owed ? "Why?" : "A note on the count, if there is one"}
            <textarea
              value={reason}
              maxLength={maxReason}
              rows={3}
              onChange={(e) => setReason(e.target.value)}
              className="rounded border border-line-strong bg-raised px-4 py-3 text-base font-normal text-fg outline-accent-tint focus-visible:border-accent focus-visible:outline-3"
            />
          </label>
          <Button type="submit" disabled={busy || reason.trim() === ""} className="self-start">
            Save note
          </Button>
        </form>
      )}
    </Card>
  );
}

function Figure({ label, amount }: { label: string; amount: ShiftResponse["float"] }) {
  return (
    <div className="flex items-baseline justify-between text-fg-2">
      <dt>{label}</dt>
      <dd>
        <Money amount={amount} />
      </dd>
    </div>
  );
}
