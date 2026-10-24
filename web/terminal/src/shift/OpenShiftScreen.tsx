// Open the shift (D02): step one of the cashier's day. The drawer is counted
// into a float, the float is the shift's opening figure, and the order screen
// follows. The shift's details and anything sold out sit beside it, so what
// matters before service is in view, not behind a wall (ADR-150).
//
// Left out because the server cannot do them: the expected figure from the last
// close (the count is blind, ADR-094), a note for the manager, the shift's
// window, who else is on the floor, the low-stock count, and the artwork.

import { LockIcon, UnlockIcon } from "lucide-react";
import { useState } from "react";
import { formatMoney } from "../../../shared/money";
import { Button, Card, useToast } from "../../../shared/ui";
import { FloatCount } from "./FloatCount";
import { emptyFloat, floatRequest, type FloatCounted } from "./open-shift-logic";
import { ShiftDetailsCard, ShiftStage, SoldOutCard, type DetailRow } from "./ShiftStage";
import type { ShiftCommands } from "./use-shift";

interface OpenShiftScreenProps {
  currency: string;
  open: ShiftCommands["open"];
  details: readonly DetailRow[];
  soldOut: { title: string; body: string } | null;
  /** Where the menu is changed, for someone who may; null for someone who may not. */
  menuHref: string | null;
  /** The Z-report this till's close just issued, if any. */
  lastZ: number | null;
  onViewZ: () => void;
  /** Back to the sign-in screen. */
  onCancel: () => void;
}

export function OpenShiftScreen({ currency, open, details, soldOut, menuHref, lastZ, onViewZ, onCancel }: OpenShiftScreenProps) {
  const [counted, setCounted] = useState<FloatCounted>(emptyFloat);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  // Null while the amount typed is not one: nothing is sent as a guessed zero.
  const float = floatRequest(currency, counted);
  const submit = async () => {
    if (!float) return;
    setBusy(true);
    setError(null);
    try {
      await open(float);
      toast(`The shift is open with ${formatMoney(float.total)}.`, { tone: "success" });
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <ShiftStage
      main={
        <>
          <p className="text-meta font-bold uppercase text-attention-text">Step 1 of 2</p>
          <h1 className="mt-2.5 font-serif text-total">Open the shift</h1>
          <p className="mt-2 text-base text-fg-muted">{"Count the cash drawer before the first order. This becomes tonight's opening float."}</p>
          {lastZ !== null && lastZ > 0 && (
            <Card className="mt-6 flex flex-wrap items-center justify-between gap-4">
              <p className="text-base">
                <span className="font-semibold">Z-report {lastZ} issued.</span> It is printing.
              </p>
              <Button onClick={onViewZ}>View the report</Button>
            </Card>
          )}
          <div className="mt-8">
            <FloatCount currency={currency} value={counted} onChange={setCounted} disabled={busy} />
          </div>
          {error && (
            <p role="alert" className="mt-4 text-base text-danger">
              {error}
            </p>
          )}
          <div className="mt-6 flex flex-col gap-3.5 sm:flex-row">
            <Button size="charge" className="sm:w-48" disabled={busy} onClick={onCancel}>
              <LockIcon aria-hidden="true" size={19} />
              Lock
            </Button>
            <Button variant="primary" size="charge" className="flex-1" disabled={busy || float === null} onClick={() => void submit()}>
              <UnlockIcon aria-hidden="true" size={19} />
              Open shift &amp; start selling
            </Button>
          </div>
        </>
      }
      aside={
        <>
          <ShiftDetailsCard rows={details} />
          {soldOut && <SoldOutCard title={soldOut.title} body={soldOut.body} menuHref={menuHref} />}
        </>
      }
    />
  );
}
