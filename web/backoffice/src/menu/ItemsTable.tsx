// The items column of the menu editor (backoffice-menu.md): the selected
// category's items in a dense table, each with its edit, order and retire.

import { useState } from "react";
import type { EditorCategory, EditorItem, MenuEditor } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon, Money } from "../../../shared/ui";
import { DataTable, type Column } from "../../../shared/ui/DataTable";
import { cx } from "../../../shared/ui/cx";
import { itemsOf, moved, namesById, problemOf } from "./menu-logic";
import type { MenuEditorApi } from "./use-menu-editor";

interface ItemsTableProps {
  menu: MenuEditor;
  category: EditorCategory;
  api: MenuEditorApi;
  onEdit: (item: EditorItem) => void;
  onProblem: (message: string) => void;
}

export function ItemsTable({ menu, category, api, onEdit, onProblem }: ItemsTableProps) {
  const [busy, setBusy] = useState(false);
  const items = itemsOf(menu, category.id);
  const ids = items.map((i) => i.id);
  const names = namesById(menu);

  const run = (fn: () => Promise<void>) => {
    setBusy(true);
    fn()
      .catch((err: unknown) => onProblem(problemOf(err).message))
      .finally(() => setBusy(false));
  };
  const move = (index: number, delta: -1 | 1) => {
    const next = moved(ids, index, delta);
    if (next) run(() => api.orderItems(category.id, next));
  };

  const columns: Column<EditorItem>[] = [
    {
      id: "name",
      header: "Item",
      cell: (i) => (
        <span className="flex items-center gap-2">
          <span className={cx(!i.active && "text-fg-muted line-through")}>{i.name}</span>
          {!i.active && <span className="text-meta uppercase text-fg-faint">Retired</span>}
        </span>
      ),
    },
    { id: "price", header: "Price", align: "right", cell: (i) => <Money amount={i.price} tone={i.active ? "default" : "muted"} /> },
    { id: "tax", header: "Tax", cell: (i) => names.taxClass(i.tax_class_id) },
    ...(menu.needs_hsn ? [{ id: "hsn", header: "HSN", align: "right" as const, cell: (i: EditorItem) => <span className="font-mono tabular-nums">{i.hsn}</span> }] : []),
    { id: "station", header: "Made at", cell: (i) => names.station(i.station_id) },
    { id: "course", header: "Course", cell: (i) => names.course(i.course_id) },
    {
      id: "actions",
      header: "",
      align: "right",
      cell: (i) => {
        const index = ids.indexOf(i.id);
        return (
          <span className="flex justify-end gap-tap-gap">
            <Button disabled={busy} onClick={() => onEdit(i)}>
              Edit
            </Button>
            <Button disabled={busy || index === 0} aria-label={`Move ${i.name} up`} onClick={() => move(index, -1)}>
              <span aria-hidden="true">↑</span>
            </Button>
            <Button disabled={busy || index === ids.length - 1} aria-label={`Move ${i.name} down`} onClick={() => move(index, 1)}>
              <span aria-hidden="true">↓</span>
            </Button>
            <Button disabled={busy} onClick={() => run(() => api.setItemActive(i.id, !i.active))}>
              {i.active ? "Retire" : "Bring back"}
            </Button>
          </span>
        );
      },
    },
  ];

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-3">
      {!category.active && (
        <p className="text-sm text-fg-2">This category is retired: none of its items are on the till.</p>
      )}
      <DataTable
        label={`Items in ${category.name}`}
        columns={columns}
        rows={items}
        rowKey={(i) => String(i.id)}
        empty={<EmptyState icon={<InboxIcon />} line={`No items in ${category.name} yet.`} />}
      />
    </div>
  );
}
