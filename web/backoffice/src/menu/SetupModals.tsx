// The two forms of the Stations and courses view (backoffice-menu.md): a name
// (and, for a new station, a code) and where a retired row's items go.

import { useState } from "react";
import { Button, Modal } from "../../../shared/ui";
import { Field, inputClass } from "../FormFields";
import {
  destinations,
  itemsText,
  nameProblem,
  retireProblem,
  setupProblemOf,
  type SetupField,
  type SetupKind,
  type SetupRow,
} from "./setup-logic";
import type { Problem } from "../settings/problem";

interface NameModalProps {
  title: string;
  initial: string;
  /** A new station may be given a code; any other form shows none, or the one it has. */
  code: { mode: "choose" } | { mode: "fixed"; value: string } | null;
  onSave: (name: string, code: string) => Promise<void>;
  onClose: () => void;
}

export function NameModal({ title, initial, code, onSave, onClose }: NameModalProps) {
  const [name, setName] = useState(initial);
  const [chosen, setChosen] = useState("");
  const [problem, setProblem] = useState<Problem<SetupField> | null>(null);
  const [busy, setBusy] = useState(false);
  const save = async () => {
    const refused = nameProblem(name);
    if (refused) {
      setProblem(refused);
      return;
    }
    setBusy(true);
    try {
      await onSave(name.trim(), chosen.trim());
      onClose();
    } catch (err) {
      setProblem(setupProblemOf(err));
      setBusy(false);
    }
  };
  const on = (field: SetupField) => (problem?.field === field ? problem.message : null);

  return (
    <Modal
      open
      onClose={onClose}
      title={title}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={busy} onClick={() => void save()}>
            {busy ? "Saving" : "Save"}
          </Button>
        </>
      }
    >
      <form
        className="flex flex-col gap-5"
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          if (!busy) void save();
        }}
      >
        {problem && problem.field === null && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
        <Field label="Name" error={on("name")}>
          {(id) => (
            <input
              id={id}
              className={inputClass}
              value={name}
              maxLength={60}
              autoComplete="off"
              onChange={(e) => {
                setName(e.target.value);
                setProblem(null);
              }}
            />
          )}
        </Field>
        {code?.mode === "choose" && (
          <Field
            label="Code"
            hint="Short and lowercase, like bar. Empty makes one from the name. It cannot be changed later: tickets and screens find the station by it."
            error={on("code")}
          >
            {(id) => (
              <input
                id={id}
                className={`${inputClass} max-w-48 font-mono`}
                value={chosen}
                maxLength={32}
                autoComplete="off"
                spellCheck={false}
                onChange={(e) => {
                  setChosen(e.target.value);
                  setProblem(null);
                }}
              />
            )}
          </Field>
        )}
        {code?.mode === "fixed" && (
          <p className="text-sm text-fg-muted">
            Its code, <span className="font-mono">{code.value}</span>, never changes.
          </p>
        )}
      </form>
    </Modal>
  );
}

interface RetireModalProps {
  kind: SetupKind;
  row: SetupRow;
  /** Every row of its list: the ones in service, apart from itself, can take what it carries. */
  rows: readonly SetupRow[];
  /** Retires it, its items going to the row with this id (0 for none). */
  onRetire: (moveTo: number) => Promise<void>;
  onClose: () => void;
}

export function RetireModal({ kind, row, rows, onRetire, onClose }: RetireModalProps) {
  const options = destinations(rows, row.id);
  // A course with no items has nowhere to send anything, and asks nothing.
  const asks = row.items > 0 || kind === "station";
  const [moveTo, setMoveTo] = useState(0);
  const [problem, setProblem] = useState<Problem<SetupField> | null>(null);
  const [busy, setBusy] = useState(false);
  const onlyOne = options.length === 0;

  const confirm = async () => {
    const refused = retireProblem(row, moveTo, kind);
    if (refused) {
      setProblem(refused);
      return;
    }
    setBusy(true);
    try {
      await onRetire(moveTo);
      onClose();
    } catch (err) {
      setProblem(setupProblemOf(err));
      setBusy(false);
    }
  };

  return (
    <Modal
      open
      onClose={onClose}
      title={`Retire ${row.name}`}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="danger" disabled={busy || onlyOne} onClick={() => void confirm()}>
            {busy ? "Retiring" : `Retire ${kind}`}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        {onlyOne ? (
          <p className="text-base">This is the only {kind} in service. Add another before retiring it.</p>
        ) : (
          <>
            <p className="text-base">
              {kind === "station"
                ? "It stops taking new items and tickets. If it prints on a printer, the printer goes to the station you choose when that one has none."
                : "It leaves the till. Tickets already rung up keep the course they copied."}{" "}
              {row.items > 0 ? `${itemsText(row.items)} ${row.items === 1 ? "is" : "are"} ${kind === "station" ? "made" : "served"} here.` : ""}
            </p>
            {asks && (
              <Field
                label={row.items > 0 ? `Move ${row.items === 1 ? "it" : `its ${row.items} items`} to` : "Its printer, if it has one, goes to"}
                {...(row.items === 0 ? { hint: "Nothing is made here. Leave this empty if it has no printer." } : {})}
                error={problem?.field === "move_to" ? problem.message : null}
              >
                {(id) => (
                  <select
                    id={id}
                    className={inputClass}
                    value={moveTo}
                    disabled={busy}
                    onChange={(e) => {
                      setMoveTo(Number(e.target.value));
                      setProblem(null);
                    }}
                  >
                    <option value={0}>{row.items > 0 ? `Choose a ${kind}` : `No ${kind}`}</option>
                    {options.map((o) => (
                      <option key={o.id} value={o.id}>
                        {o.name}
                      </option>
                    ))}
                  </select>
                )}
              </Field>
            )}
          </>
        )}
        {problem && problem.field !== "move_to" && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
      </div>
    </Modal>
  );
}
