// The staff lock: tap your name, enter your PIN (docs/design/screens/terminal-lock.md).
// The first screen anyone sees on a shift, on the terminal (ink) and as the
// back office sign-in (paper).

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useCallback, useEffect, useState } from "react";
import type { StaffListResponse, StaffResponse } from "../api-types";
import { formatWait, initials } from "../auth/format";
import { signInError, unreachableMessage } from "../auth/messages";
import { useAuth, useAuthStore } from "../auth/use-auth";
import { ConnectionIndicator } from "../ConnectionIndicator";
import { applyPinKey, Button, EmptyState, InboxIcon, Logo, Meta, NumPad, type NumKey } from "../ui";
import { cx } from "../ui/cx";
import { keyFromKeyboard } from "../ui/numpad-keys";
import { minPin, PinDots } from "./PinDots";

const maxPin = 8;

export function LockScreen() {
  const status = useAuth();
  const store = useAuthStore();
  const [selected, setSelected] = useState<StaffResponse | null>(null);
  const [pin, setPin] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [lockedUntil, setLockedUntil] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  const now = useNow(lockedUntil !== null);
  const locked = lockedUntil !== null && lockedUntil > now;

  // Read afresh every time the lock screen shows: it is where someone is
  // about to sign in, and the back office may have just added, renamed or
  // deactivated them (ADR-115). A cached list hid a new person until reload.
  const staff = useQuery({
    queryKey: ["staff"],
    queryFn: () => store.api.get<StaffListResponse>("/api/v1/staff").then((r) => r.staff),
    refetchOnMount: "always",
  });

  // One name on the list: nothing to choose, so it is chosen.
  useEffect(() => {
    if (staff.data?.length === 1 && !selected) setSelected(staff.data[0] ?? null);
  }, [staff.data, selected]);

  useEffect(() => {
    if (lockedUntil !== null && lockedUntil <= now) {
      setLockedUntil(null);
      setError(null);
    }
  }, [lockedUntil, now]);

  const canType = selected !== null && !locked && !busy;
  const canSubmit = canType && pin.length >= minPin;

  const submit = useCallback(async () => {
    if (!canSubmit || !selected) return;
    setBusy(true);
    setError(null);
    try {
      await store.signIn(selected.id, pin);
    } catch (err) {
      const failure = signInError(err, window.location.host, Date.now());
      setError(failure.message);
      setLockedUntil(failure.lockedUntil);
      setPin("");
      setBusy(false);
    }
  }, [canSubmit, selected, store, pin]);

  const key = useCallback(
    (k: NumKey) => {
      if (!canType) return;
      setError((e) => (locked ? e : null));
      setPin((p) => applyPinKey(p, k, maxPin));
    },
    [canType, locked],
  );

  // A keyboard anywhere on the screen types into the PIN; the pad itself
  // handles keys when it has focus, and says so with defaultPrevented.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.target instanceof HTMLInputElement) return;
      if (event.key === "Enter") {
        event.preventDefault();
        void submit();
        return;
      }
      const k = keyFromKeyboard(event.key);
      if (k) {
        event.preventDefault();
        key(k);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [key, submit]);

  const choose = (member: StaffResponse) => {
    setSelected(member);
    setPin("");
    if (!locked) setError(null);
  };

  const device = status.kind === "locked" || status.kind === "signed_in" ? status.device : null;
  const notice = status.kind === "locked" && status.notice === "expired" ? "Your session ended. Sign in again to carry on." : null;

  return (
    <main className="relative flex min-h-screen items-center justify-center bg-canvas p-6 text-fg lg:p-8">
      <div className="absolute right-4 top-4">
        <ConnectionIndicator />
      </div>

      <div className="grid w-full max-w-4xl items-center gap-8 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <section aria-labelledby="lock-title">
          <Logo className="mb-6 h-12" />
          <Meta items={["Sign in"]} />
          <h1 id="lock-title" className="mt-3 font-serif text-total">
            {device?.name ?? "This device"}
          </h1>
          <p className="mt-3 max-w-md text-base text-fg-2">Tap your name, then enter your PIN.</p>
          {notice && (
            <p role="status" className="mt-4 max-w-md rounded border border-chip-warning-border bg-chip-warning-bg px-4 py-3 text-base text-chip-warning-fg">
              {notice}
            </p>
          )}
          <StaffTiles query={staff} selected={selected} onChoose={choose} />
        </section>

        <section aria-label="PIN" className="rounded-card border border-line bg-chrome p-6 shadow-card">
          <p className="min-h-6 text-base font-semibold">{selected ? selected.name : "Choose your name"}</p>
          <PinDots length={pin.length} />
          <NumPad layout="pin" size="lg" label="PIN" onKey={key} disabled={!canType} className="mt-4" />
          <Button variant="primary" className="mt-4 w-full" disabled={!canSubmit} onClick={() => void submit()}>
            {busy ? "Signing in…" : "Sign in"}
          </Button>
          <p role="alert" className="mt-3 min-h-6 text-base text-danger">
            {error}
            {locked && lockedUntil !== null && ` Try again in ${formatWait(lockedUntil - now)}.`}
          </p>
          <p className="mt-1 text-sm text-fg-muted">Forgot your PIN? Ask a manager to reset it.</p>
        </section>
      </div>
    </main>
  );
}

interface StaffTilesProps {
  query: UseQueryResult<StaffResponse[]>;
  selected: StaffResponse | null;
  onChoose: (member: StaffResponse) => void;
}

function StaffTiles({ query, selected, onChoose }: StaffTilesProps) {
  if (query.isPending) return <p className="mt-6 text-base text-fg-muted">Loading staff&hellip;</p>;
  if (query.isError) {
    return (
      <div role="alert" className="mt-6 flex max-w-md flex-col items-start gap-3">
        <p className="text-base text-danger">{unreachableMessage(window.location.host)}</p>
        <Button onClick={() => void query.refetch()}>Try again</Button>
      </div>
    );
  }
  if (query.data.length === 0) {
    return <EmptyState icon={<InboxIcon />} line="Nobody can sign in here yet. Add staff in the back office." className="mt-6 items-start px-0 text-left" />;
  }
  return (
    <ul className="mt-6 grid max-w-xl grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-2.5">
      {query.data.map((member) => {
        const chosen = selected?.id === member.id;
        return (
          <li key={member.id}>
            <button
              type="button"
              aria-pressed={chosen}
              onClick={() => onChoose(member)}
              className={cx(
                "flex min-h-tap w-full items-center gap-3 rounded border px-3.5 py-2.5 text-left",
                "outline-offset-2 outline-line-focus focus-visible:outline-2",
                chosen ? "border-chip-accent-border bg-chip-accent-bg" : "border-line bg-raised hover:border-line-strong",
              )}
            >
              <span aria-hidden="true" className="flex size-9 shrink-0 items-center justify-center rounded-round border border-line-strong bg-raised-high text-sm font-semibold">
                {initials(member.name)}
              </span>
              <span className="min-w-0 truncate text-base">{member.name}</span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

// The clock, once a second, while a lockout countdown is showing.
function useNow(ticking: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!ticking) return;
    setNow(Date.now());
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [ticking]);
  return now;
}
