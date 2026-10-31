// An area's grid (backoffice-floor.md): a free cell is a dashed outline you
// tap to place or move a table; a table fills its cell and can be dragged
// to another. The drag is pointer events, so it works with a finger, a pen
// and a mouse, and it is only a shortcut: every move it makes can be made
// by tapping or by typing the column and row.
//
// A cell is never under 48px, the touch floor: so twelve columns still fit
// beside the chosen table's card at 1280px. Past that the grid scrolls sideways.
//
// From a keyboard the grid is one tab stop, and the arrows move between its
// cells, Home and End along a row (TECH-20, ADR-403): the grid pattern, not
// ninety-six tab stops.

import { useRef, useState, type FocusEvent, type KeyboardEvent, type PointerEvent } from "react";
import type { TableResponse } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";
import { selectedRing } from "../../../shared/ui/focus";
import { gridStep, seatsText, type Cell, type Place } from "./floor-logic";

interface FloorGridProps {
  cells: Cell[][];
  chosen: number | null;
  /** A table is waiting for a cell to move to. */
  moving: boolean;
  /** Tables can be chosen and nothing placed or moved: free cells are not buttons, and nothing drags. */
  readOnly?: boolean;
  onCell: (cell: Cell) => void;
  onTable: (table: TableResponse) => void;
  onDrop: (table: TableResponse, cell: Cell | null) => void;
}

/** How far a pointer travels before a press on a table is a drag, not a tap. */
const dragThresholdPx = 8;

interface Drag {
  table: TableResponse;
  pointerId: number;
  startX: number;
  startY: number;
  dx: number;
  dy: number;
  dragging: boolean;
}

export function FloorGrid({ cells, chosen, moving, readOnly = false, onCell, onTable, onDrop }: FloorGridProps) {
  const [drag, setDrag] = useState<Drag | null>(null);
  // A click follows the pointerup that ended a drag; it must not also choose the table.
  const justDragged = useRef(false);
  const columns = cells[0]?.length ?? 0;
  const grid = useRef<HTMLDivElement>(null);

  // The one cell a Tab lands on: the last one focused, else the chosen
  // table's, else the first with something to press.
  const pressable = ({ x, y }: Place): boolean => {
    const cell = cells[y]?.[x];
    return cell !== undefined && !(readOnly && cell.table === null);
  };
  const [last, setLast] = useState<Place | null>(null);
  const stop: Place | null =
    (last && pressable(last) ? last : null) ??
    cells.flat().find((c) => c.table !== null && c.table.id === chosen) ??
    cells.flat().find((c) => pressable(c)) ??
    null;
  const isStop = (c: Cell): boolean => stop !== null && stop.x === c.x && stop.y === c.y;
  const placeOf = (el: EventTarget): Place | null => {
    const at = (el as HTMLElement).closest?.<HTMLElement>("[data-cell]")?.dataset.cell;
    if (!at) return null;
    const [x, y] = at.split(",").map(Number);
    return x === undefined || y === undefined ? null : { x, y };
  };
  const onFocus = (e: FocusEvent<HTMLDivElement>) => {
    const at = placeOf(e.target);
    if (at) setLast(at);
  };
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.altKey || e.ctrlKey || e.metaKey) return;
    const from = placeOf(e.target);
    const to = from && gridStep(e.key, from, columns, cells.length, pressable);
    if (!to) return;
    e.preventDefault();
    setLast(to);
    grid.current?.querySelector<HTMLElement>(`[data-cell="${to.x},${to.y}"] button`)?.focus();
  };

  const cellAt = (clientX: number, clientY: number): Cell | null => {
    const el = document.elementFromPoint(clientX, clientY)?.closest<HTMLElement>("[data-cell]");
    if (!el) return null;
    const [x, y] = (el.dataset.cell ?? "").split(",").map(Number);
    return cells[y ?? -1]?.[x ?? -1] ?? null;
  };

  const down = (table: TableResponse) => (e: PointerEvent<HTMLButtonElement>) => {
    if (e.button !== 0 || readOnly) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    setDrag({ table, pointerId: e.pointerId, startX: e.clientX, startY: e.clientY, dx: 0, dy: 0, dragging: false });
  };
  const move = (e: PointerEvent<HTMLButtonElement>) => {
    if (!drag || e.pointerId !== drag.pointerId) return;
    const dx = e.clientX - drag.startX;
    const dy = e.clientY - drag.startY;
    setDrag({ ...drag, dx, dy, dragging: drag.dragging || Math.hypot(dx, dy) > dragThresholdPx });
  };
  const up = (e: PointerEvent<HTMLButtonElement>) => {
    if (!drag || e.pointerId !== drag.pointerId) return;
    setDrag(null);
    if (!drag.dragging) return;
    justDragged.current = true;
    onDrop(drag.table, cellAt(e.clientX, e.clientY));
  };

  return (
    <div className="relative overflow-x-auto rounded border border-line bg-sunken p-2">
      <div
        ref={grid}
        role="grid"
        aria-label="Floor plan"
        onFocus={onFocus}
        onKeyDown={onKeyDown}
        className="grid gap-1"
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(3rem, 1fr))` }}
      >
        {cells.map((row, y) => (
          <div role="row" key={y} className="contents">
            {row.map((cell) => {
              const t = cell.table;
              const dragged = drag?.dragging && t !== null && drag.table.id === t.id;
              return (
                <div role="gridcell" key={cell.x} data-cell={`${cell.x},${cell.y}`} className="relative aspect-square min-h-12">
                  {t === null && readOnly ? (
                    <span aria-hidden="true" className="absolute inset-0 rounded border border-dashed border-line-soft" />
                  ) : t === null ? (
                    <button
                      type="button"
                      tabIndex={isStop(cell) ? 0 : -1}
                      aria-label={moving ? `Move here: column ${cell.x + 1}, row ${cell.y + 1}` : `Place a table at column ${cell.x + 1}, row ${cell.y + 1}`}
                      onClick={() => onCell(cell)}
                      className={cx(
                        "absolute inset-0 rounded border border-dashed outline-offset-2 outline-focus focus-visible:outline-3",
                        moving ? "border-line-strong bg-raised" : "border-line",
                      )}
                    />
                  ) : (
                    <button
                      type="button"
                      tabIndex={isStop(cell) ? 0 : -1}
                      aria-label={`Table ${t.name}, ${seatsText(t.seats)}`}
                      aria-pressed={chosen === t.id}
                      onPointerDown={down(t)}
                      onPointerMove={move}
                      onPointerUp={up}
                      onPointerCancel={() => setDrag(null)}
                      onClick={() => {
                        if (justDragged.current) {
                          justDragged.current = false;
                          return;
                        }
                        onTable(t);
                      }}
                      style={dragged ? { transform: `translate(${drag.dx}px, ${drag.dy}px)` } : undefined}
                      className={cx(
                        "absolute inset-0 flex touch-none select-none items-center justify-center outline-offset-2 outline-focus focus-visible:outline-3",
                        dragged && "pointer-events-none z-10 opacity-80",
                      )}
                    >
                      {/* Chosen is a green ring inside the tile's edge; focus is the outline outside the cell (TECH-12). */}
                      <TableShapeTile table={t} chosen={chosen === t.id} />
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}

/** The table drawn as its shape, with its name and seats. */
function TableShapeTile({ table: t, chosen }: { table: TableResponse; chosen: boolean }) {
  return (
    <span
      className={cx(
        "flex flex-col items-center justify-center border border-line-strong bg-raised text-fg shadow-rest",
        t.shape === "round" && "aspect-square h-[88%] rounded-round",
        t.shape === "square" && "h-[88%] w-[88%] rounded",
        // Its name and seats need 40px: a 56px cell's 60% would cut them.
        t.shape === "long" && "h-[60%] min-h-11 w-full rounded",
        chosen && selectedRing,
      )}
    >
      <span className="max-w-full truncate px-1 font-serif text-lg leading-tight">{t.name}</span>
      <span className="text-sm text-fg-2">
        <span className="font-mono">{t.seats}</span>
      </span>
    </span>
  );
}
