// The editor of the menu screen (D18): an item's fields in a card, with its
// photo and its tags beside it (under it on a narrower screen), and Cancel and
// Save in a bar at the editor's foot that stays in sight while the form runs
// on below the screen (ADR-381). A new item is the same form with nothing
// saved yet. A price the form cannot read is refused here; everything else is
// the server's to check, and its refusal shows under the field it is about.
//
// Left out of the design: Cost price (a dish's cost is its recipe's, ADR-189),
// "Show in online ordering" and "Happy-hour pricing" (neither exists), the
// Nutrition tab, and stock kept per menu item (stock is by ingredient, on the
// Stock screen). "Delete item" is Retire: an item with sales is never deleted.

import { useState, type ReactNode } from "react";
import type { EditorItem, MenuEditor, MenuItemInput } from "../../../shared/api-types";
import type { MenuItemState } from "../../../shared/ui/status";
import { Button, Card, Money, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { OnAMirror } from "../../../shared/shell/OnAMirror";
import { useOnMirror } from "../../../shared/use-mirror";
import { useVenueClock } from "../../../shared/use-venue";
import { venueDateTime } from "../../../shared/venue-clock";
import { CheckField, Choice, Field, inputClass } from "../FormFields";
import { useDraftHeld } from "../shell/leave-guard";
import { groupsChanged } from "./group-logic";
import { ItemGroupsField } from "./ItemGroupsField";
import { ItemPhotoField, type PhotoChange } from "./ItemPhotoField";
import { choices, courseText, dietOptions, formChanged, priceChange, problemOf, taxClassText, toItemInput, type ItemForm, type Problem } from "./menu-logic";

/** The longest description the server takes, in characters (menus.MaxDescriptionLength). */
const MAX_DESCRIPTION = 140;

type Tab = "choices" | "kitchen";
const tabs: readonly { id: Tab; label: string }[] = [
  { id: "choices", label: "Choices" },
  { id: "kitchen", label: "Kitchen and tax" },
];
/** The tab a refusal's field is on, so a refusal is never out of sight. */
const tabOf: Partial<Record<NonNullable<Problem["field"]>, Tab>> = { ids: "choices", station_id: "kitchen", course_id: "kitchen" };

interface ItemEditorProps {
  menu: MenuEditor;
  /** Null for a new item. */
  item: EditorItem | null;
  /** Where a saved item stands, for the chip and the buttons beside it. */
  state: MenuItemState | null;
  /** When a sold-out item is back on sale, or null. */
  soldOutUntil: string | null;
  initial: ItemForm;
  /** Saves the item, the groups it offers in order, and what to do to its photo. */
  onSave: (input: MenuItemInput, groups: number[], photo: PhotoChange) => Promise<void>;
  onCancel: () => void;
  /** Sold out for the day, or back on sale. Only for a saved item that is not retired. */
  onSoldOut: (soldOut: boolean) => void;
  /** Retire, or bring back. Only for a saved item. */
  onActive: (active: boolean) => void;
  /** An action on the item is on its way. */
  busy: boolean;
}

export function ItemEditor({ menu, item, state, soldOutUntil, initial, onSave, onCancel, onSoldOut, onActive, busy }: ItemEditorProps) {
  const { timezone } = useVenueClock();
  const [form, setForm] = useState(initial);
  const [groups, setGroups] = useState<number[]>(item?.modifier_groups ?? []);
  const [photo, setPhoto] = useState<PhotoChange>({ kind: "keep" });
  const [problem, setProblem] = useState<Problem | null>(null);
  const [saving, setSaving] = useState(false);
  const [tab, setTab] = useState<Tab>("choices");
  // On a mirror the item is read: its fields are disabled and nothing saves
  // (ADR-330). The two tabs stay live, or half the item could not be read.
  const readOnly = useOnMirror();
  const set = (patch: Partial<ItemForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    setProblem(null);
  };
  const refuse = (p: Problem) => {
    setProblem(p);
    const where = p.field ? tabOf[p.field] : undefined;
    if (where) setTab(where);
  };
  const save = async () => {
    const built = toItemInput(form, menu.currency);
    if ("problem" in built) {
      refuse(built.problem);
      return;
    }
    setSaving(true);
    try {
      await onSave(built.input, groups, photo);
      // What was asked of the photo is done: the saved one is now the one to keep.
      setPhoto({ kind: "keep" });
    } catch (err) {
      refuse(problemOf(err));
    } finally {
      setSaving(false);
    }
  };
  const on = (field: Problem["field"]) => (problem?.field === field ? problem.message : null);
  const change = priceChange(item, form.price);
  const waiting = busy || saving;
  const unsaved = formChanged(initial, form, menu.currency) || groupsChanged(item, groups) || photo.kind !== "keep";
  // Another item, a new one, another view or another screen asks before this is lost (BO-2, ADR-387).
  useDraftHeld(unsaved && !readOnly, item ? item.name : "the new item");
  // One live region whose words change, as a settings section's: a screen reader hears each in turn.
  const saveWords = saving ? "Saving" : unsaved ? "Unsaved changes" : "";

  return (
    <form
      aria-label={item ? `Edit ${item.name}` : "Add a menu item"}
      onSubmit={(e) => {
        e.preventDefault();
        if (!readOnly) void save();
      }}
      className="flex min-w-0 flex-col gap-4"
    >
      {/* The photo and tags beside the fields only where the editor is wide enough for both. */}
      <div className="grid min-w-0 grid-cols-1 items-start gap-4 min-[1700px]:grid-cols-[minmax(0,1fr)_var(--side)]">
        <Card padding="lg" className="min-w-0 short:p-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="flex flex-wrap items-center gap-3">
              <h2 className="font-serif text-xl">{readOnly ? "Menu item" : item ? "Edit menu item" : "Add menu item"}</h2>
              {state && <StatusChip status={{ kind: "menu", status: state }} />}
              {state === "sold_out" && soldOutUntil && <span className="text-sm text-fg-muted">Back on sale {venueDateTime(soldOutUntil, timezone)}</span>}
            </div>
            {item && state && !readOnly && (
              <div className="flex flex-wrap gap-tap-gap">
                {state !== "retired" && (
                  <Button size="md" disabled={waiting} onClick={() => onSoldOut(state !== "sold_out")}>
                    {state === "sold_out" ? "Back on sale" : "Sold out today"}
                  </Button>
                )}
                {state === "retired" ? (
                  <Button size="md" disabled={waiting} onClick={() => onActive(true)}>
                    Bring back
                  </Button>
                ) : (
                  <Button size="md" variant="danger" disabled={waiting} onClick={() => onActive(false)}>
                    Retire item
                  </Button>
                )}
              </div>
            )}
          </div>

          {problem && problem.field === null && (
            <p role="alert" className="mt-4 text-base text-danger">
              {problem.message}
            </p>
          )}

          <fieldset disabled={readOnly} className="mt-5 grid min-w-0 gap-4 sm:grid-cols-2">
            <Field label="Item name" error={on("name")}>
              {(id) => <input id={id} className={inputClass} value={form.name} maxLength={60} required onChange={(e) => set({ name: e.target.value })} />}
            </Field>
            <Field label="Category" error={on("category_id")}>
              {(id) => (
                <Select id={id} value={form.categoryId} onChange={(categoryId) => set({ categoryId })}>
                  {choices(menu.categories, form.categoryId).map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.active ? c.name : `${c.name} (retired)`}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
            <Field label={`Price, ${menu.currency}`} hint="As the order screen charges it, like 180 or 180.50." error={on("price")}>
              {(id) => (
                <input
                  id={id}
                  className={cx(inputClass, "font-mono tabular-nums")}
                  value={form.price}
                  inputMode="decimal"
                  autoComplete="off"
                  onChange={(e) => set({ price: e.target.value })}
                />
              )}
            </Field>
            <Field label="Tax class" error={on("tax_class_id")}>
              {(id) => (
                <Select id={id} value={form.taxClassId} onChange={(taxClassId) => set({ taxClassId })}>
                  {menu.tax_classes
                    .filter((t) => t.active || t.id === form.taxClassId)
                    .map((t) => (
                      <option key={t.id} value={t.id}>
                        {taxClassText(t)}
                      </option>
                    ))}
                </Select>
              )}
            </Field>
            {(menu.needs_hsn || form.hsn !== "" || problem?.field === "hsn") && (
              <Field
                label="HSN or SAC code"
                hint={menu.needs_hsn ? "4, 6 or 8 digits. Every GST invoice line shows it." : "Not needed at this venue. You can leave it empty."}
                error={on("hsn")}
              >
                {(id) => (
                  <input
                    id={id}
                    className={cx(inputClass, "font-mono")}
                    value={form.hsn}
                    inputMode="numeric"
                    maxLength={8}
                    autoComplete="off"
                    onChange={(e) => set({ hsn: e.target.value })}
                  />
                )}
              </Field>
            )}
          </fieldset>
          {change && (
            <p role="status" className="mt-4 rounded border border-line bg-sunken px-3 py-2.5 text-sm text-fg-2">
              Orders already open keep <Money amount={change.was} />. Lines rung up after you save are <Money amount={change.now} />.
            </p>
          )}

          <fieldset disabled={readOnly} className="mt-4 min-w-0">
            <Field label="Description" error={on("description")}>
              {(id) => (
                <textarea
                  id={id}
                  rows={2}
                  className={cx(inputClass, "py-3 leading-relaxed")}
                  value={form.description}
                  maxLength={MAX_DESCRIPTION}
                  onChange={(e) => set({ description: e.target.value })}
                />
              )}
            </Field>
            <p className="mt-1 text-right font-mono text-sm tabular-nums text-fg-muted">
              {form.description.length} / {MAX_DESCRIPTION}
            </p>
          </fieldset>

          {/* Two buttons that show one part or the other: plain toggles, reached by Tab like any button. */}
          <div role="group" aria-label="More about the item" className="mt-4 flex w-fit max-w-full flex-wrap gap-1 rounded-pill bg-sunken p-1">
            {tabs.map((t) => (
              <button
                key={t.id}
                type="button"
                aria-pressed={tab === t.id}
                onClick={() => setTab(t.id)}
                className={cx(
                  "min-h-tap rounded-pill px-5 text-base outline-offset-2 outline-line-focus focus-visible:outline-2",
                  tab === t.id ? "bg-attention font-semibold text-attention-fg" : "hover:bg-raised-high",
                )}
              >
                {t.label}
              </button>
            ))}
          </div>

          <fieldset disabled={readOnly} hidden={tab !== "choices"} className="mt-4 min-w-0">
            <ItemGroupsField
              menu={menu}
              chosen={groups}
              onChange={(ids) => {
                setGroups(ids);
                setProblem(null);
              }}
              error={on("ids")}
            />
          </fieldset>
          <fieldset disabled={readOnly} hidden={tab !== "kitchen"} className="mt-4 grid min-w-0 gap-4 sm:grid-cols-2">
            <Field label="Made at" hint="Stations are set up under Stations and courses." error={on("station_id")}>
              {(id) => (
                <Select id={id} value={form.stationId} onChange={(stationId) => set({ stationId })}>
                  {choices(menu.stations, form.stationId).map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.active ? s.name : `${s.name} (retired)`}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
            <Field label="Course" hint="When it is sent to the kitchen." error={on("course_id")}>
              {(id) => (
                <Select id={id} value={form.courseId} onChange={(courseId) => set({ courseId })}>
                  {choices(menu.courses, form.courseId).map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.active ? courseText(c) : `${courseText(c)} (retired)`}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
          </fieldset>
        </Card>

        <fieldset disabled={readOnly} className="flex min-w-0 flex-col gap-4">
          <Card>
            <ItemPhotoField
              current={item?.photo ?? ""}
              change={photo}
              onChange={(c) => {
                setPhoto(c);
                setProblem(null);
              }}
              error={on("photo")}
            />
          </Card>
          <Card className="flex flex-col gap-3">
            <h3 className="text-base font-bold">Tags</h3>
            <Choice label="Mark on the card" value={form.diet} options={dietOptions} onChange={(diet) => set({ diet })} error={on("diet")} />
            <CheckField label="Gluten free" checked={form.glutenFree} onChange={(glutenFree) => set({ glutenFree })} />
            <CheckField label="Popular" hint="For the Popular tab on the order screen." checked={form.popular} onChange={(popular) => set({ popular })} />
          </Card>
          {readOnly && <OnAMirror />}
        </fieldset>
      </div>

      {/* Sticks to the foot of the screen while the editor runs on below it, and
          rests under the editor once its end is in view: Save is never scrolled to. */}
      <div
        hidden={readOnly}
        className="sticky bottom-0 z-10 flex flex-wrap items-center justify-end gap-x-4 gap-y-2 rounded-card border border-line bg-raised px-4 py-3 shadow-card short:py-2"
      >
        <p role="status" className="mr-auto text-base text-fg-2">
          {saveWords}
        </p>
        <div className="flex gap-tap-gap">
          <Button disabled={waiting} onClick={onCancel}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={waiting}>
            {saving ? "Saving" : item ? "Save changes" : "Add item"}
          </Button>
        </div>
      </div>
    </form>
  );
}

/** A native select of ids. With nothing to choose it says so, and sends 0 for the server to refuse. */
function Select({ id, value, onChange, children }: { id: string; value: number; onChange: (v: number) => void; children: ReactNode[] }) {
  return (
    <select id={id} className={inputClass} value={value} onChange={(e) => onChange(Number(e.target.value))}>
      {children.length === 0 ? <option value={0}>None set up yet</option> : children}
    </select>
  );
}
