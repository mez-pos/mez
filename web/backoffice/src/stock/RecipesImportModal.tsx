// Recipes from a spreadsheet (backoffice-stock.md, "Import recipes"): get
// the template, which lists the dishes with no recipe, fill it in, choose
// it, read the recipes it would give or what is wrong with it, and add
// them. It gives a recipe to a dish that has none and changes no other.

import { DownloadIcon } from "lucide-react";
import { useState } from "react";
import { ApiError } from "../../../shared/api";
import type { RecipesImport } from "../../../shared/api-types";
import { Button, Modal, useToast } from "../../../shared/ui";
import { asCsv, fileProblem } from "../files/file-logic";
import { ChooseFile, Problems, Step } from "../files/FileParts";
import { useCsvFile } from "../sales/use-csv-download";
import {
  addRecipesLabel,
  addedRecipesText,
  recipeLine,
  recipeProblemsText,
  recipesLeftOut,
  recipesSummary,
  recipesTemplatePath,
  recipesVerdict,
} from "./import-logic";
import { problemOf } from "./stock-logic";
import type { StockApi } from "./use-stock";

/** A file that has been checked: what it is, and what the server said of it. */
interface Checked {
  file: Blob;
  name: string;
  plan: RecipesImport;
  /** Said over the plan: why it is being shown again. */
  note: string | null;
}

type Busy = { kind: "checking"; name: string } | { kind: "adding" } | null;

const changedNote = "The menu or the store changed since this file was checked. This is how it stands now.";

interface RecipesImportModalProps {
  api: StockApi;
  onClose: () => void;
}

export function RecipesImportModal({ api, onClose }: RecipesImportModalProps) {
  const toast = useToast();
  const [checked, setChecked] = useState<Checked | null>(null);
  const [busy, setBusy] = useState<Busy>(null);
  const [problem, setProblem] = useState<string | null>(null);

  const check = async (file: Blob, name: string, note: string | null) => {
    setBusy({ kind: "checking", name });
    setProblem(null);
    try {
      setChecked({ file, name, plan: await api.previewRecipes(file), note });
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
      const done = await api.importRecipes(c.file);
      if (done.imported) {
        toast(addedRecipesText(done), { tone: "success" });
        onClose();
        return;
      }
      // Every dish had a recipe by the time it was sent: say so, as the server put it.
      setChecked({ ...c, plan: done, note: changedNote });
    } catch (err) {
      // Refused: the file no longer fits. Check it again to say how.
      if (err instanceof ApiError && err.code === "stock_import_refused") {
        await check(c.file, c.name, changedNote);
        return;
      }
      setProblem(problemOf(err).message);
    } finally {
      setBusy((b) => (b?.kind === "adding" ? null : b));
    }
  };

  const ready = checked !== null && busy === null && recipesVerdict(checked.plan) === "ready";
  return (
    <Modal
      open
      onClose={onClose}
      title="Import recipes"
      size="lg"
      footer={
        <>
          <Button onClick={onClose}>{checked ? "Cancel" : "Close"}</Button>
          {ready && (
            <Button variant="primary" onClick={() => void add(checked)}>
              {addRecipesLabel(checked.plan)}
            </Button>
          )}
        </>
      }
    >
      <div className="flex flex-col gap-4">
        {busy ? (
          <p role="status">{busy.kind === "checking" ? `Checking ${busy.name}` : "Adding the recipes"}</p>
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
      <p>
        Say what each dish takes from stock, for many dishes at once. It gives a recipe to a dish that has none: it never changes one that is there. What a
        choice takes, like the milk on Oat, is set here on the screen.
      </p>
      <ol className="flex flex-col gap-4">
        <Step n={1}>
          <p>Get the template. It lists every dish that has no recipe yet.</p>
          <Button disabled={template.busy !== null} onClick={() => void template.download(recipesTemplatePath)}>
            <DownloadIcon aria-hidden="true" size={17} />
            {template.busy ? "Downloading" : "Download template"}
          </Button>
        </Step>
        <Step n={2}>
          <p>
            Beside each dish, type an ingredient by its name on Stock levels and how much one of the dish takes, in that ingredient's unit. For a second
            ingredient, add a row with the dish's name again. Leave a dish as it is to skip it. Save it as CSV UTF-8.
          </p>
        </Step>
        <Step n={3}>
          <p>Choose the file. You see the recipes it would give before any is added.</p>
          <ChooseFile label="Choose a file" onChoose={onChoose} />
        </Step>
      </ol>
    </>
  );
}

function Result({ checked, onChoose }: { checked: Checked; onChoose: (file: File | undefined) => void }) {
  const { plan, name, note } = checked;
  const verdict = recipesVerdict(plan);
  return (
    <>
      {note && <p className="rounded bg-attention-tint p-3.5">{note}</p>}
      {verdict === "problems" && <Problems line={recipeProblemsText(plan, name)} problems={plan.problems} onChoose={onChoose} />}
      {verdict === "nothing" && (
        <>
          <p>No dish in {name} is given a recipe.</p>
          {recipesLeftOut(plan).map((line) => (
            <p key={line}>{line}</p>
          ))}
          <div>
            <ChooseFile label="Choose another file" onChoose={onChoose} />
          </div>
        </>
      )}
      {verdict === "ready" && (
        <>
          <ul className="flex flex-col gap-1">
            {recipesSummary(plan).map((line, i) => (
              <li key={line} className={i === 0 ? "font-serif text-xl text-fg-heading" : undefined}>
                {line}
              </li>
            ))}
          </ul>
          <ul aria-label="Recipes to add" className="max-h-64 overflow-y-auto rounded border border-line">
            {plan.to_add.map((d) => (
              <li key={d.item_id} className="border-b border-line-soft px-3 py-2 last:border-b-0">
                <span className="block font-semibold">{d.dish}</span>
                <span className="block text-sm text-fg-2">{recipeLine(d)}</span>
              </li>
            ))}
          </ul>
          <p className="text-sm text-fg-muted">Sales from now on take this stock. Sales already made are not changed.</p>
          <div>
            <ChooseFile label="Choose another file" onChoose={onChoose} />
          </div>
        </>
      )}
    </>
  );
}
