// The Choices view of the menu editor (backoffice-menu.md): every modifier
// group, what it asks for, its choices and how many items offer it.

import { useState } from "react";
import type { EditorGroup, MenuEditor } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon, Money } from "../../../shared/ui";
import { DataTable, type Column } from "../../../shared/ui/DataTable";
import { cx } from "../../../shared/ui/cx";
import { useOnMirror } from "../../../shared/use-mirror";
import { groupProblemOf, ruleText, usedBy } from "./group-logic";
import type { MenuEditorApi } from "./use-menu-editor";

interface GroupsViewProps {
  menu: MenuEditor;
  api: MenuEditorApi;
  onEdit: (group: EditorGroup) => void;
  onProblem: (message: string) => void;
}

export function GroupsView({ menu, api, onEdit, onProblem }: GroupsViewProps) {
  const [busy, setBusy] = useState(false);
  // A mirror lists the groups, which says all there is of each, and changes none (ADR-330).
  const mirror = useOnMirror();
  const activate = (g: EditorGroup) => {
    setBusy(true);
    api
      .setGroupActive(g.id, !g.active)
      .catch((err: unknown) => onProblem(groupProblemOf(err).message))
      .finally(() => setBusy(false));
  };

  const every: Column<EditorGroup>[] = [
    {
      id: "name",
      header: "Group",
      cell: (g) => (
        <span className="flex items-center gap-2">
          <span className={cx("font-semibold", !g.active && "text-fg-muted line-through")}>{g.name}</span>
          {!g.active && <span className="text-meta uppercase text-fg-faint">Retired</span>}
        </span>
      ),
    },
    { id: "rule", header: "Asks", cell: (g) => ruleText(g) },
    {
      id: "choices",
      header: "Choices",
      cell: (g) => (
        <ul className="flex flex-wrap gap-x-3 gap-y-0.5">
          {g.modifiers.map((m) => (
            <li key={m.id} className="whitespace-nowrap">
              {m.name}
              {m.price.minor !== 0 && (
                <>
                  {" "}
                  <Money amount={m.price} tone="muted" />
                </>
              )}
              {m.is_default && <span className="text-fg-muted"> (to start)</span>}
            </li>
          ))}
        </ul>
      ),
    },
    { id: "used", header: "Items", align: "right", cell: (g) => <span className="font-mono tabular-nums">{usedBy(menu, g.id)}</span> },
    {
      id: "actions",
      header: "",
      align: "right",
      cell: (g) => (
        <span className="flex justify-end gap-tap-gap">
          <Button disabled={busy} onClick={() => onEdit(g)}>
            Edit
          </Button>
          <Button disabled={busy} onClick={() => activate(g)}>
            {g.active ? "Retire" : "Bring back"}
          </Button>
        </span>
      ),
    },
  ];
  const columns = mirror ? every.filter((c) => c.id !== "actions") : every;

  return (
    <DataTable
      label="Groups of choices"
      columns={columns}
      rows={menu.modifier_groups}
      rowKey={(g) => String(g.id)}
      empty={
        <EmptyState
          icon={<InboxIcon />}
          line={mirror ? "No choices yet." : "No choices yet. Add a group, like Milk with Dairy and Oat, then offer it on items."}
        />
      }
    />
  );
}
