// A closed shift's tips (docs/design/screens/tips.md): who shares them,
// their hours when the rule needs them, each share as the server works it
// out, and pooling once. Every share on screen is the server's.

import { useState } from "react";
import type { TipPoolResponse } from "../../../shared/api-types";
import { Button, Confirm, EmptyState, InboxIcon, Meta, Money, useToast } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { AtTheVenue } from "../../../shared/shell/AtTheVenue";
import { useAway } from "../../../shared/use-realtime";
import { useVenueClock } from "../../../shared/use-venue";
import { formatMoney } from "../../../shared/money";
import { clockText } from "../../../shared/zreport/zreport-logic";
import { membersOf, ruleWords, startingRows, tipProblem, type Row } from "./tips-logic";
import { useTipPreview, useTips } from "./use-tips";
import { fieldClass } from "../../../shared/ui/field";

interface TipsScreenProps {
  shiftId: number;
  onBack: () => void;
}

export function TipsScreen({ shiftId, onBack }: TipsScreenProps) {
  const { pool, staff, commit } = useTips(shiftId);
  const away = useAway();
  const back = (
    <Button variant="ghost" onClick={onBack}>
      The Z-report
    </Button>
  );
  if (pool.isPending || staff.isPending) return <p className="text-base text-fg-muted">Loading the tips</p>;
  if (pool.isError || staff.isError) {
    return (
      <div className="flex flex-col items-start gap-4">
        <EmptyState
          icon={<InboxIcon />}
          line={tipProblem(pool.error ?? staff.error, "even")}
          action={{ label: "Try again", onClick: () => void (pool.refetch(), staff.refetch()) }}
        />
        {back}
      </div>
    );
  }
  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Tips</h1>
          <Meta className="mt-1" items={[ruleWords(pool.data.rule)]} />
        </div>
        {back}
      </header>
      <p className="flex items-baseline justify-between border-b border-line pb-3">
        <span className="font-serif text-xl">In tips</span>
        <Money amount={pool.data.total} variant="total" />
      </p>
      {pool.data.total.minor === 0 && <p className="text-base text-fg-muted">No tips were taken in this shift.</p>}
      {pool.data.pooled ? (
        <Pooled pool={pool.data} />
      ) : away ? (
        <AtTheVenue>Tips are pooled at the venue.</AtTheVenue>
      ) : (
        <Choose shiftId={shiftId} pool={pool.data} staffList={staff.data} commit={commit} />
      )}
    </div>
  );
}

interface ChooseProps {
  shiftId: number;
  pool: TipPoolResponse;
  staffList: Parameters<typeof startingRows>[1];
  commit: ReturnType<typeof useTips>["commit"];
}

function Choose({ shiftId, pool, staffList, commit }: ChooseProps) {
  const [rows, setRows] = useState<Row[]>(() => startingRows(pool, staffList));
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();
  const members = membersOf(rows);
  const preview = useTipPreview(shiftId, members, true);
  // A change to who is in, or their hours, is a new try: the last refusal goes.
  const set = (i: number, next: Partial<Row>) => {
    setRows((rs) => rs.map((r, j) => (j === i ? { ...r, ...next } : r)));
    setError(null);
  };
  const shareOf = (id: string) => preview.data?.members.find((m) => m.staff_id === id && m.in_pool)?.share;
  const tookOf = (id: string) => pool.members.find((m) => m.staff_id === id)?.taken ?? { ...pool.total, minor: 0 };
  const hours = pool.rule === "hours";
  const count = members?.length ?? 0;
  const problem =
    members === null
      ? "Hours are whole hours and 0 to 59 minutes."
      : count === 0
        ? "Put at least one person in the pool."
        : preview.isError
          ? tipProblem(preview.error, pool.rule)
          : null;

  const pool_ = async () => {
    if (!members) return;
    setBusy(true);
    try {
      await commit(members);
      toast("The tips are pooled.", { tone: "success" });
    } catch (err) {
      setError(tipProblem(err, pool.rule));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section aria-label="Who shares the tips" className="flex flex-col gap-4">
      <table className="w-full text-base">
        <thead>
          <tr className="text-left text-sm text-fg-muted">
            <th className="py-1 font-normal">Name</th>
            <th className="py-1 font-normal">In the pool</th>
            {hours && <th className="py-1 font-normal">Hours worked</th>}
            <th className="py-1 text-right font-normal">Took</th>
            <th className="py-1 text-right font-normal">Share</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => {
            const share = shareOf(r.staffId);
            return (
              <tr key={r.staffId} className="border-t border-line">
                <td className="py-2 pr-3">{r.name}</td>
                <td className="py-2 pr-3">
                  <button
                    type="button"
                    aria-pressed={r.inPool}
                    onClick={() => set(i, { inPool: !r.inPool })}
                    className={cx(
                      "min-h-tap min-w-20 rounded-pill border px-4 font-semibold outline-offset-2 outline-focus focus-visible:outline-3",
                      r.inPool ? "border-accent bg-accent-tint text-accent-text" : "border-line-strong bg-raised text-fg-muted",
                    )}
                  >
                    {r.inPool ? "In" : "Out"}
                  </button>
                </td>
                {hours && (
                  <td className="py-2 pr-3">
                    {r.inPool && (
                      <span className="flex items-center gap-2">
                        <TimeField label={`${r.name}'s hours`} value={r.hours} onChange={(v) => set(i, { hours: v })} />h
                        <TimeField label={`${r.name}'s minutes`} value={r.minutes} onChange={(v) => set(i, { minutes: v })} />m
                      </span>
                    )}
                  </td>
                )}
                <td className="py-2 text-right">
                  <Money amount={tookOf(r.staffId)} tone="muted" />
                </td>
                <td className="py-2 text-right">{r.inPool && share ? <Money amount={share} /> : <span className="font-mono text-fg-muted">–</span>}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {(problem ?? error) && (
        <p role="alert" className="text-base text-danger">
          {error ?? problem}
        </p>
      )}
      <Button variant="primary" size="charge" disabled={busy || problem !== null || !preview.data || preview.isFetching} onClick={() => setConfirming(true)}>
        Pool the tips
      </Button>
      <Confirm
        open={confirming}
        title="Pool the tips"
        confirmLabel="Pool the tips"
        onClose={() => setConfirming(false)}
        onConfirm={() => {
          setConfirming(false);
          void pool_();
        }}
      >
        <p className="text-base">
          Pool {formatMoney(pool.total)} among {count} {count === 1 ? "person" : "people"}? A pool can't be changed once it's made.
        </p>
      </Confirm>
    </section>
  );
}

function TimeField({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <input
      aria-label={label}
      inputMode="numeric"
      value={value}
      maxLength={2}
      onChange={(e) => onChange(e.target.value.replace(/\D/g, ""))}
      className={cx(fieldClass, "min-h-tap w-14 px-2 text-right font-mono text-base")}
    />
  );
}

function Pooled({ pool }: { pool: TipPoolResponse }) {
  const { timezone } = useVenueClock();
  const hours = pool.rule === "hours";
  const weights = pool.rule === "role_weight";
  return (
    <section aria-label="The pool" className="flex flex-col gap-3">
      <table className="w-full text-base">
        <thead>
          <tr className="text-left text-sm text-fg-muted">
            <th className="py-1 font-normal">Name</th>
            <th className="py-1 font-normal">Role</th>
            {hours && <th className="py-1 text-right font-normal">Hours</th>}
            {weights && <th className="py-1 text-right font-normal">Weight</th>}
            <th className="py-1 text-right font-normal">Took</th>
            <th className="py-1 text-right font-normal">Share</th>
          </tr>
        </thead>
        <tbody>
          {pool.members.map((m) => (
            <tr key={m.staff_id} className="border-t border-line">
              <td className="py-2 pr-3">
                {m.name}
                {!m.in_pool && <span className="ml-2 text-sm text-fg-muted">not in the pool</span>}
              </td>
              <td className="py-2 pr-3 text-fg-2">{m.role}</td>
              {hours && (
                <td className="py-2 text-right font-mono tabular-nums">
                  {Math.trunc(m.minutes / 60)}h {String(m.minutes % 60).padStart(2, "0")}m
                </td>
              )}
              {weights && <td className="py-2 text-right font-mono tabular-nums">{m.weight}</td>}
              <td className="py-2 text-right">
                <Money amount={m.taken} tone="muted" />
              </td>
              <td className="py-2 text-right">
                <Money amount={m.share} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="text-sm text-fg-muted">Pooled{pool.pooled_at ? ` at ${clockText(pool.pooled_at, timezone)}` : ""}. It can't be changed.</p>
    </section>
  );
}
