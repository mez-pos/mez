// Ingredients from a spreadsheet (backoffice-stock.md, "Import
// ingredients"): get the template, choose the file filled in, read what it
// would add or what is wrong with it, and add it. Nothing is written until
// Add, and then all of it or none. What is added has nothing in stock.

import { DownloadIcon } from "lucide-react";
import { useState } from "react";
import { ApiError } from "../../../shared/api";
import type { StockItemsImport } from "../../../shared/api-types";
import { Button, Modal, Money, useToast } from "../../../shared/ui";
import { asCsv, fileProblem } from "../files/file-logic";
import { ChooseFile, Problems, Step } from "../files/FileParts";
import { useCsvFile } from "../sales/use-csv-download";
import {
  addIngredientsLabel,
  addedIngredientsText,
  ingredientLine,
  ingredientProblemsText,
  ingredientsSummary,
  ingredientsVerdict,
  itemExamples,
  itemExamplesText,
  itemsTemplatePath,
} from "./import-logic";
import { costMeasures, problemOf } from "./stock-logic";
import type { StockApi } from "./use-stock";

/** A file that has been checked: what it is, and what the server said of it. */
interface Checked {
  file: Blob;
  name: string;
  plan: StockItemsImport;
  /** Said over the plan: why it is being shown again. */
  note: string | null;
}

type Busy = { kind: "checking"; name: string } | { kind: "adding" } | null;

const changedNote = "The store changed since this file was checked. This is how it stands now.";

interface ItemsImportModalProps {
  api: StockApi;
  onClose: () => void;
}

export function ItemsImportModal({ api, onClose }: ItemsImportModalProps) {
  const toast = useToast();
  const [checked, setChecked] = useState<Checked | null>(null);
  const [busy, setBusy] = useState<Busy>(null);
  const [problem, setProblem] = useState<string | null>(null);

  const check = async (file: Blob, name: string, note: string | null) => {
    setBusy({ kind: "checking", name });
    setProblem(null);
    try {
      setChecked({ file, name, plan: await api.previewItems(file), note });
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
      const done = await api.importItems(c.file);
      if (done.imported) {
        toast(addedIngredientsText(done), { tone: "success" });
        onClose();
        return;
      }
      // Nothing was new by the time it was sent: say so, as the server put it.
      setChecked({ ...c, plan: done, note: changedNote });
    } catch (err) {
      // Refused: the store no longer fits the file. Check it again to say how.
      if (err instanceof ApiError && err.code === "stock_import_refused") {
        await check(c.file, c.name, changedNote);
        return;
      }
      setProblem(problemOf(err).message);
    } finally {
      setBusy((b) => (b?.kind === "adding" ? null : b));
    }
  };

  const ready = checked !== null && busy === null && ingredientsVerdict(checked.plan) === "ready";
  return (
    <Modal
      open
      onClose={onClose}
      title="Import ingredients"
      size="lg"
      footer={
        <>
          <Button onClick={onClose}>{checked ? "Cancel" : "Close"}</Button>
          {ready && (
            <Button variant="primary" onClick={() => void add(checked)}>
              {addIngredientsLabel(checked.plan)}
            </Button>
          )}
        </>
      }
    >
      <div className="flex flex-col gap-4">
        {busy ? (
          <p role="status">{busy.kind === "checking" ? `Checking ${busy.name}` : "Adding the ingredients"}</p>
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

function Steps({ onChoose }: { onChoose: (file: File | undefined) => void }) {
  const template = useCsvFile();
  return (
    <>
      <p>Add many ingredients at once from a spreadsheet. It adds them: it never changes or removes one that is already in the store.</p>
      <ol className="flex flex-col gap-4">
        <Step n={1}>
          <p>Get the template.</p>
          <Button disabled={template.busy !== null} onClick={() => void template.download(itemsTemplatePath)}>
            <DownloadIcon aria-hidden="true" size={17} />
            {template.busy ? "Downloading" : "Download template"}
          </Button>
        </Step>
        <Step n={2}>
          <p>
            Fill it in, one ingredient on each row, and save it as CSV UTF-8. Counted in is g, ml or each. Low at is in that unit. Cost is what you pay for a
            kilogram, a litre or one.
          </p>
        </Step>
        <Step n={3}>
          <p>Choose the file. You see what it would add before anything is added.</p>
          <ChooseFile label="Choose a file" onChoose={onChoose} />
        </Step>
      </ol>
    </>
  );
}

function Result({ checked, onChoose }: { checked: Checked; onChoose: (file: File | undefined) => void }) {
  const { plan, name, note } = checked;
  const verdict = ingredientsVerdict(plan);
  const examples = itemExamples(plan);
  return (
    <>
      {note && <p className="rounded bg-attention-tint p-3.5">{note}</p>}
      {verdict === "problems" && <Problems line={ingredientProblemsText(plan, name)} problems={plan.problems} onChoose={onChoose} />}
      {verdict === "nothing" && (
        <>
          <p>Every ingredient in {name} is already in the store.</p>
          <div>
            <ChooseFile label="Choose another file" onChoose={onChoose} />
          </div>
        </>
      )}
      {verdict === "ready" && (
        <>
          <ul className="flex flex-col gap-1">
            {ingredientsSummary(plan).map((line, i) => (
              <li key={line} className={i === 0 ? "font-serif text-xl text-fg-heading" : undefined}>
                {line}
              </li>
            ))}
          </ul>
          {examples > 0 && <p className="rounded bg-attention-tint p-3.5">{itemExamplesText(examples)}</p>}
          <ul aria-label="Ingredients to add" className="max-h-64 overflow-y-auto rounded border border-line">
            {plan.to_add.map((r) => (
              <li key={r.row} className="flex items-center gap-3 border-b border-line-soft px-3 py-2 last:border-b-0">
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-semibold">{r.name}</span>
                  <span className="block truncate text-sm text-fg-muted">{ingredientLine(r)}</span>
                </span>
                {r.cost.minor === 0 ? (
                  <span className="shrink-0 text-sm text-fg-muted">Not costed</span>
                ) : (
                  <span className="shrink-0 whitespace-nowrap">
                    <Money amount={r.cost} /> <span className="text-sm text-fg-muted">/ {costMeasures[r.unit].short}</span>
                  </span>
                )}
              </li>
            ))}
          </ul>
          <p className="text-sm text-fg-muted">They are added with nothing in stock. Use Receive when a delivery comes in.</p>
          <div>
            <ChooseFile label="Choose another file" onChoose={onChoose} />
          </div>
        </>
      )}
    </>
  );
}
