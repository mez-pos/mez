// Making a split (D08a): by item, equally, or custom amounts. The mode and
// what it needs down the left, a card for each guest on the right, and Back
// and Complete split along the foot. Every figure on a card comes from
// POST /orders/{id}/split, which runs the domain's split package; the panel
// only says what was chosen. "Complete split" fixes the shares, and each is
// then collected as one payment (SplitShares.tsx).
//
// By item, a guest is picked and the items tapped go to them. An item can be
// shared between every guest, and a guest taken off it again on their card.
// The design's "Share of tax" row is left out: a line's figure here already
// has its tax in it, as the bill's does.

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { DivideIcon, PercentIcon, UserPlusIcon, UsersIcon, type LucideIcon } from "lucide-react";
import { useState } from "react";
import type { Money as Amount, OrderResponse, SplitResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Button, Money, NumPad, Qty, applyAmountKey } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { failureOf } from "../register/order-messages";
import { orderQueryKey } from "../register/use-ticket";
import {
  assign,
  billed,
  looseShare,
  maxWays,
  payerLabel,
  payersOf,
  requestFor,
  splitBlock,
  splitNames,
  unassign,
  unassigned,
  type SplitDraft,
  type TillSplit,
} from "./split-logic";
import { ColumnHeading, GuestCard, SplitFrame, modeTile, modeTileOn, type ShareLine } from "./SplitParts";

interface SplitPanelProps {
  order: OrderResponse;
  /** "Table T4", or how the order is served. */
  title: string;
  draft: SplitDraft;
  onDraft: (draft: SplitDraft) => void;
  onUse: (split: SplitResponse, amountsKeyed: number) => void;
  onBack: () => void;
}

const modes: readonly { kind: TillSplit; icon: LucideIcon }[] = [
  { kind: "item", icon: UsersIcon },
  { kind: "evenly", icon: DivideIcon },
  { kind: "amount", icon: PercentIcon },
];

export function SplitPanel({ order, title, draft, onDraft, onUse, onBack }: SplitPanelProps) {
  const store = useAuthStore();
  const [keyed, setKeyed] = useState(0);
  const [picked, setPicked] = useState(0);
  const currency = order.currency;
  const money = (minor: number): Amount => ({ minor, currency });
  const request = requestFor(draft, order.lines, currency);
  // The server divides the lines only on a bill with nothing paid yet.
  const partPaid = order.totals.net_paid.minor !== 0;

  // Asked again whenever the choices or the order move, so the shares shown
  // are always the server's for the bill as it is now. The last answer stays
  // up, dimmed, while the next is on its way.
  const split = useQuery({
    queryKey: [...orderQueryKey(order.id), "split", order.last_seq, request],
    queryFn: ({ signal }) => store.api.post<SplitResponse>(`/api/v1/orders/${encodeURIComponent(order.id)}/split`, request, { signal }),
    enabled: request !== null,
    staleTime: Infinity,
    retry: false,
    placeholderData: keepPreviousData,
  });
  const answer = request && split.data?.kind === draft.kind ? split.data : undefined;
  const settling = split.isPlaceholderData || split.isFetching;
  const fresh = answer && !settling ? answer : undefined;
  const block = splitBlock(draft, order.lines, fresh);

  const set = (patch: Partial<SplitDraft>) => onDraft({ ...draft, ...patch });
  const guest = Math.min(picked, draft.ways - 1);
  const loose = unassigned(draft, order.lines);
  const held = answer ? looseShare(draft, order.lines, answer) : undefined;
  const guests = draft.kind === "amount" ? (answer ? answer.shares.length : draft.amounts.length) : draft.ways;
  // By item, one payer is kept free to hold the items nobody has yet.
  const most = draft.kind === "item" ? maxWays - 1 : maxWays;

  const linesOf = (payer: number): ShareLine[] =>
    draft.kind !== "item"
      ? []
      : billed(order.lines).flatMap((l) => {
          const payers = payersOf(draft, l.id);
          if (!payers.includes(payer)) return [];
          return [
            {
              id: l.id,
              label: `${l.qty} × ${l.name}`,
              amount: payers.length === 1 ? l.due : null,
              sharedBy: payers.length,
              onRemove: () => onDraft(unassign(draft, l.id, payer)),
            },
          ];
        });

  const aside = (
    <>
      <ColumnHeading>Split mode</ColumnHeading>
      <div role="radiogroup" aria-label="Split mode" className="grid gap-2.5">
        {modes.map(({ kind, icon: Icon }) => {
          const on = draft.kind === kind;
          return (
            <button
              key={kind}
              type="button"
              role="radio"
              aria-checked={on}
              disabled={kind === "item" && partPaid}
              onClick={() => set({ kind, ways: Math.min(draft.ways, kind === "item" ? maxWays - 1 : maxWays) })}
              className={cx(
                modeTile,
                "outline-offset-2 outline-attention focus-visible:outline-3 disabled:opacity-40",
                on ? modeTileOn : "border-line-strong bg-raised font-semibold text-fg hover:bg-raised-high",
              )}
            >
              <Icon aria-hidden="true" size={19} strokeWidth={1.9} />
              {splitNames[kind]}
            </button>
          );
        })}
      </div>
      {partPaid && <p className="mt-2.5 text-sm text-fg-muted">By item is for a bill with nothing paid yet.</p>}

      <div aria-hidden="true" className="my-5 h-px bg-line" />

      {draft.kind === "amount" ? (
        <AmountChooser
          guest={draft.amounts.length + 1}
          keyed={money(keyed)}
          full={draft.amounts.length >= maxWays - 1}
          onKey={(k) => setKeyed((m) => applyAmountKey(m, k, order.totals.balance_due.minor))}
          onAdd={() => {
            set({ amounts: [...draft.amounts, keyed] });
            setKeyed(0);
          }}
        />
      ) : (
        <div className="flex items-center justify-between gap-3">
          <ColumnHeading>Guests</ColumnHeading>
          <Qty value={draft.ways} onChange={(ways) => set({ ways })} bounds={{ min: 2, max: most }} label="Guests" className="mb-3.5" />
        </div>
      )}

      {draft.kind === "evenly" && <p className="text-sm text-fg-muted">Each guest pays an equal share of what is owed.</p>}

      {draft.kind === "item" && (
        <>
          <ColumnHeading>Unassigned items</ColumnHeading>
          {loose.length === 0 ? (
            <p className="text-sm text-fg-muted">Every item has a guest.</p>
          ) : (
            <>
              <p className="mb-2.5 text-sm text-fg-muted">Tap an item to give it to Guest {guest + 1}, or share it between everyone.</p>
              <ul className="grid gap-2.5">
                {loose.map((l) => (
                  <li key={l.id} className="flex items-stretch gap-1 rounded border border-dashed border-line-strong p-1">
                    <button
                      type="button"
                      aria-label={`Give ${l.name} to Guest ${guest + 1}`}
                      onClick={() => onDraft(assign(draft, l.id, [guest]))}
                      className="flex min-h-tap min-w-0 flex-1 items-center gap-2.5 rounded-tight px-2.5 text-left text-base outline-offset-2 outline-attention hover:bg-raised-high focus-visible:outline-3"
                    >
                      <span className="min-w-0 flex-1">
                        {l.qty} × {l.name}
                      </span>
                      <Money amount={l.due} className="font-bold" />
                    </button>
                    <button
                      type="button"
                      aria-label={`Share ${l.name} between every guest`}
                      onClick={() =>
                        onDraft(
                          assign(
                            draft,
                            l.id,
                            Array.from({ length: draft.ways }, (_, i) => i),
                          ),
                        )
                      }
                      className="min-h-tap shrink-0 rounded-tight px-3 text-base font-semibold text-attention-text outline-offset-2 outline-attention hover:bg-attention-tint focus-visible:outline-3"
                    >
                      Share
                    </button>
                  </li>
                ))}
              </ul>
              <p className="mt-4 rounded bg-chip-warning-bg p-3 text-sm text-chip-warning-fg">Assign every item before completing the split.</p>
            </>
          )}
        </>
      )}
    </>
  );

  const footer = (
    <>
      <p className="min-w-0 flex-1 text-base text-fg-muted" aria-live="polite">
        {draft.kind === "item" && answer && (
          <>
            Assigned <Money amount={money(answer.total.minor - (held?.amount.minor ?? 0))} className="font-bold text-fg" /> of <Money amount={answer.total} />
          </>
        )}
        {draft.kind !== "item" && fresh && `${fresh.shares.length} shares`}
        {block && loose.length === 0 && !split.isError && <span className="block text-sm">{block}</span>}
      </p>
      <Button onClick={onBack} size="charge" className="w-40">
        Back
      </Button>
      <Button variant="primary" size="charge" disabled={!fresh || block !== null} onClick={() => fresh && onUse(fresh, draft.amounts.length)} className="w-64">
        Complete split
      </Button>
    </>
  );

  return (
    <SplitFrame heading={`Split bill · ${title}`} totalLabel={partPaid ? "Still owed" : "Total"} total={order.totals.balance_due} aside={aside} footer={footer}>
      {split.isError && request && (
        <p role="alert" className="col-span-2 text-base text-danger">
          {failureOf(split.error)}
        </p>
      )}
      {Array.from({ length: guests }, (_, payer) => {
        const share = answer?.shares.find((s) => s.payer === payer);
        const keyedHere = draft.kind === "amount" && payer < draft.amounts.length;
        return (
          <GuestCard
            key={payer}
            seat={payer + 1}
            name={payerLabel(draft.kind, payer, draft.amounts.length)}
            paid={false}
            lines={linesOf(payer)}
            due={share?.amount ?? (keyedHere ? money(draft.amounts[payer] ?? 0) : undefined)}
            settling={settling}
            selected={draft.kind === "item" && payer === guest}
            onSelect={draft.kind === "item" ? () => setPicked(payer) : undefined}
          >
            {keyedHere && (
              <Button variant="ghost" onClick={() => set({ amounts: draft.amounts.filter((_, j) => j !== payer) })}>
                Remove
              </Button>
            )}
          </GuestCard>
        );
      })}
      {draft.kind === "amount" ? (
        draft.amounts.length === 0 && <p className="col-span-2 text-base text-fg-muted">Key what the first guest pays, then add them.</p>
      ) : (
        <button
          type="button"
          disabled={draft.ways >= most}
          onClick={() => {
            set({ ways: draft.ways + 1 });
            setPicked(draft.ways);
          }}
          className="flex min-h-40 flex-col items-center justify-center gap-2.5 rounded-card border border-dashed border-line-strong p-4 text-base font-semibold text-fg-muted outline-offset-2 outline-attention hover:bg-raised-high focus-visible:outline-3 disabled:opacity-40"
        >
          <UserPlusIcon aria-hidden="true" size={26} strokeWidth={1.75} />
          Add another guest
        </button>
      )}
    </SplitFrame>
  );
}

interface AmountChooserProps {
  /** Which guest the amount being keyed is for, counted from one. */
  guest: number;
  keyed: Amount;
  /** No more guests can be added. */
  full: boolean;
  onKey: Parameters<typeof NumPad>[0]["onKey"];
  onAdd: () => void;
}

function AmountChooser({ guest, keyed, full, onKey, onAdd }: AmountChooserProps) {
  return (
    <div className="flex flex-col gap-3">
      <ColumnHeading>Guest {guest} pays</ColumnHeading>
      <div className="-mt-1 flex min-h-charge items-center justify-end rounded border border-accent bg-raised px-4 outline-3 outline-accent-tint">
        <Money amount={keyed} className="text-xl font-bold" />
      </div>
      <NumPad layout="amount" label={`What guest ${guest} pays`} onKey={onKey} />
      <Button disabled={keyed.minor <= 0 || full} onClick={onAdd}>
        Add guest
      </Button>
      <p className="text-sm text-fg-muted">Whatever is left is one more guest&apos;s.</p>
    </div>
  );
}
