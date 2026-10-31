// Staff Login (D01): tap your profile, enter your PIN. The first screen anyone
// sees on a shift, on the terminal and as the back office sign-in, one screen for
// everyone; where they land after it depends on their role (ADR-148).

import type { UseQueryResult } from "@tanstack/react-query";
import { CircleHelpIcon } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import type { StaffResponse } from "../api-types";
import { formatWait } from "../auth/format";
import { signInError, unreachableMessage } from "../auth/messages";
import { useAuth, useAuthStore } from "../auth/use-auth";
import { ConnectionIndicator } from "../ConnectionIndicator";
import { applyPinKey, Button, EmptyState, InboxIcon, Logo, type NumKey } from "../ui";
import { keyFromKeyboard } from "../ui/numpad-keys";
import { useClock } from "../use-clock";
import { useStaffList } from "../use-staff-list";
import { useServerVersion, useVenue } from "../use-venue";
import { clockText } from "../venue-text";
import { BrandPanel } from "./login/BrandPanel";
import { LoginPad } from "./login/LoginPad";
import { ProfileTiles } from "./login/ProfileTiles";
import { minPin } from "./PinDots";

const maxPin = 8;
/** What a key pressed before a name says (TILL-22). */
const chooseFirst = "Tap your name first.";

export function LockScreen() {
  const status = useAuth();
  const store = useAuthStore();
  // The person chosen, by id: the list is read afresh, and someone removed or
  // deactivated meanwhile must stop being chosen, not linger as a stale copy.
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [page, setPage] = useState(0);
  const [pin, setPin] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [lockedUntil, setLockedUntil] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  // The clock, once a second, while a lockout countdown is showing.
  const now = useClock(1000, lockedUntil !== null);
  const locked = lockedUntil !== null && lockedUntil > now;

  // Read afresh every time the lock screen shows: it is where someone is
  // about to sign in, and the back office may have just added, renamed or
  // deactivated them (ADR-115). A cached list hid a new person until reload.
  const staff = useStaffList({ refetchOnMount: "always" });

  const people = staff.data;
  const selected = useMemo(() => people?.find((p) => p.id === selectedId) ?? null, [people, selectedId]);

  // One name on the list: nothing to choose, so it is chosen. A person chosen
  // who is no longer on the list is no longer chosen, and what was typed goes.
  useEffect(() => {
    if (!people) return;
    const only = people.length === 1 ? people[0] : undefined;
    if (only && selectedId !== only.id) {
      setSelectedId(only.id);
    } else if (selectedId !== null && !people.some((p) => p.id === selectedId)) {
      setSelectedId(null);
      setPin("");
    }
  }, [people, selectedId]);

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
      // The pad is live before a name is chosen, and says what to do first:
      // dimmed and silent, it looked broken (TILL-22, ADR-381, ADR-393).
      if (selected === null && !locked && !busy) {
        setError(chooseFirst);
        return;
      }
      if (!canType) return;
      setError((e) => (locked ? e : null));
      setPin((p) => applyPinKey(p, k, maxPin));
    },
    [canType, locked, busy, selected],
  );

  // A keyboard anywhere on the screen types into the PIN. Enter signs in, except
  // on a focused button or link, where it is that control's own click: a tile it
  // chooses, More it pages, Try again it retries.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.target instanceof HTMLInputElement) return;
      if (event.key === "Enter") {
        if (event.target instanceof HTMLButtonElement || event.target instanceof HTMLAnchorElement) return;
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
    // The tile keeps the focus of the tap; Enter would then be its click,
    // choosing again and wiping the PIN just typed. Off the tile, Enter
    // signs in.
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    setSelectedId(member.id);
    setPin("");
    if (!locked) setError(null);
  };

  const notice = status.kind === "locked" && status.notice === "expired" ? "Your session ended. Sign in again to carry on." : null;
  const clock = useClock(15_000);
  const venue = useVenue();
  const version = useServerVersion();

  return (
    <main className="flex min-h-screen bg-canvas text-fg">
      <BrandPanel version={version} />

      <section aria-labelledby="lock-title" className="relative flex min-w-0 flex-1 flex-col p-10 short:py-5 2xl:px-18 2xl:py-14">
        {/* The script line sits beside the title, not over it: placed on top, it ran into the subtitle on a narrow column. */}
        <div className="flex items-start justify-between gap-6">
          <div className="min-w-0">
            <Logo className="mb-6 h-10 short:mb-3 xl:hidden" />
            <h1 id="lock-title" className="font-serif text-display leading-tight">
              Staff login
            </h1>
            <p className="mt-2 text-lg text-fg-muted">Select your profile and enter your PIN</p>
          </div>
          <p aria-hidden="true" className="hidden shrink-0 text-right font-script text-total leading-tight text-fg-2 md:block">
            Good Teams,
            <br />
            Great Service ♡
          </p>
        </div>
        {notice && (
          <p
            role="status"
            className="mt-4 max-w-md rounded border border-chip-warning-border bg-chip-warning-bg px-4 py-3 text-base text-chip-warning-fg short:mt-3 short:py-2"
          >
            {notice}
          </p>
        )}
        <StaffPicker query={staff} selected={selected} page={page} onChoose={choose} onPage={setPage} />
        <p className="sr-only" aria-live="polite">
          {selected ? `${selected.name} is chosen. Enter your PIN.` : "Choose your profile."}
        </p>

        <LoginPad
          name={selected?.name ?? null}
          length={pin.length}
          disabled={locked || busy}
          canSubmit={canSubmit}
          busy={busy}
          onKey={key}
          onSubmit={() => void submit()}
        />
        <p role="alert" className="mt-2 min-h-6 text-base text-danger short:mt-1">
          {error}
          {locked && lockedUntil !== null && ` Try again in ${formatWait(lockedUntil - now)}.`}
        </p>
        <p className="flex items-center gap-2 text-sm text-fg-muted">
          <CircleHelpIcon aria-hidden="true" size={17} />
          Forgot your PIN? Ask a manager to reset it.
        </p>

        <footer className="mt-auto flex flex-wrap items-center justify-between gap-x-6 gap-y-2 pt-6 text-sm text-fg-muted short:pt-3">
          <span>{venue?.name ?? ""}</span>
          <ConnectionIndicator />
          <span className="font-mono tabular-nums">{clockText(new Date(clock), venue?.timezone ?? null)}</span>
        </footer>
      </section>
    </main>
  );
}

interface StaffPickerProps {
  query: UseQueryResult<StaffResponse[]>;
  selected: StaffResponse | null;
  page: number;
  onChoose: (member: StaffResponse) => void;
  onPage: (page: number) => void;
}

function StaffPicker({ query, selected, page, onChoose, onPage }: StaffPickerProps) {
  if (query.isPending) return <p className="mt-10 text-base text-fg-muted">Loading staff&hellip;</p>;
  if (query.isError) {
    return (
      <div role="alert" className="mt-10 flex max-w-md flex-col items-start gap-3">
        <p className="text-base text-danger">{unreachableMessage(window.location.host)}</p>
        <Button onClick={() => void query.refetch()}>Try again</Button>
      </div>
    );
  }
  if (query.data.length === 0) {
    return <EmptyState icon={<InboxIcon />} line="Nobody can sign in here yet. Add staff in the back office." className="mt-10 items-start px-0 text-left" />;
  }
  return <ProfileTiles people={query.data} selectedId={selected?.id ?? null} page={page} onChoose={onChoose} onPage={onPage} />;
}
