// What every dialog that takes a file shares (backoffice-menu.md, "Import"
// and "Change prices"; backoffice-stock.md, "Import"): the button that opens the file picker, a numbered
// step, and the table of what is wrong with a file.

import { useId, type ReactNode } from "react";
import { whereOf, type FileProblem } from "./file-logic";

const chooseClass =
  "inline-flex min-h-tap cursor-pointer items-center rounded border border-line-strong bg-raised-high px-4 text-base font-semibold focus-within:outline-3 focus-within:outline-focus";

interface ChooseFileProps {
  label: string;
  onChoose: (file: File | undefined) => void;
}

/** A button that opens the file picker. The same file may be chosen twice: it may have been edited since. */
export function ChooseFile({ label, onChoose }: ChooseFileProps) {
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

/** One numbered step of getting a file ready. */
export function Step({ n, children }: { n: number; children: ReactNode }) {
  return (
    <li className="flex items-start gap-3">
      <span aria-hidden="true" className="flex size-8 shrink-0 items-center justify-center rounded-round bg-sunken font-mono text-sm font-bold tabular-nums">
        {n}
      </span>
      <div className="flex min-w-0 flex-col items-start gap-2 pt-1">{children}</div>
    </li>
  );
}

const head = "px-3 pb-2 pt-2.5 text-left text-meta font-bold uppercase text-fg-muted";

interface ProblemsProps {
  /** How many, in which file, and that nothing was done. */
  line: string;
  problems: readonly FileProblem[];
  onChoose: (file: File | undefined) => void;
}

/** What is wrong with a file, by row and column in the server's words, and the way to try again. */
export function Problems({ line, problems, onChoose }: ProblemsProps) {
  return (
    <>
      <p className="font-semibold text-danger">{line}</p>
      <div className="max-h-72 overflow-y-auto rounded border border-line">
        <table aria-label="Problems in the file" className="w-full text-base">
          <thead className="sticky top-0 bg-raised">
            <tr className="border-b border-line">
              <th scope="col" className={head}>
                Where
              </th>
              <th scope="col" className={head}>
                Column
              </th>
              <th scope="col" className={head}>
                What is wrong
              </th>
            </tr>
          </thead>
          <tbody>
            {problems.map((p, i) => (
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
  );
}
