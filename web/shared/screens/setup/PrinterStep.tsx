// The wizard's printer step (setup-wizard.md, ADR-268): look for printers,
// pick one, print a test page on it. The owner is signed in by now, so this is
// the printers API the back office uses. It can always be skipped: a venue
// with no printer yet still opens.

import { ArrowRightIcon, PrinterIcon, SearchIcon } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import type { ApiClient } from "../../api";
import type { DiscoverResponse } from "../../api-types";
import { Button } from "../../ui";
import { cx } from "../../ui/cx";
import { CardChoice, TextField } from "./fields";
import {
  addAndTest,
  addressProblem,
  choiceKey,
  choicesOf,
  forget,
  printerError,
  searchedWords,
  typedChoice,
  watchTest,
  type PaperMM,
  type PrinterChoice,
  type TestOutcome,
} from "./printer-logic";
import { radioGroup } from "../../ui/radio-group";

/** The key of the choice that is "enter an address", which no found printer can have. */
const typedKey = "typed";

type Search = { state: "searching" } | { state: "found"; found: DiscoverResponse } | { state: "failed"; message: string };

const papers: readonly { value: `${PaperMM}`; label: string; blurb: string }[] = [
  { value: "80", label: "80mm", blurb: "The wide roll most counters use." },
  { value: "58", label: "58mm", blurb: "The narrow roll of a small or handheld printer." },
];

export function PrinterStep({ api, onDone }: { api: ApiClient; onDone: (kept: boolean) => void }) {
  const host = window.location.host;
  const [search, setSearch] = useState<Search>({ state: "searching" });
  const [picked, setPicked] = useState<string | null>(null);
  const [address, setAddress] = useState("");
  const [paper, setPaper] = useState<PaperMM>(80);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<{ message: string; address: boolean } | null>(null);
  const [outcome, setOutcome] = useState<TestOutcome | null>(null);
  // The printer an earlier try added, so a second choice replaces it and does not join it.
  const added = useRef<number | null>(null);
  const watching = useRef<AbortController | null>(null);

  const look = useCallback(async () => {
    setSearch({ state: "searching" });
    try {
      const found = await api.post<DiscoverResponse>("/api/v1/printers/discover", undefined, { timeoutMs: 25_000 });
      setSearch({ state: "found", found });
      // One find is the likely answer: it is picked, and the owner still has to press the button.
      const choices = choicesOf(found);
      setPicked((now) => now ?? (choices.length === 1 && choices[0] ? choiceKey(choices[0]) : null));
    } catch (err) {
      setSearch({ state: "failed", message: printerError(err, host).message });
    }
  }, [api, host]);

  useEffect(() => {
    void look();
    return () => watching.current?.abort();
  }, [look]);

  const choices = search.state === "found" ? choicesOf(search.found) : [];
  const chosen: PrinterChoice | null = picked === typedKey ? typedChoice(address) : (choices.find((c) => choiceKey(c) === picked) ?? null);

  const pick = (key: string) => {
    setPicked(key);
    setProblem(null);
    setOutcome(null);
  };

  const print = async () => {
    if (!chosen || busy) return;
    if (picked === typedKey) {
      const wrong = addressProblem(address);
      if (wrong) {
        setProblem({ message: wrong, address: true });
        return;
      }
    }
    setBusy(true);
    setProblem(null);
    setOutcome(null);
    try {
      const sent = await addAndTest(api, chosen, paper, added.current);
      added.current = sent.printerId;
      watching.current?.abort();
      watching.current = new AbortController();
      const result = await watchTest(api, sent, watching.current.signal);
      if (!watching.current.signal.aborted) setOutcome(result);
    } catch (err) {
      setProblem(printerError(err, host));
    } finally {
      setBusy(false);
    }
  };

  // Leaving with no page printed leaves no printer behind (printer-logic, forget).
  const leaveWithout = async () => {
    setBusy(true);
    watching.current?.abort();
    await forget(api, added.current);
    onDone(false);
  };

  return (
    <div className="flex flex-1 flex-col">
      <h2 className="font-serif text-xl">Receipt printer</h2>
      <p className="mt-1 text-sm text-fg-muted">
        Pick the printer receipts come out of and print a test page. Kitchen printers, and any others, are added in the back office.
      </p>

      <div className="mt-5 flex flex-col gap-5">
        <div className="flex flex-col gap-1.5">
          <div className="flex flex-wrap items-center gap-3">
            <p className="text-base font-semibold">Printers found</p>
            <Button size="md" className="ml-auto" disabled={search.state === "searching" || busy} onClick={() => void look()}>
              <SearchIcon aria-hidden="true" size={16} />
              {search.state === "searching" ? "Searching" : "Search again"}
            </Button>
          </div>
          <div role="radiogroup" {...radioGroup} aria-label="Printers found" aria-busy={search.state === "searching"} className="flex flex-col gap-2">
            {search.state === "searching" && (
              <p role="status" className="rounded border border-line bg-sunken px-4 py-3 text-base text-fg-2">
                Looking on this network and this computer. It takes a few seconds.
              </p>
            )}
            {search.state === "failed" && (
              <p role="alert" className="rounded border border-danger-line bg-danger-tint px-4 py-3 text-base text-danger">
                {search.message}
              </p>
            )}
            {search.state === "found" && choices.length === 0 && (
              <p className="rounded border border-line bg-sunken px-4 py-3 text-base text-fg-2">
                No printer was found. Check that it is switched on and on this network, or plugged into this computer with its driver installed, then search
                again.
              </p>
            )}
            {choices.map((c) => (
              <Option
                key={choiceKey(c)}
                chosen={picked === choiceKey(c)}
                onPick={() => pick(choiceKey(c))}
                label={c.label}
                detail={c.known ? `${c.detail} · already added` : c.detail}
                mono={c.transport === "tcp"}
              />
            ))}
            <Option
              chosen={picked === typedKey}
              onPick={() => pick(typedKey)}
              label="Enter its address"
              detail="For a network printer the search did not find."
            />
          </div>
          {search.state === "found" && <p className="text-sm text-fg-muted">{searchedWords(search.found)}</p>}
        </div>

        {picked === typedKey && (
          <TextField
            label="Printer's address"
            value={address}
            onChange={(v) => {
              setAddress(v);
              setProblem(null);
            }}
            error={problem?.address ? problem.message : undefined}
            hint="Its IP address on this network. Most receipt printers print it on a self-test page: hold the feed button while switching the printer on."
            placeholder="192.168.1.50"
            inputMode="decimal"
            className="font-mono"
            autoFocus
          />
        )}

        {chosen && <CardChoice label="Paper" value={`${paper}`} onChange={(v) => setPaper(v === "58" ? 58 : 80)} options={papers} min="14rem" />}

        {problem && !problem.address && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
        {busy && (
          <p role="status" className="text-base text-fg-2">
            Printing a test page
          </p>
        )}
        {outcome && !busy && <Outcome outcome={outcome} />}
      </div>

      <div className="mt-auto flex flex-wrap items-center gap-3 pt-6">
        <p className="text-sm text-fg-muted">You can add or change printers later in the back office.</p>
        <div className="ml-auto flex flex-wrap gap-3">
          {outcome?.kind === "sent" ? (
            <>
              {/* A page that never came out is not a dead end (SET-5): the printer is left behind, as Skip for now does. */}
              <Button disabled={busy} onClick={() => void leaveWithout()}>
                Nothing came out, skip
              </Button>
              <Button disabled={busy} onClick={() => void print()}>
                Print again
              </Button>
              <Button variant="primary" className="min-w-44" onClick={() => onDone(true)}>
                It printed
                <ArrowRightIcon aria-hidden="true" size={18} strokeWidth={2.2} />
              </Button>
            </>
          ) : (
            <>
              <Button disabled={busy} onClick={() => void leaveWithout()}>
                {outcome ? "Continue without it" : "Skip for now"}
              </Button>
              <Button variant="primary" className="min-w-44" disabled={!chosen || busy} onClick={() => void print()}>
                <PrinterIcon aria-hidden="true" size={18} />
                {outcome ? "Try again" : "Print a test page"}
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function Option({ chosen, onPick, label, detail, mono }: { chosen: boolean; onPick: () => void; label: string; detail: string; mono?: boolean }) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={chosen}
      onClick={onPick}
      className={cx(
        "flex min-h-tap items-center gap-3 rounded border px-4 py-2.5 text-left outline-offset-2 outline-focus focus-visible:outline-3",
        chosen ? "border-accent bg-accent-tint" : "border-line-strong bg-raised hover:bg-raised-high",
      )}
    >
      <span
        aria-hidden="true"
        className={cx("flex size-5 shrink-0 items-center justify-center rounded-round border-2", chosen ? "border-accent" : "border-line-strong")}
      >
        {chosen && <span className="size-2.5 rounded-round bg-accent" />}
      </span>
      <span className="flex min-w-0 flex-col">
        <span className={cx("truncate text-base font-semibold", chosen && "text-accent-text", mono && "tabular-nums")}>{label}</span>
        <span className="text-sm text-fg-2">{detail}</span>
      </span>
    </button>
  );
}

/** What became of the test page: a word and a tone, never a colour alone. */
function Outcome({ outcome }: { outcome: TestOutcome }) {
  if (outcome.kind === "sent") {
    return (
      <div role="status" className="rounded border border-chip-success-border bg-chip-success-bg px-4 py-3 text-chip-success-fg">
        <p className="text-base font-semibold">Sent. Did a page come out?</p>
        <p className="text-sm">
          {outcome.message} A printer cannot say that it printed, so look for the page. If nothing came out, this may not be the printer: pick another.
        </p>
      </div>
    );
  }
  return (
    <div role="alert" className="rounded border border-chip-warning-border bg-chip-warning-bg px-4 py-3 text-chip-warning-fg">
      <p className="text-base font-semibold">{outcome.kind === "failed" ? "Not printed" : "No answer yet"}</p>
      <p className="text-sm">{outcome.message}</p>
    </div>
  );
}
