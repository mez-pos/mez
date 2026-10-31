// Transfer and merge (T06): a modal from the table's screen. Transfer moves the
// party to a free table; Merge joins free tables to it and lets joined ones go.
// The tables are tiles: a free one can be taken, the rest say why not.
//
// Left out: the note that guest QR sessions follow the table (phase 11) and
// "Notify the kitchen of the new table number" (the kitchen's board reads the
// table from the order, so it follows without being told).

import { ArrowLeftRightIcon, ArrowRightIcon, GitMergeIcon, type LucideIcon } from "lucide-react";
import { useState } from "react";
import type { LiveFloorResponse, LiveTableResponse, OrderResponse } from "../../../shared/api-types";
import { Button, Modal, Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import type { FloorAction } from "../floor/floor-logic";
import { wordsOf } from "../register/order-messages";
import { guestsText } from "../register/rail-logic";
import { canPick, moveActions, moveLabel, moveTiles, pickTile, type MoveMode, type MoveTile } from "./move-logic";

interface MoveTableProps {
  floor: LiveFloorResponse;
  party: LiveTableResponse;
  /** "Table T4 + T7". */
  title: string;
  order: OrderResponse;
  startIn: MoveMode;
  act: (action: FloorAction) => Promise<void>;
  /** Called before a transfer is asked for, with where the party is going. */
  onMoving: (to: number | null) => void;
  onClose: () => void;
}

const modes: readonly { id: MoveMode; label: string; icon: LucideIcon; hint: string }[] = [
  { id: "transfer", label: "Transfer", icon: ArrowLeftRightIcon, hint: "Choose the destination table" },
  { id: "merge", label: "Merge", icon: GitMergeIcon, hint: "Choose free tables to join to this party. Tap a joined one to let it go." },
];

const tileLook = (tile: MoveTile, on: boolean): string => {
  if (on) return "border-accent bg-chip-success-bg text-accent";
  if (tile.state === "free") return "border-chip-success-border bg-chip-success-bg/40 hover:bg-chip-success-bg";
  if (tile.state === "joined") return "border-chip-info-border bg-chip-info-bg/40";
  return "border-line bg-sunken";
};

export function MoveTable({ floor, party, title, order, startIn, act, onMoving, onClose }: MoveTableProps) {
  const [mode, setMode] = useState<MoveMode>(startIn);
  const [picked, setPicked] = useState<readonly number[]>([]);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const tiles = moveTiles(floor, party);
  const label = moveLabel(mode, tiles, picked);
  const lines = order.lines.filter((l) => l.status !== "voided").length;
  const guests = guestsText(order.covers);

  const confirm = async () => {
    const actions = moveActions(mode, order.id, party, tiles, picked);
    const move = actions.find((a) => a.kind === "move");
    setBusy(true);
    setFailure(null);
    // The table's screen follows the party: it must know where before the floor says the old table is free.
    if (move?.kind === "move") onMoving(move.to);
    try {
      for (const action of actions) await act(action);
      onClose();
    } catch (err) {
      onMoving(null);
      setFailure(wordsOf(err));
      setBusy(false);
    }
  };

  return (
    <Modal
      open
      onClose={onClose}
      title={`Move ${title}`}
      size="md"
      footer={
        <div className="flex w-full gap-3">
          <Button size="charge" className="flex-1" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" size="charge" className="flex-1" disabled={busy || label === null} onClick={() => void confirm()}>
            {label ?? (mode === "transfer" ? "Move" : "Merge")}
            {mode === "transfer" && <ArrowRightIcon aria-hidden="true" size={18} />}
          </Button>
        </div>
      }
    >
      <p className="-mt-2 flex flex-wrap items-center gap-x-2 text-base text-fg-muted">
        {lines} {lines === 1 ? "item" : "items"} · <Money amount={order.totals.total} />
        {guests && ` · ${guests}`}
      </p>

      <div role="group" aria-label="What to do" className="mt-4 inline-flex gap-1 rounded-pill bg-sunken p-1">
        {modes.map(({ id, label: name, icon: Icon }) => (
          <button
            key={id}
            type="button"
            aria-pressed={mode === id}
            onClick={() => {
              setMode(id);
              setPicked([]);
              setFailure(null);
            }}
            className={cx(
              "inline-flex min-h-tap items-center gap-2 rounded-pill px-5 text-base outline-offset-2 outline-focus focus-visible:outline-3",
              mode === id ? "bg-accent font-semibold text-accent-fg" : "text-fg-2 hover:bg-raised-high",
            )}
          >
            <Icon aria-hidden="true" size={17} />
            {name}
          </button>
        ))}
      </div>

      <p className="mt-4 text-base text-fg-2">{modes.find((m) => m.id === mode)?.hint}</p>
      <ul aria-label="Tables" className="mt-3 grid max-h-80 grid-cols-3 gap-2.5 overflow-y-auto sm:grid-cols-4">
        {tiles.map((tile) => {
          const on = picked.includes(tile.table.id);
          const pickable = canPick(mode, tile);
          return (
            <li key={tile.table.id}>
              <button
                type="button"
                aria-pressed={on}
                disabled={!pickable || busy}
                onClick={() => setPicked(pickTile(mode, picked, tile))}
                className={cx(
                  "flex min-h-20 w-full flex-col items-center justify-center gap-1 rounded border outline-offset-2 outline-focus focus-visible:outline-3",
                  tileLook(tile, on),
                  !pickable && "opacity-60",
                )}
              >
                <span className="font-serif text-lg leading-none">{tile.table.name}</span>
                <span className={cx("text-sm font-semibold", tile.state === "free" ? "text-chip-success-fg" : "text-fg-muted")}>{tile.word}</span>
              </button>
            </li>
          );
        })}
      </ul>
      {tiles.length === 0 && <p className="mt-3 text-base text-fg-muted">There are no other tables on the floor.</p>}

      {failure && (
        <p role="alert" className="mt-4 text-base text-danger">
          {failure}
        </p>
      )}
    </Modal>
  );
}
