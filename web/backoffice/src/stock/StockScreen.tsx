// The Stock screen (docs/design/screens/backoffice-stock.md): stock items
// with what the ledger says is on hand, the forms that move it, each
// item's history, and the recipes that make sales take stock.

import { useState } from "react";
import type { StockItem, StockRecorded } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon, useToast } from "../../../shared/ui";
import { DataTable, type Column } from "../../../shared/ui/DataTable";
import { cx } from "../../../shared/ui/cx";
import { useMenu } from "../../../shared/use-menu";
import { HistoryModal } from "./HistoryModal";
import { MovementModal } from "./MovementModal";
import { formatDelta, formatQty } from "../../../shared/stock-qty";
import { RecipesView } from "./RecipesView";
import { problemOf, type HandKind } from "./stock-logic";
import { StockItemModal } from "./StockItemModal";
import { useStock, type StockApi } from "./use-stock";

type Open =
  | { kind: "add" }
  | { kind: "edit"; item: StockItem }
  | { kind: "move"; item: StockItem; move: HandKind }
  | { kind: "history"; item: StockItem };

type View = "items" | "recipes";

export function StockScreen() {
  const api = useStock();
  const menu = useMenu();
  const toast = useToast();
  const [view, setView] = useState<View>("items");
  const [open, setOpen] = useState<Open | null>(null);
  const items = api.items.data?.items;

  return (
    <div className="flex max-w-6xl flex-col gap-6">
      <div className="flex flex-wrap items-baseline justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Stock</h1>
          <p className="mt-1 text-sm text-fg-3">On hand is what was received, less what was sold, wasted and corrected. A count records the difference.</p>
        </div>
        {view === "items" && (
          <Button variant="primary" onClick={() => setOpen({ kind: "add" })}>
            Add a stock item
          </Button>
        )}
      </div>

      <div role="group" aria-label="Show" className="flex gap-tap-gap">
        {(["items", "recipes"] as const).map((v) => (
          <Button key={v} variant="ghost" aria-pressed={view === v} className={view === v ? "bg-raised font-semibold text-fg" : undefined} onClick={() => setView(v)}>
            {v === "items" ? "Items" : "Recipes"}
          </Button>
        ))}
      </div>

      {api.items.isPending && <p className="text-base text-fg-muted">Loading the stock</p>}
      {api.items.error && (
        <EmptyState icon={<InboxIcon />} line={problemOf(api.items.error).message} action={{ label: "Try again", onClick: () => void api.items.refetch() }} />
      )}
      {items && view === "items" && <ItemsTable items={items} onOpen={setOpen} />}
      {items && view === "recipes" && menu.data && (
        <RecipesView menu={menu.data} stock={items} api={api} onSaved={(name) => toast(`Saved what ${name} takes.`, { tone: "success" })} />
      )}
      {items && view === "recipes" && !menu.data && <p className="text-base text-fg-muted">Loading the menu</p>}

      {open && <Opened open={open} api={api} onOpen={setOpen} onClose={() => setOpen(null)} />}
    </div>
  );
}

function ItemsTable({ items, onOpen }: { items: readonly StockItem[]; onOpen: (o: Open) => void }) {
  const columns: Column<StockItem>[] = [
    {
      id: "name",
      header: "Item",
      cell: (it) => (
        <span className="flex items-center gap-2">
          <span className={cx("font-semibold", !it.active && "text-fg-muted line-through")}>{it.name}</span>
          {!it.active && <span className="text-meta uppercase text-fg-faint">Retired</span>}
          {it.low && <span className="text-meta font-semibold uppercase text-warning">Low</span>}
          {it.active && it.count_at_close && <span className="text-meta uppercase text-fg-faint">At close</span>}
        </span>
      ),
    },
    {
      id: "on-hand",
      header: "On hand",
      align: "right",
      cell: (it) => <span className={cx("font-mono tabular-nums", it.on_hand < 0 && "text-danger")}>{formatQty(it.on_hand, it.unit)}</span>,
    },
    {
      id: "low",
      header: "Low at",
      align: "right",
      cell: (it) =>
        it.low_level === 0 ? <span className="text-fg-muted">None</span> : <span className="font-mono tabular-nums">{formatQty(it.low_level, it.unit)}</span>,
    },
    {
      id: "actions",
      header: "",
      align: "right",
      cell: (it) => (
        <span className="flex flex-wrap justify-end gap-tap-gap">
          {it.active &&
            (["received", "wasted", "counted"] as const).map((move) => (
              <Button key={move} onClick={() => onOpen({ kind: "move", item: it, move })}>
                {move === "received" ? "Receive" : move === "wasted" ? "Waste" : "Count"}
              </Button>
            ))}
          <Button onClick={() => onOpen({ kind: "history", item: it })}>History</Button>
          <Button onClick={() => onOpen({ kind: "edit", item: it })}>Edit</Button>
        </span>
      ),
    },
  ];
  return (
    <DataTable
      label="Stock items"
      columns={columns}
      rows={items}
      rowKey={(it) => String(it.id)}
      empty={<EmptyState icon={<InboxIcon />} line="No stock yet. Add what you keep: beans, milk, batter." />}
    />
  );
}

interface OpenedProps {
  open: Open;
  api: StockApi;
  onOpen: (o: Open) => void;
  onClose: () => void;
}

function Opened({ open, api, onOpen, onClose }: OpenedProps) {
  const toast = useToast();
  switch (open.kind) {
    case "add":
      return (
        <StockItemModal
          item={null}
          onSave={async (input) => {
            await api.createItem(input);
            toast(`Added ${input.name}.`, { tone: "success" });
          }}
          onCorrect={() => undefined}
          onSetActive={() => Promise.resolve()}
          onClose={onClose}
        />
      );
    case "edit":
      return (
        <StockItemModal
          item={open.item}
          onSave={async (input) => {
            await api.updateItem(open.item.id, input);
            toast("Saved.", { tone: "success" });
          }}
          onCorrect={() => onOpen({ kind: "move", item: open.item, move: "adjusted" })}
          onSetActive={(active) => api.setActive(open.item.id, active)}
          onClose={onClose}
        />
      );
    case "move":
      return (
        <MovementModal
          item={open.item}
          kind={open.move}
          onSave={(input) => api.record(open.item.id, input)}
          onDone={(r) => toast(doneText(open.move, open.item, r), { tone: "success" })}
          onClose={onClose}
        />
      );
    case "history":
      return <HistoryModal item={open.item} onClose={onClose} />;
  }
}

function doneText(move: HandKind, item: StockItem, r: StockRecorded): string {
  switch (move) {
    case "received":
      return `Received ${formatQty(r.delta, item.unit)} of ${item.name}.`;
    case "wasted":
      return `Recorded ${formatQty(-r.delta, item.unit)} of ${item.name} thrown away.`;
    case "counted":
      return `Counted. That recorded ${formatDelta(r.delta, item.unit)}.`;
    case "adjusted":
      return `Corrected ${item.name} by ${formatDelta(r.delta, item.unit)}.`;
  }
}
