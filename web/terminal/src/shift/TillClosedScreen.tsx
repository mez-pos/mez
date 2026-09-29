// Till closed (terminal-close.md): in the register's place while no shift
// is open. Count the float in and open the till; a float of nothing is a
// float.

import { useState } from "react";
import { formatMoney } from "../../../shared/money";
import { Button, Card, EmptyState, InboxIcon, useToast } from "../../../shared/ui";
import { DrawerCounter, emptyCount, type Counted } from "./DrawerCounter";
import { countRequest } from "./shift-logic";
import type { ShiftState } from "./use-shift";

interface TillClosedScreenProps {
  currency: string;
  state: ShiftState;
  /** The Z-report this till's close just issued, if any. */
  lastZ: number | null;
  onViewZ: () => void;
}

export function TillClosedScreen({ currency, state, lastZ, onViewZ }: TillClosedScreenProps) {
  const [counted, setCounted] = useState<Counted>(emptyCount);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  if (state.shift === undefined) {
    return state.error ? (
      <EmptyState
        icon={<InboxIcon />}
        line="Can't reach the Nofiltr server to check the till."
        action={{ label: "Try again", onClick: state.retry }}
      />
    ) : (
      <p className="m-auto text-base text-fg-muted">Checking the till</p>
    );
  }

  const float = countRequest(currency, counted.tally, counted.typed);
  const open = async () => {
    setBusy(true);
    setError(null);
    try {
      await state.commands.open(float);
      toast("The till is open.", { tone: "success" });
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-w-0 flex-1 justify-center overflow-y-auto px-6 py-8">
      <div className="flex w-full max-w-3xl flex-col gap-6">
        <header>
          <h1 className="font-serif text-total">The till is closed</h1>
          <p className="mt-1 text-base text-fg-muted">Count the float into the drawer to start the day.</p>
        </header>
        {lastZ !== null && lastZ > 0 && (
          <Card className="flex flex-wrap items-center justify-between gap-4">
            <p className="text-base">
              <span className="font-semibold">Z-report {lastZ} issued.</span> It is printing.
            </p>
            <Button onClick={onViewZ}>View the report</Button>
          </Card>
        )}
        <DrawerCounter title="Float" currency={currency} value={counted} onChange={setCounted} disabled={busy} />
        {error && (
          <p role="alert" className="text-base text-danger">
            {error}
          </p>
        )}
        <Button variant="primary" size="charge" disabled={busy} onClick={() => void open()}>
          Open the till with {formatMoney(float.total)}
        </Button>
      </div>
    </div>
  );
}
