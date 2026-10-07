// The menu from a spreadsheet (backoffice-menu.md, "Import"): get the
// template, choose the file filled in, read what it would add or what is
// wrong with it, and add it. Nothing is written until Add, and then all of
// it or none.

import { DownloadIcon } from "lucide-react";
import { useId, useState } from "react";
import { ApiError } from "../../../shared/api";
import type { MenuImport } from "../../../shared/api-types";
import { Button, Modal, Money, useToast } from "../../../shared/ui";
import { useCsvFile } from "../sales/use-csv-download";
import {
  addLabel,
  addedText,
  asCsv,
  examplesIn,
  examplesText,
  fileProblem,
  problemsText,
  summaryLines,
  templatePath,
  verdictOf,
  whereOf,
} from "./import-logic";
import { problemOf } from "./menu-logic";
import type { MenuEditorApi } from "./use-menu-editor";

/** A file that has been checked: what it is, and what the server said of it. */
interface Checked {
  file: Blob;
  name: string;
  plan: MenuImport;
  /** Said over the plan: why it is being shown again. */
  note: string | null;
}

type Busy = { kind: "checking"; name: string } | { kind: "adding" } | null;

const changedNote = "The menu changed since this file was checked. This is how it stands now.";

interface ImportModalProps {
  api: MenuEditorApi;
  onClose: () => void;
}

export function ImportModal({ api, onClose }: ImportModalProps) {
  const toast = useToast();
  const [checked, setChecked] = useState<Checked | null>(null);
  const [busy, setBusy] = useState<Busy>(null);
  const [problem, setProblem] = useState<string | null>(null);

  const check = async (file: Blob, name: string, note: string | null) => {
    setBusy({ kind: "checking", name });
    setProblem(null);
    try {
      setChecked({ file, name, plan: await api.previewImport(file), note });
    } catch (err) {
      setChecked(null);
      setProblem(problemOf(err).message);
    } finally {
      setBusy(null);
    }
  };
  const choose = (file: File | undefined) => {
    if (!file) return;
    const wrong = fileProblem(file);
    if (wrong) {
      setChecked(null);
      setProblem(wrong);
      return;
    }
    void check(asCsv(file), file.name, null);
  };
  const add = async (c: Checked) => {
    setBusy({ kind: "adding" });
    setProblem(null);
    try {
      const done = await api.importMenu(c.file);
      if (done.imported) {
        toast(addedText(done), { tone: "success" });
        onClose();
        return;
      }
      // Nothing was new by the time it was sent: say so, as the server put it.
      setChecked({ ...c, plan: done, note: changedNote });
    } catch (err) {
      // Refused: the menu no longer fits the file. Check it again to say how.
      if (err instanceof ApiError && err.code === "menu_import_refused") {
        await check(c.file, c.name, changedNote);
        return;
      }
      setProblem(problemOf(err).message);
    } finally {
      setBusy((b) => (b?.kind === "adding" ? null : b));
    }
  };

  const ready = checked !== null && busy === null && verdictOf(checked.plan) === "ready";
  return (
    <Modal
      open
      onClose={onClose}
      title="Import menu items"
      size="lg"
      footer={
        <>
          <Button onClick={onClose}>{checked ? "Cancel" : "Close"}</Button>
          {ready && (
            <Button variant="primary" onClick={() => void add(checked)}>
              {addLabel(checked.plan)}
            </Button>
          )}
        </>
      }
    >
      <div className="flex flex-col gap-4">
        {busy ? (
          <p role="status">{busy.kind === "checking" ? `Checking ${busy.name}` : "Adding the items"}</p>
        ) : checked ? (
          <Result checked={checked} onChoose={choose} />
        ) : (
          <Steps onChoose={choose} />
        )}
        {problem && (
          <p role="alert" className="text-base text-danger">
            {problem}
          </p>
        )}
      </div>
    </Modal>
  );
}

const chooseClass =
  "inline-flex min-h-tap cursor-pointer items-center rounded border border-line-strong bg-raised-high px-4 text-base font-semibold focus-within:outline-2 focus-within:outline-line-focus";

/** A button that opens the file picker. The same file may be chosen twice: it may have been edited since. */
function ChooseFile({ label, onChoose }: { label: string; onChoose: (file: File | undefined) => void }) {
  const id = useId();
  return (
    <label htmlFor={id} className={chooseClass}>
      {label}
      <input
        id={id}
        type="file"
        accept=".csv,text/csv"
        className="sr-only"
        onChange={(e) => {
          onChoose(e.target.files?.[0]);
          e.target.value = "";
        }}
      />
    </label>
  );
}

const stepNumber = "flex size-8 shrink-0 items-center justify-center rounded-round bg-sunken font-mono text-sm font-bold tabular-nums";

function Steps({ onChoose }: { onChoose: (file: File | undefined) => void }) {
  const template = useCsvFile();
  return (
    <>
      <p>Add many items at once from a spreadsheet. It adds items: it never changes or removes one that is already on the menu.</p>
      <ol className="flex flex-col gap-4">
        <li className="flex items-start gap-3">
          <span aria-hidden="true" className={stepNumber}>
            1
          </span>
          <div className="flex min-w-0 flex-col items-start gap-2">
            <p>Get the template. It has this venue's own category, tax class, station and course names in it.</p>
            <Button disabled={template.busy !== null} onClick={() => void template.download(templatePath)}>
              <DownloadIcon aria-hidden="true" size={17} />
              {template.busy ? "Downloading" : "Download template"}
            </Button>
          </div>
        </li>
        <li className="flex items-start gap-3">
          <span aria-hidden="true" className={stepNumber}>
            2
          </span>
          <p className="pt-1">Fill it in, one item on each row, and save it as CSV UTF-8.</p>
        </li>
        <li className="flex items-start gap-3">
          <span aria-hidden="true" className={stepNumber}>
            3
          </span>
          <div className="flex min-w-0 flex-col items-start gap-2">
            <p className="pt-1">Choose the file. You see what it would add before anything is added.</p>
            <ChooseFile label="Choose a file" onChoose={onChoose} />
          </div>
        </li>
      </ol>
    </>
  );
}

const head = "pb-2 text-left text-meta font-bold uppercase text-fg-muted";

function Result({ checked, onChoose }: { checked: Checked; onChoose: (file: File | undefined) => void }) {
  const { plan, name, note } = checked;
  const verdict = verdictOf(plan);
  const examples = examplesIn(plan);
  return (
    <>
      {note && <p className="rounded bg-attention-tint p-3.5">{note}</p>}
      {verdict === "problems" && (
        <>
          <p className="font-semibold text-danger">{problemsText(plan, name)}</p>
          <div className="max-h-72 overflow-y-auto rounded border border-line">
            <table aria-label="Problems in the file" className="w-full text-base">
              <thead className="sticky top-0 bg-raised">
                <tr className="border-b border-line">
                  <th scope="col" className={`${head} px-3 pt-2.5`}>
                    Where
                  </th>
                  <th scope="col" className={`${head} px-3 pt-2.5`}>
                    Column
                  </th>
                  <th scope="col" className={`${head} px-3 pt-2.5`}>
                    What is wrong
                  </th>
                </tr>
              </thead>
              <tbody>
                {plan.problems.map((p, i) => (
                  <tr key={i} className="border-b border-line-soft align-top last:border-b-0">
                    <td className="whitespace-nowrap px-3 py-2 font-mono tabular-nums">{whereOf(p)}</td>
                    <td className="whitespace-nowrap px-3 py-2 font-semibold">{p.column}</td>
                    <td className="px-3 py-2">{p.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p>Fix them in the file and choose it again.</p>
          <div>
            <ChooseFile label="Choose the file again" onChoose={onChoose} />
          </div>
        </>
      )}
      {verdict === "nothing" && (
        <>
          <p>Every item in {name} is already on the menu.</p>
          <div>
            <ChooseFile label="Choose another file" onChoose={onChoose} />
          </div>
        </>
      )}
      {verdict === "ready" && (
        <>
          <ul className="flex flex-col gap-1">
            {summaryLines(plan).map((line, i) => (
              <li key={line} className={i === 0 ? "font-serif text-xl text-fg-heading" : undefined}>
                {line}
              </li>
            ))}
          </ul>
          {examples > 0 && <p className="rounded bg-attention-tint p-3.5">{examplesText(examples)}</p>}
          <ul aria-label="Items to add" className="max-h-64 overflow-y-auto rounded border border-line">
            {plan.to_add.map((it) => (
              <li key={it.row} className="flex items-center gap-3 border-b border-line-soft px-3 py-2 last:border-b-0">
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-semibold">{it.name}</span>
                  <span className="block truncate text-sm text-fg-muted">{it.category}</span>
                </span>
                <Money amount={it.price} className="shrink-0" />
              </li>
            ))}
          </ul>
          <div>
            <ChooseFile label="Choose another file" onChoose={onChoose} />
          </div>
        </>
      )}
    </>
  );
}
