// Many prices at once (backoffice-menu.md, "Change prices"): export the
// menu, edit its Price column, give it back, read every price it would
// change, and change them. Nothing but a price changes, and no price
// changes until Change, and then all of them or none.

import { DownloadIcon } from "lucide-react";
import { useState } from "react";
import { ApiError } from "../../../shared/api";
import type { MenuPrices } from "../../../shared/api-types";
import { Button, Modal, Money, useToast } from "../../../shared/ui";
import type { CsvFile } from "../sales/use-csv-download";
import { ChooseFile, Problems, Step } from "../files/FileParts";
import { asCsv, exportPath, fileProblem } from "./import-logic";
import { problemOf } from "./menu-logic";
import {
  appliedText,
  applyLabel,
  byLargestChange,
  differenceOf,
  freeCount,
  freeText,
  priceProblemsText,
  priceSummary,
  priceVerdict,
  unknownText,
} from "./prices-logic";
import type { MenuEditorApi } from "./use-menu-editor";

/** A file that has been checked: what it is, and what the server said of it. */
interface Checked {
  file: Blob;
  name: string;
  plan: MenuPrices;
  /** Said over the plan: why it is being shown again. */
  note: string | null;
}

type Busy = { kind: "checking"; name: string } | { kind: "changing" } | null;

const changedNote = "The menu changed since this file was checked. This is how it stands now.";

/** The refusals that mean the file must be checked again against the menu as it is now. */
const recheck: readonly string[] = ["menu_prices_stale", "menu_prices_refused"];

interface PricesModalProps {
  api: MenuEditorApi;
  /** The screen's own download, so its pill shows the export under way. */
  exported: CsvFile;
  onClose: () => void;
}

export function PricesModal({ api, exported, onClose }: PricesModalProps) {
  const toast = useToast();
  const [checked, setChecked] = useState<Checked | null>(null);
  const [busy, setBusy] = useState<Busy>(null);
  const [problem, setProblem] = useState<string | null>(null);

  const check = async (file: Blob, name: string, note: string | null) => {
    setBusy({ kind: "checking", name });
    setProblem(null);
    try {
      setChecked({ file, name, plan: await api.previewPrices(file), note });
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
  const apply = async (c: Checked) => {
    setBusy({ kind: "changing" });
    setProblem(null);
    try {
      const done = await api.applyPrices(c.file, c.plan.version);
      if (done.applied) {
        toast(appliedText(done), { tone: "success" });
        onClose();
        return;
      }
      // Every price was already what the file says by the time it was sent.
      setChecked({ ...c, plan: done, note: changedNote });
    } catch (err) {
      // The menu moved under the file. Nothing changed: show it as it stands
      // now, for the owner to read again and press again.
      if (err instanceof ApiError && recheck.includes(err.code)) {
        await check(c.file, c.name, changedNote);
        return;
      }
      setProblem(problemOf(err).message);
    } finally {
      setBusy((b) => (b?.kind === "changing" ? null : b));
    }
  };

  const ready = checked !== null && busy === null && priceVerdict(checked.plan) === "ready";
  return (
    <Modal
      open
      onClose={onClose}
      title="Change prices"
      size="lg"
      footer={
        <>
          <Button onClick={onClose}>{checked ? "Cancel" : "Close"}</Button>
          {ready && (
            <Button variant="primary" onClick={() => void apply(checked)}>
              {applyLabel(checked.plan)}
            </Button>
          )}
        </>
      }
    >
      <div className="flex flex-col gap-4">
        {busy ? (
          <p role="status">{busy.kind === "checking" ? `Checking ${busy.name}` : "Changing the prices"}</p>
        ) : checked ? (
          <Result checked={checked} onChoose={choose} />
        ) : (
          <Steps exported={exported} onChoose={choose} />
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

function Steps({ exported, onChoose }: { exported: CsvFile; onChoose: (file: File | undefined) => void }) {
  return (
    <>
      <p>Change many prices at once from a spreadsheet. It changes prices and nothing else: it never renames, moves, retires, adds or removes an item.</p>
      <ol className="flex flex-col gap-4">
        <Step n={1}>
          <p>Export the menu.</p>
          <Button disabled={exported.busy !== null} onClick={() => void exported.download(exportPath)}>
            <DownloadIcon aria-hidden="true" size={17} />
            {exported.busy ? "Exporting" : "Export the menu"}
          </Button>
        </Step>
        <Step n={2}>
          <p>Change the numbers in the Price column and save it as CSV UTF-8. Leave the Category and Name as they are: that is how each row finds its item.</p>
        </Step>
        <Step n={3}>
          <p>Choose the file. You see every price it would change before any changes.</p>
          <ChooseFile label="Choose a file" onChoose={onChoose} />
        </Step>
      </ol>
    </>
  );
}

const head = "px-3 pb-2 pt-2.5 text-meta font-bold uppercase text-fg-muted";
const figure = "whitespace-nowrap px-3 py-2 text-right";

function Result({ checked, onChoose }: { checked: Checked; onChoose: (file: File | undefined) => void }) {
  const { plan, name, note } = checked;
  const verdict = priceVerdict(plan);
  const unknown = unknownText(plan);
  const free = freeCount(plan);
  return (
    <>
      {note && <p className="rounded bg-attention-tint p-3.5">{note}</p>}
      {verdict === "problems" && <Problems line={priceProblemsText(plan, name)} problems={plan.problems} onChoose={onChoose} />}
      {verdict === "nothing" && (
        <>
          <p>No price in {name} differs from the menu.</p>
          {unknown && <p>{unknown}</p>}
          <div>
            <ChooseFile label="Choose another file" onChoose={onChoose} />
          </div>
        </>
      )}
      {verdict === "ready" && (
        <>
          <ul className="flex flex-col gap-1">
            {priceSummary(plan).map((line, i) => (
              <li key={line} className={i === 0 ? "font-serif text-xl text-fg-heading" : undefined}>
                {line}
              </li>
            ))}
            {unknown && <li>{unknown}</li>}
          </ul>
          {free > 0 && <p className="rounded bg-attention-tint p-3.5 font-semibold">{freeText(free)}</p>}
          <div className="max-h-72 overflow-y-auto rounded border border-line">
            <table aria-label="Prices to change" className="w-full text-base">
              <thead className="sticky top-0 bg-raised">
                <tr className="border-b border-line">
                  <th scope="col" className={`${head} text-left`}>
                    Item
                  </th>
                  <th scope="col" className={`${head} text-right`}>
                    Now
                  </th>
                  <th scope="col" className={`${head} text-right`}>
                    New
                  </th>
                  <th scope="col" className={`${head} text-right`}>
                    Change
                  </th>
                </tr>
              </thead>
              <tbody>
                {byLargestChange(plan.changes).map((c) => {
                  const by = differenceOf(c);
                  return (
                    <tr key={c.item_id} className="border-b border-line-soft last:border-b-0">
                      <td className="px-3 py-2">
                        <span className="block font-semibold">{c.name}</span>
                        <span className="block text-sm text-fg-muted">{c.category}</span>
                      </td>
                      <td className={figure}>
                        <Money amount={c.old} tone="muted" />
                      </td>
                      <td className={figure}>
                        <Money amount={c.new} className="font-bold" />
                      </td>
                      <td className={`${figure} font-mono tabular-nums`}>
                        {/* The sign is a word for a reader that cannot see it. */}
                        <span aria-hidden="true">{by.direction === "up" ? "+" : "−"}</span>
                        <span className="sr-only">{by.direction === "up" ? "up by " : "down by "}</span>
                        <Money amount={by.amount} />
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <p className="text-sm text-fg-muted">Orders already open keep the prices they were rung up at. The next line rung up takes the new one.</p>
          <div>
            <ChooseFile label="Choose another file" onChoose={onChoose} />
          </div>
        </>
      )}
    </>
  );
}
