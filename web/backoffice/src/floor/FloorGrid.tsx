// An area's grid (backoffice-floor.md): a free cell is a dashed outline you
// tap to place or move a table; a table fills its cell and can be dragged
// to another. The drag is pointer events, so it works with a finger, a pen
// and a mouse, and it is only a shortcut: every move it makes can be made
// by tapping or by typing the column and row.

import { useRef, useState, type PointerEvent } from "react";
import type { TableResponse } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";
import { seatsText, type Cell } from "./floor-logic";

interface FloorGridProps {
  cells: Cell[][];
  chosen: number | null;
  /** A table is waiting for a cell to move to. */
  moving: boolean;
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

export function FloorGrid({ cells, chosen, moving, onCell, onTable, onDrop }: FloorGridProps) {
  const [drag, setDrag] = useState<Drag | null>(null);
  // A click follows the pointerup that ended a drag; it must not also choose the table.
  const justDragged = useRef(false);
  const columns = cells[0]?.length ?? 0;

  const cellAt = (clientX: number, clientY: number): Cell | null => {
    const el = document.elementFromPoint(clientX, clientY)?.closest<HTMLElement>("[data-cell]");
    if (!el) return null;
    const [x, y] = (el.dataset.cell ?? "").split(",").map(Number);
    return cells[y ?? -1]?.[x ?? -1] ?? null;
  };

  const down = (table: TableResponse) => (e: PointerEvent<HTMLButtonElement>) => {
    if (e.button !== 0) return;
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
    <div className="overflow-x-auto rounded border border-line bg-sunken p-2">
      <div
        role="grid"
        aria-label="Floor plan"
        className="grid gap-1"
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(3.5rem, 1fr))` }}
      >
        {cells.map((row, y) => (
          <div role="row" key={y} className="contents">
            {row.map((cell) => {
              const t = cell.table;
              const dragged = drag?.dragging && t !== null && drag.table.id === t.id;
              return (
                <div role="gridcell" key={cell.x} data-cell={`${cell.x},${cell.y}`} className="relative aspect-square min-h-14">
                  {t === null ? (
                    <button
                      type="button"
                      aria-label={moving ? `Move here: column ${cell.x + 1}, row ${cell.y + 1}` : `Place a table at column ${cell.x + 1}, row ${cell.y + 1}`}
                      onClick={() => onCell(cell)}
                      className={cx(
                        "absolute inset-0 rounded border border-dashed outline-offset-2 outline-line-focus focus-visible:outline-2",
                        moving ? "border-line-strong bg-raised" : "border-line",
                      )}
                    />
                  ) : (
                    <button
                      type="button"
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
                        "absolute inset-0 flex touch-none select-none items-center justify-center outline-offset-2 outline-line-focus focus-visible:outline-2",
                        dragged && "pointer-events-none z-10 opacity-80",
                        chosen === t.id && "outline outline-2",
                      )}
                    >
                      <TableShapeTile table={t} />
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
function TableShapeTile({ table: t }: { table: TableResponse }) {
  return (
    <span
      className={cx(
        "flex flex-col items-center justify-center border border-line-strong bg-raised-high text-fg",
        t.shape === "round" && "aspect-square h-[88%] rounded-round",
        t.shape === "square" && "h-[88%] w-[88%] rounded",
        t.shape === "long" && "h-[60%] w-full rounded",
      )}
    >
      <span className="max-w-full truncate px-1 text-lg font-semibold leading-tight">{t.name}</span>
      <span className="text-sm text-fg-2">
        <span className="font-mono">{t.seats}</span>
      </span>
    </span>
  );
}
