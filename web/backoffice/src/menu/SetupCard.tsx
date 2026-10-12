// One list of the Stations and courses view (backoffice-menu.md): each row with
// what names it, how many items it carries, and what can be done to it. A
// station and a course are drawn the same way, so one card does both.

import type { ReactNode } from "react";
import { Button, Card, EmptyState, InboxIcon } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { itemsText, type SetupRow } from "./setup-logic";

export interface SetupCardProps {
  title: string;
  description: ReactNode;
  addLabel: string;
  empty: string;
  rows: readonly SetupRow[];
  /** An edit is out: nothing else is started meanwhile. */
  busy: boolean;
  /** The rows are read and none is changed: a mirror (ADR-330). */
  readOnly?: boolean;
  onAdd: () => void;
  onRename: (row: SetupRow) => void;
  onMove: (row: SetupRow, delta: -1 | 1) => void;
  onRetire: (row: SetupRow) => void;
  onRestore: (row: SetupRow) => void;
}

export function SetupCard({ title, description, addLabel, empty, rows, busy, readOnly = false, onAdd, onRename, onMove, onRetire, onRestore }: SetupCardProps) {
  return (
    <Card padding="lg" className="min-w-0 md:p-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <h2 className="font-serif text-xl">{title}</h2>
          <p className="mt-1 text-base text-fg-2">{description}</p>
        </div>
        {!readOnly && (
          <Button disabled={busy} onClick={onAdd}>
            {addLabel}
          </Button>
        )}
      </div>

      {rows.length === 0 ? (
        <div className="mt-6">
          <EmptyState icon={<InboxIcon />} line={empty} />
        </div>
      ) : (
        <ul className="mt-6 flex flex-col divide-y divide-line-soft border-y border-line-soft">
          {rows.map((r, i) => (
            <li key={r.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 py-3">
              <div className="min-w-0 flex-1">
                <p className={cx("text-base font-semibold", !r.active && "text-fg-muted line-through")}>{r.name}</p>
                <p className="flex flex-wrap gap-x-3 text-sm text-fg-muted">
                  <span className="font-mono">{r.tag}</span>
                  <span>{itemsText(r.items)}</span>
                  {!r.active && <span>Retired</span>}
                </p>
              </div>
              <div hidden={readOnly} className="flex flex-wrap gap-2">
                <Button size="md" disabled={busy} onClick={() => onRename(r)}>
                  Rename
                </Button>
                <Button size="md" aria-label={`Move ${r.name} up`} disabled={busy || i === 0} onClick={() => onMove(r, -1)}>
                  ↑
                </Button>
                <Button size="md" aria-label={`Move ${r.name} down`} disabled={busy || i === rows.length - 1} onClick={() => onMove(r, 1)}>
                  ↓
                </Button>
                {r.active ? (
                  <Button size="md" variant="danger" disabled={busy} onClick={() => onRetire(r)}>
                    Retire
                  </Button>
                ) : (
                  <Button size="md" disabled={busy} onClick={() => onRestore(r)}>
                    Bring back
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
