// The People screen's forms (backoffice-people.md): a person, a new PIN,
// and a role with its permissions in words. Each shows the server's
// refusal under its field, or over the form.

import { useState, type ReactNode } from "react";
import type { Permission, Person, PersonInput, Role, RoleInput } from "../../../shared/api-types";
import { Field, inputClass } from "../FormFields";
import { FormModal } from "../FormModal";
import {
  mayGive,
  newStaffForm,
  permissionGroups,
  permissionWords,
  pinProblem,
  problemOf,
  roleFormFrom,
  staffFormFrom,
  toggled,
  toPersonInput,
  toRoleInput,
  type Problem,
  type RoleForm,
  type StaffForm,
} from "./people-logic";

/** A modal form that saves, shows a refusal and closes on success. */
function useSave(onClose: () => void) {
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const save = async (fn: () => Promise<void>) => {
    setBusy(true);
    try {
      await fn();
      onClose();
    } catch (err) {
      setProblem(problemOf(err));
      setBusy(false);
    }
  };
  const on = (field: string) => (problem?.field === field ? problem.message : null);
  return { problem, setProblem, busy, save, on };
}

function Frame({
  title,
  busy,
  onSave,
  onClose,
  problem,
  size = "sm",
  children,
}: {
  title: string;
  busy: boolean;
  onSave: () => void;
  onClose: () => void;
  problem: Problem | null;
  size?: "sm" | "md";
  children: ReactNode;
}) {
  const own = problem !== null && problem.field !== null && ["name", "pin", "again", "role_id"].includes(problem.field);
  return (
    <FormModal title={title} size={size} busy={busy} onSave={onSave} onClose={onClose} refusal={problem && !own ? problem.message : null}>
      {children}
    </FormModal>
  );
}

function PinFields({
  pin,
  again,
  onChange,
  on,
}: {
  pin: string;
  again: string;
  onChange: (p: { pin?: string; again?: string }) => void;
  on: (f: string) => string | null;
}) {
  const props = { type: "password", inputMode: "numeric" as const, autoComplete: "new-password", maxLength: 8, className: `${inputClass} font-mono` };
  return (
    <>
      <Field label="PIN" hint="4 to 8 digits." error={on("pin")}>
        {(id) => <input id={id} {...props} value={pin} onChange={(e) => onChange({ pin: e.target.value })} />}
      </Field>
      <Field label="The PIN again" error={on("again")}>
        {(id) => <input id={id} {...props} value={again} onChange={(e) => onChange({ again: e.target.value })} />}
      </Field>
    </>
  );
}

interface StaffModalProps {
  person: Person | null;
  roles: readonly Role[];
  held: readonly Permission[];
  onSave: (input: PersonInput) => Promise<void>;
  onClose: () => void;
}

export function StaffModal({ person, roles, held, onSave, onClose }: StaffModalProps) {
  const [form, setForm] = useState<StaffForm>(person ? staffFormFrom(person) : newStaffForm(roles, held));
  const s = useSave(onClose);
  const set = (patch: Partial<StaffForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    s.setProblem(null);
  };
  const submit = () => {
    const built = toPersonInput(form, person === null);
    if ("problem" in built) {
      s.setProblem(built.problem);
      return;
    }
    void s.save(() => onSave(built.input));
  };

  return (
    <Frame title={person ? `Edit ${person.name}` : "Add a person"} busy={s.busy} onSave={submit} onClose={onClose} problem={s.problem}>
      <Field label="Name" error={s.on("name")}>
        {(id) => <input id={id} className={inputClass} value={form.name} maxLength={40} autoFocus onChange={(e) => set({ name: e.target.value })} />}
      </Field>
      <Field label="Role" error={s.on("role_id")}>
        {(id) => (
          <select id={id} className={inputClass} value={form.roleId} onChange={(e) => set({ roleId: Number(e.target.value) })}>
            {form.roleId === 0 && <option value={0}>Choose a role</option>}
            {roles.map((r) => {
              const mine = mayGive(held, r.permissions);
              return (
                <option key={r.id} value={r.id} disabled={!mine && r.id !== form.roleId}>
                  {mine ? r.name : `${r.name} (not yours to give)`}
                </option>
              );
            })}
          </select>
        )}
      </Field>
      {!person && <PinFields pin={form.pin} again={form.again} onChange={set} on={s.on} />}
    </Frame>
  );
}

export function PinModal({ person, onSave, onClose }: { person: Person; onSave: (pin: string) => Promise<void>; onClose: () => void }) {
  const [pins, setPins] = useState({ pin: "", again: "" });
  const s = useSave(onClose);
  const submit = () => {
    const problem = pinProblem(pins.pin, pins.again);
    if (problem) {
      s.setProblem(problem);
      return;
    }
    void s.save(() => onSave(pins.pin));
  };
  return (
    <Frame title={`New PIN for ${person.name}`} busy={s.busy} onSave={submit} onClose={onClose} problem={s.problem}>
      <p className="text-base text-fg-2">They will be signed out of every till and back office now.</p>
      <PinFields
        pin={pins.pin}
        again={pins.again}
        onChange={(p) => {
          setPins((x) => ({ ...x, ...p }));
          s.setProblem(null);
        }}
        on={s.on}
      />
    </Frame>
  );
}

interface RoleModalProps {
  role: Role | null;
  held: readonly Permission[];
  onSave: (input: RoleInput) => Promise<void>;
  onClose: () => void;
}

export function RoleModal({ role, held, onSave, onClose }: RoleModalProps) {
  const [form, setForm] = useState<RoleForm>(roleFormFrom(role));
  const s = useSave(onClose);

  return (
    <Frame
      title={role ? `Edit ${role.name}` : "Add a role"}
      busy={s.busy}
      onSave={() => void s.save(() => onSave(toRoleInput(form)))}
      onClose={onClose}
      problem={s.problem}
      size="md"
    >
      <Field label="Name" error={s.on("name")}>
        {(id) => (
          <input
            id={id}
            className={inputClass}
            value={form.name}
            maxLength={40}
            autoFocus
            onChange={(e) => {
              setForm((f) => ({ ...f, name: e.target.value }));
              s.setProblem(null);
            }}
          />
        )}
      </Field>
      {permissionGroups.map((g) => {
        const someNotMine = g.permissions.some((p) => !held.includes(p));
        return (
          <fieldset key={g.title} className="flex flex-col gap-0.5">
            <legend className="mb-1 text-base font-semibold">{g.title}</legend>
            {g.note && <p className="mb-1 text-sm text-fg-muted">{g.note}</p>}
            {g.permissions.map((p) => {
              const mine = held.includes(p);
              return (
                <label key={p} className={`flex min-h-tap items-center gap-2.5 text-base ${mine ? "" : "text-fg-muted"}`}>
                  <input
                    type="checkbox"
                    className="size-5"
                    checked={form.permissions.includes(p)}
                    disabled={!mine}
                    onChange={(e) => {
                      setForm((f) => toggled(f, p, e.target.checked));
                      s.setProblem(null);
                    }}
                  />
                  {permissionWords[p]}
                </label>
              );
            })}
            {someNotMine && <p className="text-sm text-fg-muted">You don't have the greyed ones, so you can't give them.</p>}
          </fieldset>
        );
      })}
    </Frame>
  );
}
