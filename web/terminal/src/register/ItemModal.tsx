// The item modal (D04): what was tapped, across the middle of the order screen.
// On the left its photo, from 1024 wide, when it has one: with none there is
// no left pane at all (TILL-13), so the choices, the course and the note get
// the room and the dialog is only as wide as they need. On the right its name
// and price, each group of choices saying what it asks for, the course and a
// note for the kitchen, and along the bottom how many, Cancel and "Add to
// Order" with what the line will cost. Its defaults are chosen for a new line;
// a line being changed shows its own choices and quantity. The button stays
// shut until every required group is met, and says which is not (ADR-153).

import { CheckIcon, CrownIcon, PlusIcon } from "lucide-react";
import { useId, useState } from "react";
import type { CourseResponse, LineModifier, LineResponse, MenuItemResponse, ModifierGroupResponse } from "../../../shared/api-types";
import { photoUrl } from "../../../shared/menu";
import { Button, CloseIcon, Money, Qty } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useDialog } from "../../../shared/ui/use-dialog";
import { DietMark } from "../../../shared/ui/DietMark";
import { firstUnmet, groupHint, groupState, initialSelection, lineTotal, toggle, toModifiers, type GroupState } from "../../../shared/modifier-selection";

/** What the modal hands back: the line as the person wants it. */
export interface ItemChoice {
  modifiers: LineModifier[];
  qty: number;
  course: number;
  /** For the kitchen, about this line. Empty when nothing was written. */
  note: string;
}

interface ItemModalProps {
  item: MenuItemResponse;
  groups: readonly ModifierGroupResponse[];
  /** The venue's courses. The course is chosen only for a new line, and only when there is more than one. */
  courses: readonly CourseResponse[];
  /** The line being changed; absent for a new one. */
  line?: LineResponse | undefined;
  /** Why the last attempt was refused, from the server. */
  error?: string | null;
  /** Why saving cannot work now, seen before trying: the line moved on. */
  blocked?: string | null;
  busy?: boolean;
  onConfirm: (choice: ItemChoice) => void;
  onClose: () => void;
}

/** One order line holds at most this many of an item from the modal; more is a second line. */
const qtyBounds = { min: 1, max: 99 } as const;
const noteLength = 200;

const field = "rounded border border-line-strong bg-raised text-fg outline-accent-tint focus-visible:border-accent focus-visible:outline-3";

export function ItemModal({ item, groups, courses, line, error, blocked, busy = false, onConfirm, onClose }: ItemModalProps) {
  const ref = useDialog(true, onClose);
  const titleId = useId();
  const editing = line !== undefined;
  const [selection, setSelection] = useState(() => initialSelection(groups, line ? (line.modifiers ?? []) : undefined));
  const [qty, setQty] = useState(line?.qty ?? 1);
  const [course, setCourse] = useState(line?.course ?? item.course);
  const [note, setNote] = useState("");

  const chosen = toModifiers(groups, selection);
  const unmet = firstUnmet(groups, selection);
  const shut = unmet !== null || busy || Boolean(blocked);
  const photo = item.photo ? photoUrl(item.photo) : null;

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      className={cx(
        "m-auto max-h-[calc(100dvh-2rem)] w-full overflow-hidden rounded-sheet bg-raised p-0 text-fg shadow-modal",
        photo ? "max-w-[min(63.75rem,calc(100vw-2rem))]" : "max-w-[min(42rem,calc(100vw-2rem))]",
      )}
    >
      <div className="flex max-h-[calc(100dvh-2rem)]">
        {photo && (
          // The rail's width: the drawing's 400px on a wide screen, less on a small one.
          <div className="relative hidden w-rail shrink-0 bg-sunken lg:block">
            {/* The name is beside it, so the photo says nothing a reader needs. */}
            <img src={photo} alt="" decoding="async" className="absolute inset-0 size-full object-cover" />
            {item.popular && <PopularPill className="absolute left-4 top-4" />}
          </div>
        )}

        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-start justify-between gap-5 px-7 pt-6 short:pt-5">
            <div className="min-w-0">
              <div className="flex items-start gap-2.5">
                <span className="pt-2.5">
                  <DietMark diet={item.diet} />
                </span>
                <h2 id={titleId} className="font-serif text-total leading-tight">
                  {item.name}
                </h2>
              </div>
              {item.description && <p className="mt-2 max-w-md text-base text-fg-muted">{item.description}</p>}
              {/* On the photo when there is one to wear it; here otherwise. */}
              {item.popular && <PopularPill className={cx("mt-3", photo && "lg:hidden")} />}
            </div>
            <div className="flex shrink-0 items-start gap-4">
              <Money amount={item.price} variant="total" className="leading-tight" />
              <button
                type="button"
                aria-label="Close"
                onClick={onClose}
                className="flex size-tap items-center justify-center rounded-round bg-sunken text-fg outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
              >
                <CloseIcon />
              </button>
            </div>
          </div>

          <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-7 py-6 short:gap-5 short:py-4">
            {groups.map((group) => (
              <GroupPicker
                key={group.id}
                group={group}
                picked={selection.get(group.id) ?? []}
                state={groupState(group, selection)}
                onTap={(id) => setSelection((s) => toggle(s, group, id))}
              />
            ))}

            {!editing && courses.length > 1 && (
              <label className="flex flex-col gap-3 sm:w-56">
                <span className="font-serif text-xl">Course</span>
                <select value={course} onChange={(e) => setCourse(Number(e.target.value))} className={cx(field, "h-tap px-4 font-semibold")}>
                  {courses.map((c) => (
                    <option key={c.id} value={c.number}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </label>
            )}

            <label className="flex flex-col gap-3">
              <span className="font-serif text-xl">Kitchen note</span>
              <textarea
                value={note}
                onChange={(e) => setNote(e.target.value)}
                maxLength={noteLength}
                rows={2}
                placeholder="e.g. well done, no onions, serve after mains"
                // Controls are 16px on every surface (tokens.css), so no size here.
                className={cx(field, "px-4 py-3 placeholder:text-fg-muted")}
              />
            </label>
          </div>

          <div className="border-t border-line px-7 py-5 short:py-4">
            <p className="min-h-6 pb-2 text-sm" role="status">
              {blocked ? (
                <span className="text-danger">{blocked}</span>
              ) : error ? (
                <span className="text-danger">{error}</span>
              ) : unmet ? (
                <span className="text-fg-muted">Choose {unmet.name.toLowerCase()} to carry on.</span>
              ) : null}
            </p>
            <div className="flex flex-wrap items-center gap-4">
              <Qty value={qty} onChange={setQty} bounds={qtyBounds} label={item.name} className="shrink-0" />
              <Button size="charge" className="w-36" onClick={onClose}>
                Cancel
              </Button>
              <Button
                variant="primary"
                size="charge"
                className="min-w-56 flex-1"
                disabled={shut}
                onClick={() => onConfirm({ modifiers: chosen, qty, course, note: note.trim() })}
              >
                {!editing && <PlusIcon aria-hidden="true" size={19} strokeWidth={2.2} />}
                <span>{editing ? "Save changes" : "Add to Order"}</span>
                <span aria-hidden="true">·</span>
                <Money amount={lineTotal(item.price, chosen, qty)} />
              </Button>
            </div>
          </div>
        </div>
      </div>
    </dialog>
  );
}

function PopularPill({ className }: { className?: string | undefined }) {
  return (
    <span className={cx("inline-flex h-8 items-center gap-1.5 rounded-pill bg-highlight px-3 text-sm font-bold text-fg-heading", className)}>
      <CrownIcon aria-hidden="true" size={15} />
      Popular
    </span>
  );
}

interface GroupPickerProps {
  group: ModifierGroupResponse;
  picked: readonly number[];
  state: GroupState;
  onTap: (modifierId: number) => void;
}

/** One group: a radio tile each where one is chosen, a checkbox tile each where several may be. */
function GroupPicker({ group, picked, state, onTap }: GroupPickerProps) {
  const required = group.min_select > 0;
  const single = group.max_select === 1;
  return (
    <fieldset>
      <legend className="flex w-full items-baseline justify-between gap-4">
        <span className="font-serif text-xl">{group.name}</span>
        <span className={cx("text-sm", required ? "font-bold text-danger" : "text-fg-muted")}>{groupHint(group)}</span>
      </legend>
      <div
        role={single ? "radiogroup" : "group"}
        aria-label={group.name}
        className={cx("mt-3 grid gap-3", single ? "grid-cols-2 sm:grid-cols-3" : "grid-cols-1 sm:grid-cols-2")}
      >
        {group.modifiers.map((m) => {
          const on = picked.includes(m.id);
          const full = !on && !single && state.full;
          const extra = m.price.minor > 0 && (
            <span className={cx("flex items-baseline text-sm", on ? "text-accent-text" : "text-fg-muted", !single && "font-semibold")}>
              +<Money amount={m.price} />
            </span>
          );
          return (
            <button
              key={m.id}
              type="button"
              role={single ? "radio" : "checkbox"}
              aria-checked={on}
              aria-disabled={full}
              onClick={() => onTap(m.id)}
              className={cx(
                "flex min-h-tap items-center gap-3 rounded border px-3.5 py-3 text-left text-base",
                "outline-offset-2 outline-line-focus focus-visible:outline-2",
                on ? "border-accent bg-accent-tint text-accent-text" : "border-line bg-raised text-fg hover:bg-raised-high",
                full && "opacity-40",
              )}
            >
              <span
                aria-hidden="true"
                className={cx(
                  "flex size-5 shrink-0 items-center justify-center border-2",
                  single ? "rounded-round" : "rounded-[6px]",
                  on ? (single ? "border-accent" : "border-accent bg-accent text-accent-fg") : "border-line-strong",
                )}
              >
                {on && (single ? <span className="size-2.5 rounded-round bg-accent" /> : <CheckIcon size={14} strokeWidth={3.2} />)}
              </span>
              {single ? (
                <span className="min-w-0">
                  <span className="block font-bold">{m.name}</span>
                  {extra}
                </span>
              ) : (
                <>
                  <span className={cx("min-w-0 flex-1", on && "font-semibold")}>{m.name}</span>
                  {extra}
                </>
              )}
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}
