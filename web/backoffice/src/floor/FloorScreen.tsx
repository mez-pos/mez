// The venue's floor plan (docs/design/screens/backoffice-floor.md): its
// areas as tabs, each area's grid of tables, the chosen table's settings
// beside the grid from 1280px (under it below that), and the retired tables
// waiting to come back.

import { useEffect, useRef, useState } from "react";
import type { AreaResponse, FloorResponse, TableResponse } from "../../../shared/api-types";
import { OnAMirror } from "../../../shared/shell/OnAMirror";
import { Button, Card, EmptyState, InboxIcon } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { pillClass } from "../../../shared/ui/pill";
import { useOnMirror } from "../../../shared/use-mirror";
import { retireAreaWords, retireTableWords, type RetireWords } from "../retire-logic";
import { RetireConfirm } from "../RetireConfirm";
import {
  areaToShow,
  cellsOf,
  emptyTableForm,
  moveTo,
  placeText,
  problemOf,
  retiredIn,
  seatsText,
  tableFormFrom,
  toTableSettings,
  type Cell,
  type TableForm,
} from "./floor-logic";
import { AreaFormModal, TableFormModal } from "./FloorFormModals";
import { FloorGrid } from "./FloorGrid";
import { TableCodesCard } from "./TableCodesCard";
import { useFloor, type FloorApi } from "./use-floor";

type Editing =
  { kind: "add-table"; form: TableForm } | { kind: "edit-table"; table: TableResponse } | { kind: "add-area" } | { kind: "rename-area"; area: AreaResponse };

interface Message {
  text: string;
  error: boolean;
}

export function FloorScreen() {
  const api = useFloor();
  const [areaChoice, setAreaChoice] = useState<number | null>(null);
  const [editing, setEditing] = useState<Editing | null>(null);
  // A mirror shows the plan as it was and moves nothing on it (ADR-330).
  const mirror = useOnMirror();
  const data = api.data;
  const areaId = data ? areaToShow(data, areaChoice) : null;

  return (
    // Wider from 1280px, where the chosen table's card takes a column beside the grid.
    <div className="mx-auto flex max-w-5xl flex-col gap-6 short:gap-4 xl:max-w-7xl">
      <div className="flex flex-wrap items-baseline justify-between gap-4">
        <h1 className="font-serif text-total">Floor plan</h1>
        {mirror ? (
          <OnAMirror />
        ) : (
          <div className="flex flex-wrap gap-tap-gap">
            <Button onClick={() => setEditing({ kind: "add-area" })}>Add an area</Button>
            {areaId !== null && (
              <Button variant="primary" onClick={() => setEditing({ kind: "add-table", form: emptyTableForm(areaId) })}>
                Add a table
              </Button>
            )}
          </div>
        )}
      </div>

      {api.loading && <p className="text-base text-fg-muted">Loading the floor plan</p>}
      {!api.loading && api.error !== null && (
        <EmptyState icon={<InboxIcon />} line={problemOf(api.error).message} action={{ label: "Try again", onClick: api.refetch }} />
      )}
      {data && areaId === null && (
        <EmptyState icon={<InboxIcon />} line={mirror ? "No areas yet." : "No areas yet. Add one for each room or terrace, then place its tables."} />
      )}
      {data && areaId !== null && (
        <AreaView key={areaId} data={data} areaId={areaId} api={api} readOnly={mirror} onChooseArea={setAreaChoice} onEdit={setEditing} />
      )}
      {/* The codes guests order by: for the tables of every area, under whichever is shown.
          A mirror serves no guests and refuses the codes (403), so there they are not asked for. */}
      {data && (mirror ? <NoCodesOnAMirror /> : <TableCodesCard />)}

      {editing && data && (editing.kind === "add-table" || editing.kind === "edit-table") && (
        <TableFormModal
          title={editing.kind === "add-table" ? "Add a table" : `Edit ${editing.table.name}`}
          initial={editing.kind === "add-table" ? editing.form : tableFormFrom(editing.table)}
          areas={data.areas}
          columns={data.grid.columns}
          rows={data.grid.rows}
          onSave={(form) => (editing.kind === "add-table" ? api.createTable(toTableSettings(form)) : api.updateTable(editing.table.id, toTableSettings(form)))}
          onClose={() => setEditing(null)}
        />
      )}
      {editing && (editing.kind === "add-area" || editing.kind === "rename-area") && (
        <AreaFormModal
          title={editing.kind === "add-area" ? "Add an area" : `Rename ${editing.area.name}`}
          initial={editing.kind === "add-area" ? "" : editing.area.name}
          onSave={(name) => (editing.kind === "add-area" ? api.createArea({ name }) : api.renameArea(editing.area.id, { name }))}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}

/** Where the table codes are, on a mirror: nobody orders from a copy. */
function NoCodesOnAMirror() {
  return (
    <Card padding="lg" className="flex min-w-0 flex-col gap-3">
      <h2 className="font-serif text-xl">Ordering from the table</h2>
      <OnAMirror>Nobody orders from a copy, so it shows no table codes. They are seen, renewed and printed at the venue.</OnAMirror>
    </Card>
  );
}

interface AreaViewProps {
  data: FloorResponse;
  areaId: number;
  api: FloorApi;
  /** The plan is shown and nothing on it is changed: a mirror. */
  readOnly: boolean;
  onChooseArea: (id: number) => void;
  onEdit: (e: Editing) => void;
}

/** One area: its tabs, its grid, the chosen table and the retired ones. */
function AreaView({ data, areaId, api, readOnly, onChooseArea, onEdit }: AreaViewProps) {
  const [chosenId, setChosenId] = useState<number | null>(null);
  const [moving, setMoving] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<Message | null>(null);
  // What a Retire is asking about, and what it does once agreed (BO-6).
  const [retiring, setRetiring] = useState<{ words: RetireWords; retire: () => void } | null>(null);
  const area = data.areas.find((a) => a.id === areaId)!;
  const cells = cellsOf(data, areaId);
  const chosen = data.tables.find((t) => t.id === chosenId && t.active && t.area_id === areaId) ?? null;
  const retired = retiredIn(data, areaId);
  const empty = cells.every((row) => row.every((c) => c.table === null));
  const card = useRef<HTMLDivElement>(null);
  const shownId = chosen?.id ?? null;

  // Under 1280px the card is under the grid, below the fold of a 768 screen:
  // picking a table brings it up. Beside the grid it is in sight and nothing moves.
  useEffect(() => {
    if (shownId === null) return;
    const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    card.current?.scrollIntoView({ block: "nearest", behavior: still ? "auto" : "smooth" });
  }, [shownId]);

  const run = (fn: () => Promise<void>, done: string) => {
    setBusy(true);
    setMessage(null);
    fn()
      .then(() => setMessage({ text: done, error: false }))
      .catch((err: unknown) => setMessage({ text: problemOf(err).message, error: true }))
      .finally(() => setBusy(false));
  };
  const moveTable = (table: TableResponse, cell: Cell | null) => {
    setMoving(false);
    const m = moveTo(table, cell, areaId);
    if (m.kind === "refused") setMessage({ text: m.message, error: true });
    if (m.kind === "move") run(() => api.updateTable(table.id, m.settings), `Moved ${table.name}.`);
  };
  const onCell = (cell: Cell) => {
    if (moving && chosen) moveTable(chosen, cell);
    else onEdit({ kind: "add-table", form: emptyTableForm(areaId, cell.x, cell.y) });
  };

  return (
    <>
      <div role="tablist" aria-label="Areas" className="flex flex-wrap gap-tap-gap">
        {data.areas.map((a) => (
          <button
            key={a.id}
            type="button"
            role="tab"
            aria-selected={a.id === areaId}
            onClick={() => onChooseArea(a.id)}
            className={cx(pillClass(a.id === areaId), !a.active && "line-through opacity-60")}
          >
            {a.name}
          </button>
        ))}
      </div>

      <div hidden={readOnly} className="flex flex-wrap items-center gap-tap-gap">
        <Button disabled={busy} onClick={() => onEdit({ kind: "rename-area", area })}>
          Rename {area.name}
        </Button>
        {area.active ? (
          <Button
            variant="danger"
            disabled={busy}
            onClick={() =>
              setRetiring({
                words: retireAreaWords(area.name, data.tables.filter((t) => t.active && t.area_id === area.id).length),
                retire: () => run(() => api.retireArea(area.id), `Retired ${area.name}.`),
              })
            }
          >
            Retire {area.name}
          </Button>
        ) : (
          <Button disabled={busy} onClick={() => run(() => api.restoreArea(area.id), `${area.name} is back.`)}>
            Bring back {area.name}
          </Button>
        )}
      </div>

      {moving && chosen && (
        <div role="status" className="flex flex-wrap items-center gap-tap-gap">
          <p className="text-base font-semibold">Tap a free cell for {chosen.name}</p>
          <Button variant="ghost" onClick={() => setMoving(false)}>
            Cancel
          </Button>
        </div>
      )}
      {empty && area.active && !readOnly && <p className="text-base text-fg-muted">Tap a cell to place a table.</p>}

      {/* From 1280px the chosen table's card is a column beside the grid, and stays in
          sight as the page scrolls; under that it follows the grid (BO-7). */}
      <div className={cx("grid grid-cols-1 items-start gap-4", !empty && "xl:grid-cols-[minmax(0,1fr)_var(--aside)]")}>
        <div className="flex min-w-0 flex-col gap-3">
          <FloorGrid
            cells={cells}
            chosen={chosen?.id ?? null}
            moving={moving}
            readOnly={readOnly}
            onCell={onCell}
            onTable={(t) => {
              setMoving(false);
              setMessage(null);
              setChosenId(t.id);
            }}
            onDrop={moveTable}
          />
          {message && (
            <p role={message.error ? "alert" : "status"} className={message.error ? "text-base text-danger" : "text-base text-fg-2"}>
              {message.text}
            </p>
          )}
        </div>

        {!empty && (
          // With no table picked, only beside the grid: under it an empty column is a gap before the codes.
          <div ref={card} className={cx("min-w-0 scroll-my-4 xl:sticky xl:top-0", !chosen && "hidden xl:block")}>
            {chosen ? (
              <Card padding="lg" className="flex flex-col gap-3 short:p-4">
                <h2 className="text-lg font-semibold">Table {chosen.name}</h2>
                <p className="flex flex-wrap gap-x-4 text-base text-fg-2">
                  <span>{seatsText(chosen.seats)}</span>
                  <span className="capitalize">{chosen.shape}</span>
                  <span>{placeText(data, chosen)}</span>
                </p>
                <div hidden={readOnly} className="flex flex-wrap gap-tap-gap">
                  <Button disabled={busy} onClick={() => onEdit({ kind: "edit-table", table: chosen })}>
                    Edit
                  </Button>
                  <Button disabled={busy} onClick={() => setMoving(true)}>
                    Move
                  </Button>
                  <Button
                    variant="danger"
                    disabled={busy}
                    onClick={() =>
                      setRetiring({ words: retireTableWords(chosen.name), retire: () => run(() => api.retireTable(chosen.id), `Retired ${chosen.name}.`) })
                    }
                  >
                    Retire
                  </Button>
                </div>
              </Card>
            ) : (
              <Card className="text-base text-fg-muted">
                {readOnly ? "Pick a table to see its seats and where it is." : "Pick a table to edit, move or retire it."}
              </Card>
            )}
          </div>
        )}
      </div>

      {retired.length > 0 && (
        <section aria-labelledby="retired-tables" className="flex flex-col gap-3">
          <h2 id="retired-tables" className="text-xl font-semibold">
            Retired tables
          </h2>
          <ul className="flex flex-col gap-2">
            {retired.map((t) => (
              <li key={t.id} className="flex flex-wrap items-center gap-4">
                <span className="text-base text-fg-muted">
                  {t.name}, {placeText(data, t)}
                </span>
                {!readOnly && (
                  <>
                    <Button disabled={busy} onClick={() => run(() => api.restoreTable(t.id), `${t.name} is back.`)}>
                      Bring back
                    </Button>
                    <Button variant="ghost" disabled={busy} onClick={() => onEdit({ kind: "edit-table", table: t })}>
                      Edit
                    </Button>
                  </>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      <RetireConfirm words={retiring?.words ?? null} onClose={() => setRetiring(null)} onRetire={() => retiring?.retire()} />
    </>
  );
}
