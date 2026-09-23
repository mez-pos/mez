// The modifier sheet (terminal-register.md, Task 3): an item's groups, each
// saying how many to choose, its defaults already chosen for a new line or
// the line's own choices when editing one. The price shown is the item with
// these choices, live; the button stays shut until every required group is
// met, and says which one is not.

import { useState } from "react";
import type { LineModifier, MenuItemResponse, ModifierGroupResponse } from "../../../shared/api-types";
import { Button, Money, Sheet } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { firstUnmet, groupState, initialSelection, toggle, toModifiers, unitPrice } from "./modifier-selection";

interface ModifierSheetProps {
  item: MenuItemResponse;
  groups: readonly ModifierGroupResponse[];
  /** The line's current modifiers when editing; absent for a new line. */
  current?: readonly LineModifier[] | undefined;
  /** Why the last attempt was refused, from the server. */
  error?: string | null;
  /** Why saving cannot work now, seen before trying: the line moved on. */
  blocked?: string | null;
  busy?: boolean;
  onConfirm: (modifiers: LineModifier[]) => void;
  onClose: () => void;
}

export function ModifierSheet({ item, groups, current, error, blocked, busy = false, onConfirm, onClose }: ModifierSheetProps) {
  const [selection, setSelection] = useState(() => initialSelection(groups, current));
  const chosen = toModifiers(groups, selection);
  const unmet = firstUnmet(groups, selection);
  const editing = current !== undefined;

  const footer = (
    <div className="flex flex-col gap-2">
      <p className="min-h-6 text-sm" role="status">
        {blocked ? (
          <span className="text-danger">{blocked}</span>
        ) : error ? (
          <span className="text-danger">{error}</span>
        ) : unmet ? (
          <span className="text-fg-muted">Choose {unmet.name.toLowerCase()} to carry on.</span>
        ) : null}
      </p>
      <Button
        variant="primary"
        size="charge"
        className="w-full justify-between"
        disabled={unmet !== null || busy || Boolean(blocked)}
        onClick={() => onConfirm(chosen)}
      >
        <span>{editing ? "Save changes" : "Add to ticket"}</span>
        <Money amount={unitPrice(item.price, chosen)} />
      </Button>
    </div>
  );

  return (
    <Sheet open onClose={onClose} title={item.name} footer={footer}>
      <div className="flex flex-col gap-6">
        {groups.map((group) => (
          <GroupPicker
            key={group.id}
            group={group}
            picked={selection.get(group.id) ?? []}
            state={groupState(group, selection)}
            onTap={(id) => setSelection((s) => toggle(s, group, id))}
          />
        ))}
      </div>
    </Sheet>
  );
}

interface GroupPickerProps {
  group: ModifierGroupResponse;
  picked: readonly number[];
  state: ReturnType<typeof groupState>;
  onTap: (modifierId: number) => void;
}

function GroupPicker({ group, picked, state, onTap }: GroupPickerProps) {
  const required = group.min_select > 0;
  const single = group.max_select === 1;
  return (
    <fieldset>
      <legend className="flex w-full items-baseline justify-between gap-4">
        <span className="text-lg font-semibold">{group.name}</span>
        <span className={cx("text-meta uppercase", required && !state.satisfied ? "text-accent-text" : "text-fg-faint")}>
          {required ? "Required" : "Optional"} · {state.rule}
        </span>
      </legend>
      <div role={single ? "radiogroup" : "group"} aria-label={group.name} className="mt-3 grid grid-cols-2 gap-tap-gap">
        {group.modifiers.map((m) => {
          const on = picked.includes(m.id);
          const blocked = !on && !single && state.full;
          return (
            <button
              key={m.id}
              type="button"
              role={single ? "radio" : "checkbox"}
              aria-checked={on}
              aria-disabled={blocked}
              onClick={() => onTap(m.id)}
              className={cx(
                "flex min-h-tap items-center justify-between gap-3 rounded border px-4 py-2.5 text-left text-base",
                "outline-offset-2 outline-line-focus focus-visible:outline-2",
                on ? "border-line-focus bg-raised-high font-semibold text-fg" : "border-line bg-raised text-fg-2",
                blocked && "opacity-40",
              )}
            >
              <span>{m.name}</span>
              {m.price.minor > 0 && (
                <span className="flex items-baseline gap-0.5 text-sm text-fg-muted">
                  +<Money amount={m.price} />
                </span>
              )}
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}
