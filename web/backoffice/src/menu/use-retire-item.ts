// Retiring an item stays one click, and the toast that says so takes it back
// (ADR-381 item 6, BO-6, ADR-387): an item is retired often, by the day's
// menu, and brought back as easily, so it is undone rather than asked about.
// Areas, tables and groups of choices are asked first (retire-logic.ts).

import type { EditorItem } from "../../../shared/api-types";
import { useToast } from "../../../shared/ui";
import { problemOf } from "./menu-logic";
import type { MenuEditorApi } from "./use-menu-editor";

/** Retires an item, then offers Undo on the toast. A failed Undo is said where the screen says its refusals. */
export function useRetireItem(api: MenuEditorApi, onProblem: (message: string) => void): (item: Pick<EditorItem, "id" | "name">) => Promise<void> {
  const toast = useToast();
  return async (item) => {
    await api.setItemActive(item.id, false);
    toast(`Retired ${item.name}. It is off every order screen.`, {
      button: {
        label: "Undo",
        run: () => {
          api
            .setItemActive(item.id, true)
            .then(() => toast(`${item.name} is back on the menu.`, { tone: "success" }))
            .catch((err: unknown) => onProblem(problemOf(err).message));
        },
      },
    });
  };
}
