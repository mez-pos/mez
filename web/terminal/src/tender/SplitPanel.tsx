// Dividing the bill (terminal-tender.md, Task 2): evenly, by amount, by item
// or by guest. Every share shown comes from POST /orders/{id}/split, which
// runs the domain's split package; the panel only says what was chosen.
// "Use this split" hands the shares to the tender screen, one tender each.

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import type { OrderResponse, SplitKind, SplitResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Button, Money, NumPad, Qty, applyAmountKey } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { failureOf } from "../register/order-messages";
import { orderQueryKey } from "../register/use-ticket";
import { billed, maxWays, payerLabel, requestFor, togglePayer, type SplitDraft } from "./split-logic";

interface SplitPanelProps {
  order: OrderResponse;
  onUse: (split: SplitResponse, amountsKeyed: number) => void;
  onCancel: () => void;
}

const kinds: { kind: SplitKind; name: string }[] = [
  { kind: "evenly", name: "Evenly" },
  { kind: "amount", name: "By amount" },
  { kind: "item", name: "By item" },
  { kind: "guest", name: "By guest" },
];

export function SplitPanel({ order, onUse, onCancel }: SplitPanelProps) {
  const store = useAuthStore();
  const [draft, setDraft] = useState<SplitDraft>({ kind: "evenly", ways: 2, amounts: [], payers: {}, seats: {} });
  const [keyed, setKeyed] = useState(0);
  const request = requestFor(draft, order.lines, order.currency);

  // Asked again whenever the choices or the order move, so the shares shown
  // are always the server's for the bill as it is now.
  const split = useQuery({
    queryKey: [...orderQueryKey(order.id), "split", order.last_seq, request],
    queryFn: ({ signal }) => store.api.post<SplitResponse>(`/api/v1/orders/${encodeURIComponent(order.id)}/split`, request, { signal }),
    enabled: request !== null,
    staleTime: Infinity,
    retry: false,
  });
  const shares = request ? split.data : undefined;
  const set = (patch: Partial<SplitDraft>) => setDraft((d) => ({ ...d, ...patch }));
  const who = (payer: number) => payerLabel(draft.kind, payer, draft.amounts.length);

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-serif text-xl">Split the bill</h2>
        <Button variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
      <div role="radiogroup" aria-label="Split by" className="grid grid-cols-4 gap-tap-gap">
        {kinds.map((k) => (
          <button
            key={k.kind}
            type="button"
            role="radio"
            aria-checked={draft.kind === k.kind}
            onClick={() => set({ kind: k.kind })}
            className={cx(
              "min-h-tap rounded border px-3 text-base font-semibold outline-offset-2 outline-line-focus focus-visible:outline-2",
              draft.kind === k.kind ? "border-accent bg-accent-tint text-accent-text" : "border-line bg-raised text-fg-2",
            )}
          >
            {k.name}
          </button>
        ))}
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)] gap-5">
        <div className="min-h-0 overflow-y-auto rounded-card border border-line bg-chrome shadow-rest p-5">
          {draft.kind === "amount" ? (
            <AmountChooser
              amounts={draft.amounts}
              keyed={keyed}
              currency={order.currency}
              onKey={(k) => setKeyed((m) => applyAmountKey(m, k, order.totals.balance_due.minor))}
              onAdd={() => {
                set({ amounts: [...draft.amounts, keyed] });
                setKeyed(0);
              }}
              onRemove={(i) => set({ amounts: draft.amounts.filter((_, j) => j !== i) })}
            />
          ) : (
            <>
              <div className="flex items-center justify-between gap-3">
                <span className="text-base text-fg-2">{draft.kind === "guest" ? "Guests" : "Payers"}</span>
                <Qty
                  value={draft.ways}
                  onChange={(ways) => set({ ways })}
                  bounds={{ min: draft.kind === "evenly" ? 2 : 1, max: maxWays }}
                  label={draft.kind === "guest" ? "Guests" : "Payers"}
                />
              </div>
              {(draft.kind === "item" || draft.kind === "guest") && <LineChooser draft={draft} order={order} onDraft={set} />}
            </>
          )}
        </div>

        <section aria-label="Shares" className="flex min-h-0 flex-col rounded-card border border-line bg-chrome shadow-rest p-5">
          <p className="text-meta font-semibold uppercase text-fg-muted">Shares, from the server</p>
          <div className="min-h-0 flex-1 overflow-y-auto" aria-live="polite">
            {!request && <p className="mt-3 text-base text-fg-muted">Key an amount and add it.</p>}
            {request && split.isPending && <p className="mt-3 text-base text-fg-muted">Working it out</p>}
            {request && split.isError && (
              <p role="alert" className="mt-3 text-base text-danger">
                {failureOf(split.error)}
              </p>
            )}
            {shares && (
              <ol className="mt-2">
                {shares.shares.map((s) => (
                  <li key={s.payer} className="flex items-baseline justify-between border-b border-line py-2.5 text-base">
                    <span>{who(s.payer)}</span>
                    <Money amount={s.amount} />
                  </li>
                ))}
              </ol>
            )}
          </div>
          {shares && (
            <div className="mt-3 flex items-baseline justify-between text-base text-fg-2">
              <span>In all, {shares.kind === "evenly" || shares.kind === "amount" ? "what is owed" : "the bill"}</span>
              <Money amount={shares.total} />
            </div>
          )}
          <Button
            variant="primary"
            size="charge"
            disabled={!shares || split.isFetching}
            onClick={() => shares && onUse(shares, draft.amounts.length)}
            className="mt-4"
          >
            Use this split
          </Button>
        </section>
      </div>
    </div>
  );
}

interface AmountChooserProps {
  amounts: readonly number[];
  keyed: number;
  currency: string;
  onKey: Parameters<typeof NumPad>[0]["onKey"];
  onAdd: () => void;
  onRemove: (index: number) => void;
}

function AmountChooser({ amounts, keyed, currency, onKey, onAdd, onRemove }: AmountChooserProps) {
  const money = (minor: number) => ({ minor, currency });
  return (
    <div className="flex flex-col gap-3">
      <p className="text-base text-fg-muted">Key what each payer puts in. Whatever is left is one more payer.</p>
      {amounts.map((a, i) => (
        <div key={i} className="flex items-center justify-between gap-3">
          <span className="text-base">Payer {i + 1}</span>
          <span className="flex items-center gap-tap-gap">
            <Money amount={money(a)} />
            <Button variant="ghost" onClick={() => onRemove(i)} aria-label={`Remove payer ${i + 1}`}>
              Remove
            </Button>
          </span>
        </div>
      ))}
      <div className="flex items-center gap-tap-gap">
        <div className="flex min-h-tap flex-1 items-center justify-end rounded border border-line-focus bg-canvas px-4">
          <Money amount={money(keyed)} className="text-xl" />
        </div>
        <Button disabled={keyed <= 0 || amounts.length >= maxWays - 1} onClick={onAdd}>
          Add payer
        </Button>
      </div>
      <NumPad layout="amount" label="Payer's amount" onKey={onKey} />
    </div>
  );
}

interface LineChooserProps {
  draft: SplitDraft;
  order: OrderResponse;
  onDraft: (patch: Partial<SplitDraft>) => void;
}

/** Every billed line with who pays for it: payers to toggle, or one seat. */
function LineChooser({ draft, order, onDraft }: LineChooserProps) {
  const byGuest = draft.kind === "guest";
  const people = Array.from({ length: draft.ways }, (_, i) => i);
  return (
    <ol className="mt-4">
      {billed(order.lines).map((l) => {
        const payers = (draft.payers[l.id] ?? [0]).filter((p) => p < draft.ways);
        const seat = draft.seats[l.id] ?? 0;
        const chosen = (i: number) => (byGuest ? seat === i : (payers.length > 0 ? payers : [0]).includes(i));
        const options = byGuest ? [0, ...people.map((p) => p + 1)] : people;
        return (
          <li key={l.id} className="border-b border-line py-3">
            <div className="flex items-baseline justify-between gap-3">
              <span className="text-base font-medium">
                <span className="font-mono text-sm text-fg-3">{l.qty}×</span> {l.name}
              </span>
              <Money amount={l.due} />
            </div>
            <div role="group" aria-label={`Who pays for ${l.name}`} className="mt-2 flex flex-wrap gap-tap-gap">
              {options.map((i) => (
                <button
                  key={i}
                  type="button"
                  aria-pressed={chosen(i)}
                  onClick={() =>
                    byGuest
                      ? onDraft({ seats: { ...draft.seats, [l.id]: i } })
                      : onDraft({ payers: { ...draft.payers, [l.id]: togglePayer(payers.length > 0 ? payers : [0], i) } })
                  }
                  className={cx(
                    "min-h-tap min-w-tap rounded border px-3 text-base outline-offset-2 outline-line-focus focus-visible:outline-2",
                    chosen(i) ? "border-accent bg-accent-tint font-semibold text-accent-text" : "border-line bg-raised text-fg-muted",
                  )}
                >
                  {byGuest ? (i === 0 ? "Table" : `Guest ${i}`) : `Payer ${i + 1}`}
                </button>
              ))}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
