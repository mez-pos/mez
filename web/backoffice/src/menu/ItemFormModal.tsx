// Add or edit an item (backoffice-menu.md). A price the form cannot read is
// refused here; everything else is the server's to check, and its refusal
// shows under the field it is about. Editing a price says, before Save,
// that open tickets keep the price they were rung up at.

import { useState, type ReactNode } from "react";
import type { EditorItem, MenuEditor, MenuItemInput } from "../../../shared/api-types";
import { Button, Modal, Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { Field, inputClass } from "../FormFields";
import { ItemGroupsField } from "./ItemGroupsField";
import { ItemPhotoField, type PhotoChange } from "./ItemPhotoField";
import { choices, courseText, priceChange, problemOf, taxClassText, toItemInput, type ItemForm, type Problem } from "./menu-logic";

interface ItemFormModalProps {
  menu: MenuEditor;
  /** Null for a new item. */
  item: EditorItem | null;
  initial: ItemForm;
  /** The item, the groups it offers in order, and what to do to its photo. */
  onSave: (input: MenuItemInput, groups: number[], photo: PhotoChange) => Promise<void>;
  onClose: () => void;
}

export function ItemFormModal({ menu, item, initial, onSave, onClose }: ItemFormModalProps) {
  const [form, setForm] = useState(initial);
  const [groups, setGroups] = useState<number[]>(item?.modifier_groups ?? []);
  const [photo, setPhoto] = useState<PhotoChange>({ kind: "keep" });
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<ItemForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    setProblem(null);
  };
  const save = async () => {
    const built = toItemInput(form, menu.currency);
    if ("problem" in built) {
      setProblem(built.problem);
      return;
    }
    setBusy(true);
    try {
      await onSave(built.input, groups, photo);
      onClose();
    } catch (err) {
      setProblem(problemOf(err));
      setBusy(false);
    }
  };
  const on = (field: Problem["field"]) => (problem?.field === field ? problem.message : null);
  const change = priceChange(item, form.price);
  const categories = choices(menu.categories, form.categoryId);
  const stations = choices(menu.stations, form.stationId);
  const courses = choices(menu.courses, form.courseId);

  return (
    <Modal
      open
      onClose={onClose}
      title={item ? `Edit ${item.name}` : "Add an item"}
      size="md"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={busy} onClick={() => void save()}>
            {busy ? "Saving" : "Save"}
          </Button>
        </>
      }
    >
      <form
        className="flex flex-col gap-5"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        {problem && problem.field === null && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
        <Field label="Name" error={on("name")}>
          {(id) => <input id={id} className={inputClass} value={form.name} maxLength={60} autoFocus onChange={(e) => set({ name: e.target.value })} />}
        </Field>
        <Field label="Category" error={on("category_id")}>
          {(id) => (
            <Select id={id} value={form.categoryId} onChange={(categoryId) => set({ categoryId })}>
              {categories.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.active ? c.name : `${c.name} (retired)`}
                </option>
              ))}
            </Select>
          )}
        </Field>
        <Field label={`Price, ${menu.currency}`} hint="As the till charges it, like 180 or 180.50." error={on("price")}>
          {(id) => (
            <input
              id={id}
              className={cx(inputClass, "text-right font-mono tabular-nums")}
              value={form.price}
              inputMode="decimal"
              autoComplete="off"
              onChange={(e) => set({ price: e.target.value })}
            />
          )}
        </Field>
        {change && (
          <p role="status" className="rounded border border-line bg-sunken px-3 py-2.5 text-sm text-fg-2">
            Tickets already open keep <Money amount={change.was} />. Lines rung up after you save are <Money amount={change.now} />.
          </p>
        )}
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
        {(menu.needs_hsn || form.hsn !== "") && (
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
        <Field label="Made at" error={on("station_id")}>
          {(id) => (
            <Select id={id} value={form.stationId} onChange={(stationId) => set({ stationId })}>
              {stations.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.active ? s.name : `${s.name} (retired)`}
                </option>
              ))}
            </Select>
          )}
        </Field>
        <Field label="Course" error={on("course_id")}>
          {(id) => (
            <Select id={id} value={form.courseId} onChange={(courseId) => set({ courseId })}>
              {courses.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.active ? courseText(c) : `${courseText(c)} (retired)`}
                </option>
              ))}
            </Select>
          )}
        </Field>
        <ItemPhotoField
          current={item?.photo ?? ""}
          change={photo}
          onChange={(c) => {
            setPhoto(c);
            setProblem(null);
          }}
          error={on("photo")}
        />
        <ItemGroupsField
          menu={menu}
          chosen={groups}
          onChange={(ids) => {
            setGroups(ids);
            setProblem(null);
          }}
          error={problem?.field === "ids" ? problem.message : null}
        />
        {/* Enter in a field saves, as a form should; the visible Save is in the footer. */}
        <button type="submit" hidden />
      </form>
    </Modal>
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
