// The Inventory screen (D19): the ingredients with what the ledger says is
// on hand, the forms that move it, each one's history, the week's wastage
// beside the table, and the recipes that make sales take stock.
//
// Left out of the design: purchase orders and suppliers, which are out of v1.
// "New purchase order" is Add ingredient, and the Purchase orders and
// Suppliers tabs are not drawn. The wastage log is the report on Items and
// inventory. Recipes are a view of their own here (rebuild.md, rule 4).

import { ClipboardListIcon, PlusIcon, Trash2Icon, UploadIcon } from "lucide-react";
import { useState } from "react";
import type { StockItem, StockRecorded } from "../../../shared/api-types";
import { formatDelta, formatQty } from "../../../shared/stock-qty";
import { Button, EmptyState, InboxIcon, Modal, useToast } from "../../../shared/ui";
import { pillClass } from "../../../shared/ui/pill";
import { useMenu } from "../../../shared/use-menu";
import { CheckField } from "../FormFields";
import { usePermissions } from "../shell/BackOfficeShell";
import { HistoryModal } from "./HistoryModal";
import { ItemsImportModal } from "./ItemsImportModal";
import { retiredCount, summary, summaryText, tableRows } from "./levels-logic";
import { MovementModal } from "./MovementModal";
import { RecipesImportModal } from "./RecipesImportModal";
import { RecipesView } from "./RecipesView";
import { problemOf, type HandKind } from "./stock-logic";
import { StockItemModal } from "./StockItemModal";
import { StockLevels } from "./StockLevels";
import { useStock, type StockApi } from "./use-stock";
import { WastageCard } from "./WastageCard";

type Open =
  | { kind: "add" }
  | { kind: "edit"; item: StockItem }
  | { kind: "move"; item: StockItem; move: HandKind }
  | { kind: "history"; item: StockItem }
  /** Which ingredient a count or a write-off is of, asked from the buttons beside the title. */
  | { kind: "pick"; move: "counted" | "wasted" };

type View = "levels" | "recipes";
const views: readonly { id: View; label: string }[] = [
  { id: "levels", label: "Stock levels" },
  { id: "recipes", label: "Recipes" },
];

export function StockScreen() {
  const api = useStock();
  const menu = useMenu();
  const toast = useToast();
  const held = usePermissions();
  const [view, setView] = useState<View>("levels");
  const [open, setOpen] = useState<Open | null>(null);
  const [showRetired, setShowRetired] = useState(false);
  const [importing, setImporting] = useState(false);
  const items = api.items.data?.items;
  const hasActive = items?.some((it) => it.active) ?? false;
  // The week's wastage is a report: without reports.view the card is not drawn.
  const canReport = held.includes("reports.view");

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Inventory</h1>
          <p className="mt-1 text-base text-fg-muted">{items ? summaryText(summary(items)) : "Reading the stock"}</p>
        </div>
        <div className="flex flex-wrap gap-2.5">
          <Button disabled={!hasActive} onClick={() => setOpen({ kind: "pick", move: "counted" })}>
            <ClipboardListIcon aria-hidden="true" size={17} />
            Stock count
          </Button>
          <Button disabled={!hasActive} onClick={() => setOpen({ kind: "pick", move: "wasted" })}>
            <Trash2Icon aria-hidden="true" size={17} />
            Log wastage
          </Button>
          {/* Of whatever the view shows: ingredients on Stock levels, recipes on Recipes. */}
          <Button disabled={!items} onClick={() => setImporting(true)}>
            <UploadIcon aria-hidden="true" size={17} />
            Import
          </Button>
          <Button variant="attention" onClick={() => setOpen({ kind: "add" })}>
            <PlusIcon aria-hidden="true" size={18} />
            Add ingredient
          </Button>
        </div>
      </div>

      <div role="group" aria-label="Show" className="flex flex-wrap gap-2">
        {views.map((v) => (
          <button key={v.id} type="button" aria-pressed={view === v.id} className={pillClass(view === v.id)} onClick={() => setView(v.id)}>
            {v.label}
          </button>
        ))}
      </div>

      {api.items.isPending && <p className="text-base text-fg-muted">Loading the stock</p>}
      {api.items.error && (
        <EmptyState icon={<InboxIcon />} line={problemOf(api.items.error).message} action={{ label: "Try again", onClick: () => void api.items.refetch() }} />
      )}
      {items && view === "levels" && (
        <div
          className={
            canReport ? "grid grid-cols-1 items-start gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,22rem)] min-[1700px]:grid-cols-[minmax(0,1fr)_400px]" : ""
          }
        >
          <div className="flex min-w-0 flex-col gap-3">
            <StockLevels
              rows={tableRows(items, showRetired)}
              allRetired={items.length > 0 && !hasActive}
              onAction={(item, action) => setOpen(action.kind === "move" ? { kind: "move", item, move: action.move } : { kind: action.kind, item })}
            />
            {retiredCount(items) > 0 && (
              <div className="max-w-md">
                <CheckField label={`Show the ${retiredCount(items)} retired`} checked={showRetired} onChange={setShowRetired} />
              </div>
            )}
            <p className="text-sm text-fg-muted">
              In stock is what was received, less what was sold, wasted and corrected. A count records the difference. The bar is full at three times the low
              level.
            </p>
          </div>
          {canReport && <WastageCard />}
        </div>
      )}
      {items && view === "recipes" && menu.data && (
        <div className="max-w-6xl">
          <RecipesView menu={menu.data} stock={items} api={api} onSaved={(name) => toast(`Saved what ${name} takes.`, { tone: "success" })} />
        </div>
      )}
      {items && view === "recipes" && !menu.data && <p className="text-base text-fg-muted">Loading the menu</p>}

      {importing && view === "levels" && <ItemsImportModal api={api} onClose={() => setImporting(false)} />}
      {importing && view === "recipes" && <RecipesImportModal api={api} onClose={() => setImporting(false)} />}
      {open && api.items.data && (
        <Opened open={open} items={api.items.data.items} currency={api.items.data.currency} api={api} onOpen={setOpen} onClose={() => setOpen(null)} />
      )}
    </div>
  );
}

interface OpenedProps {
  open: Open;
  items: readonly StockItem[];
  /** The venue currency, which stock is costed in. */
  currency: string;
  api: StockApi;
  onOpen: (o: Open) => void;
  onClose: () => void;
}

function Opened({ open, items, currency, api, onOpen, onClose }: OpenedProps) {
  const toast = useToast();
  switch (open.kind) {
    case "add":
      return (
        <StockItemModal
          item={null}
          currency={currency}
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
          currency={currency}
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
    case "pick":
      return (
        <Modal open onClose={onClose} title={open.move === "counted" ? "Count which ingredient?" : "Log wastage of which ingredient?"} size="sm">
          <ul className="grid max-h-[60vh] gap-1 overflow-y-auto">
            {items
              .filter((it) => it.active)
              .map((it) => (
                <li key={it.id}>
                  <button
                    type="button"
                    onClick={() => onOpen({ kind: "move", item: it, move: open.move })}
                    className="flex min-h-tap w-full items-center justify-between gap-3 rounded px-4 text-left text-base outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
                  >
                    <span className="min-w-0 truncate font-semibold">{it.name}</span>
                    <span className="shrink-0 font-mono text-sm tabular-nums text-fg-muted">{formatQty(it.on_hand, it.unit)}</span>
                  </button>
                </li>
              ))}
          </ul>
        </Modal>
      );
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
