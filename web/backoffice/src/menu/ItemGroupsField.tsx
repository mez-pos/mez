// Which groups of choices an item offers, in the order its choice sheet
// shows them (backoffice-menu.md). A retired group it already offers stays
// listed, marked, so saving the item does not quietly drop it.

import { useId } from "react";
import type { MenuEditor } from "../../../shared/api-types";
import { Button } from "../../../shared/ui";
import { inputClass } from "../FormFields";
import { addableGroups, groupById, ruleText } from "./group-logic";
import { moved } from "./menu-logic";

interface ItemGroupsFieldProps {
  menu: MenuEditor;
  chosen: number[];
  onChange: (ids: number[]) => void;
  error: string | null;
}

export function ItemGroupsField({ menu, chosen, onChange, error }: ItemGroupsFieldProps) {
  const addId = useId();
  const addable = addableGroups(menu, chosen);
  const move = (index: number, delta: -1 | 1) => {
    const next = moved(chosen, index, delta);
    if (next) onChange(next);
  };

  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="mb-1.5 text-base font-semibold">Choices offered</legend>
      {chosen.length === 0 && <p className="text-sm text-fg-muted">None: it rings up with no questions.</p>}
      <ol className="flex flex-col gap-1.5">
        {chosen.map((id, i) => {
          const g = groupById(menu, id);
          const name = g?.name ?? "A group no longer on the menu";
          return (
            <li key={id} className="flex flex-wrap items-center gap-tap-gap rounded border border-line-soft px-2.5 py-1.5">
              <span className="min-w-0 flex-1 text-sm">
                <span className="font-semibold">{name}</span>
                {g && <span className="text-fg-3"> · {ruleText(g)}</span>}
                {g && !g.active && <span className="text-meta uppercase text-fg-faint"> Retired</span>}
              </span>
              <Button aria-label={`Move ${name} up`} disabled={i === 0} onClick={() => move(i, -1)}>
                <span aria-hidden="true">↑</span>
              </Button>
              <Button aria-label={`Move ${name} down`} disabled={i === chosen.length - 1} onClick={() => move(i, 1)}>
                <span aria-hidden="true">↓</span>
              </Button>
              <Button variant="ghost" onClick={() => onChange(chosen.filter((x) => x !== id))}>
                Remove
              </Button>
            </li>
          );
        })}
      </ol>
      {addable.length > 0 && (
        <div className="flex flex-col gap-1.5">
          <label htmlFor={addId} className="text-sm text-fg-2">
            Offer another
          </label>
          <select
            id={addId}
            className={inputClass}
            value=""
            onChange={(e) => {
              const id = Number(e.target.value);
              if (id) onChange([...chosen, id]);
            }}
          >
            <option value="">Choose a group</option>
            {addable.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}, {ruleText(g).toLowerCase()}
              </option>
            ))}
          </select>
        </div>
      )}
      {error && (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
    </fieldset>
  );
}
