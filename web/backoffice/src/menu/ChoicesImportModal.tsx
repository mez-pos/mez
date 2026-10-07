// Groups of choices from a spreadsheet (backoffice-menu.md, "Import
// choices"): get the template, choose the file filled in, read the groups
// it would add or what is wrong with it, and add them. A group is added
// offered by no item: that is done from an item, or from the items' file.

import { DownloadIcon } from "lucide-react";
import { Fragment, useState } from "react";
import { ApiError } from "../../../shared/api";
import type { ChoicesImport, ChoicesImportGroup } from "../../../shared/api-types";
import { Button, Modal, Money, useToast } from "../../../shared/ui";
import { ChooseFile, Problems, Step } from "../files/FileParts";
import { useCsvFile } from "../sales/use-csv-download";
import { ruleText } from "./group-logic";
import {
  addGroupsLabel,
  addedGroupsText,
  asCsv,
  choicePrice,
  choicesLeftOut,
  choicesNothingText,
  choicesProblemsText,
  choicesSummary,
  choicesTemplatePath,
  choicesVerdict,
  fileProblem,
} from "./import-logic";
import { problemOf } from "./menu-logic";
import type { MenuEditorApi } from "./use-menu-editor";

/** A file that has been checked: what it is, and what the server said of it. */
interface Checked {
  file: Blob;
  name: string;
  plan: ChoicesImport;
  /** Said over the plan: why it is being shown again. */
  note: string | null;
}

type Busy = { kind: "checking"; name: string } | { kind: "adding" } | null;

const changedNote = "The menu changed since this file was checked. This is how it stands now.";

interface ChoicesImportModalProps {
  api: MenuEditorApi;
  onClose: () => void;
}

export function ChoicesImportModal({ api, onClose }: ChoicesImportModalProps) {
  const toast = useToast();
  const [checked, setChecked] = useState<Checked | null>(null);
  const [busy, setBusy] = useState<Busy>(null);
  const [problem, setProblem] = useState<string | null>(null);

  const check = async (file: Blob, name: string, note: string | null) => {
    setBusy({ kind: "checking", name });
    setProblem(null);
    try {
      setChecked({ file, name, plan: await api.previewChoices(file), note });
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
      const done = await api.importChoices(c.file);
      if (done.imported) {
        toast(addedGroupsText(done), { tone: "success" });
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

  const ready = checked !== null && busy === null && choicesVerdict(checked.plan) === "ready";
  return (
    <Modal
      open
      onClose={onClose}
      title="Import choices"
      size="lg"
      footer={
        <>
          <Button onClick={onClose}>{checked ? "Cancel" : "Close"}</Button>
          {ready && (
            <Button variant="primary" onClick={() => void add(checked)}>
              {addGroupsLabel(checked.plan)}
            </Button>
          )}
        </>
      }
    >
      <div className="flex flex-col gap-4">
        {busy ? (
          <p role="status">{busy.kind === "checking" ? `Checking ${busy.name}` : "Adding the groups"}</p>
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
        Add groups of choices, like Size, Milk or Extras, from a spreadsheet. It adds groups: it never changes one that is already on the menu. Import these
        before the items that offer them.
      </p>
      <ol className="flex flex-col gap-4">
        <Step n={1}>
          <p>Get the template.</p>
          <Button disabled={template.busy !== null} onClick={() => void template.download(choicesTemplatePath)}>
            <DownloadIcon aria-hidden="true" size={17} />
            {template.busy ? "Downloading" : "Download template"}
          </Button>
        </Step>
        <Step n={2}>
          <p>
            Fill it in, one choice on each row, with its group's name beside it. On a group's first row say the fewest and the most a guest picks: 1 and 1 for a
            size they must choose, 0 and 2 for up to two extras. Price is what the choice adds, and may be below zero. Save it as CSV UTF-8.
          </p>
        </Step>
        <Step n={3}>
          <p>Choose the file. You see the groups it would add before any is added.</p>
          <ChooseFile label="Choose a file" onChoose={onChoose} />
        </Step>
      </ol>
    </>
  );
}

/** A group's choices in order, each with what it adds and whether it is picked to start. */
function ChoicesLine({ group }: { group: ChoicesImportGroup }) {
  return (
    <span className="block text-sm text-fg-2">
      {group.choices.map((c, i) => {
        const price = choicePrice(c);
        return (
          <Fragment key={c.name}>
            {i > 0 && ", "}
            {c.name}
            {c.is_default && " (to start)"}
            {price && (
              <>
                {" "}
                <span className="whitespace-nowrap font-mono tabular-nums">
                  {price.sign}
                  <Money amount={price.amount} />
                </span>
              </>
            )}
          </Fragment>
        );
      })}
    </span>
  );
}

function Result({ checked, onChoose }: { checked: Checked; onChoose: (file: File | undefined) => void }) {
  const { plan, name, note } = checked;
  const verdict = choicesVerdict(plan);
  return (
    <>
      {note && <p className="rounded bg-attention-tint p-3.5">{note}</p>}
      {verdict === "problems" && <Problems line={choicesProblemsText(plan, name)} problems={plan.problems} onChoose={onChoose} />}
      {verdict === "nothing" && (
        <>
          <p>{choicesNothingText(plan, name)}</p>
          {choicesLeftOut(plan).map((line) => (
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
            {choicesSummary(plan).map((line, i) => (
              <li key={line} className={i === 0 ? "font-serif text-xl text-fg-heading" : undefined}>
                {line}
              </li>
            ))}
          </ul>
          <ul aria-label="Groups to add" className="max-h-64 overflow-y-auto rounded border border-line">
            {plan.to_add.map((g) => (
              <li key={g.row} className="border-b border-line-soft px-3 py-2 last:border-b-0">
                <span className="flex items-baseline justify-between gap-3">
                  <span className="min-w-0 truncate font-semibold">{g.name}</span>
                  <span className="shrink-0 text-sm text-fg-muted">{ruleText(g)}</span>
                </span>
                <ChoicesLine group={g} />
              </li>
            ))}
          </ul>
          <p className="text-sm text-fg-muted">
            No item offers them yet. Offer a group from an item's editor, or name it in the Choices column when you import items.
          </p>
          <div>
            <ChooseFile label="Choose another file" onChoose={onChoose} />
          </div>
        </>
      )}
    </>
  );
}
