// One stock item's history, newest first, a page at a time
// (backoffice-stock.md).

import type { StockItem } from "../../../shared/api-types";
import { Button, Modal } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { formatDelta, formatQty } from "./qty";
import { movementText, problemOf } from "./stock-logic";
import { useHistory } from "./use-stock";

export function HistoryModal({ item, onClose }: { item: StockItem; onClose: () => void }) {
  const history = useHistory(item.id);
  const movements = history.data?.pages.flatMap((p) => p.movements) ?? [];

  return (
    <Modal open onClose={onClose} title={`${item.name}: history`} size="md" footer={<Button onClick={onClose}>Close</Button>}>
      <p className="mb-3 text-sm text-fg-3">
        On hand now <span className="font-mono">{formatQty(item.on_hand, item.unit)}</span>
      </p>
      {history.isPending && <p className="text-base text-fg-muted">Loading the history</p>}
      {history.error && (
        <p role="alert" className="text-base text-danger">
          {problemOf(history.error).message}
        </p>
      )}
      {history.data && movements.length === 0 && <p className="text-base text-fg-muted">Nothing recorded yet.</p>}
      <ol className="flex flex-col">
        {movements.map((m) => (
          <li key={m.id} className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5 border-b border-line-soft py-2.5 text-sm last:border-b-0">
            <span className="min-w-0 flex-1">
              <span className="font-semibold">{movementText(m, item.unit)}</span>
              <span className="text-fg-3">
                {" "}
                · {m.staff_name} · {new Date(m.at).toLocaleString()}
              </span>
              {m.note && <span className="block text-fg-2">{m.note}</span>}
            </span>
            <span className={cx("font-mono tabular-nums", m.delta < 0 ? "text-danger" : "text-fg")}>{formatDelta(m.delta, item.unit)}</span>
          </li>
        ))}
      </ol>
      {history.hasNextPage && (
        <Button className="mt-3" disabled={history.isFetchingNextPage} onClick={() => void history.fetchNextPage()}>
          {history.isFetchingNextPage ? "Loading" : "Show older"}
        </Button>
      )}
    </Modal>
  );
}
