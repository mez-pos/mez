// The permissions card of the Staff and roles screen (D20): the roles as
// pills, and the chosen one's permissions as switches, grouped, with Reset and
// Save role. What you may give is what you hold (ADR-115): a switch for a
// permission you lack cannot be moved, and the server decides in any case.
//
// The design lists eight switches; a role here grants any of the server's
// permissions, so they are listed in their groups. Its note says anything
// switched off asks for a manager's PIN: that is so for the corrections, and
// the note sits with that group.

import { useEffect, useState } from "react";
import type { Permission, Person, Role, RoleInput } from "../../../shared/api-types";
import { roleText } from "../../../shared/auth/format";
import { Button, Card } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { pillClass } from "../../../shared/ui/pill";
import { Field, inputClass } from "../FormFields";
import {
  appliesText,
  mayGive,
  permissionGroups,
  permissionWords,
  problemOf,
  roleChanged,
  roleFormFrom,
  toRoleInput,
  toggled,
  type Problem,
  type RoleForm,
} from "./people-logic";

interface RolePanelProps {
  roles: readonly Role[];
  role: Role;
  staff: readonly Person[];
  /** The signed-in person's own permissions: what they may give. */
  held: readonly Permission[];
  onRole: (id: number) => void;
  onSave: (id: number, input: RoleInput) => Promise<void>;
  onDelete: (role: Role) => Promise<void>;
}

export function RolePanel({ roles, role, staff, held, onRole, onSave, onDelete }: RolePanelProps) {
  const [form, setForm] = useState<RoleForm>(() => roleFormFrom(role));
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  // Another role, or this one's name or grants changed (saved here or on
  // another screen), is the form's new start. Not its people count: someone
  // joining the role must not wipe what is being typed.
  const saved = `${role.id}|${role.name}|${role.permissions.join(",")}`;
  useEffect(() => {
    setForm(roleFormFrom(role));
    setProblem(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `saved` is the role's content
  }, [saved]);

  // A role that grants what you do not hold is not yours to change (ADR-115): the server would refuse.
  const mine = mayGive(held, role.permissions);

  const changed = roleChanged(role, form);
  const run = (fn: () => Promise<void>) => {
    setBusy(true);
    setProblem(null);
    fn()
      .catch((err: unknown) => setProblem(problemOf(err)))
      .finally(() => setBusy(false));
  };

  return (
    <Card padding="lg" className="flex min-w-0 flex-col">
      <div role="group" aria-label="Roles" className="flex flex-wrap gap-2">
        {roles.map((r) => (
          <button key={r.id} type="button" aria-pressed={r.id === role.id} className={pillClass(r.id === role.id)} onClick={() => onRole(r.id)}>
            {roleText(r.name)}
            <span className="font-mono text-sm tabular-nums" aria-label={r.people === 1 ? "1 person" : `${r.people} people`}>
              {r.people}
            </span>
          </button>
        ))}
      </div>

      <form
        aria-label={`Permissions of ${role.name}`}
        className="mt-5 flex flex-1 flex-col"
        onSubmit={(e) => {
          e.preventDefault();
          run(() => onSave(role.id, toRoleInput(form)));
        }}
      >
        <h2 className="font-serif text-xl">Permissions · {roleText(role.name)}</h2>
        <p className="mt-1 text-sm text-fg-muted">{appliesText(staff, role)}</p>
        {!mine && (
          <p className="mt-3 rounded bg-chip-warning-bg px-3 py-2.5 text-sm leading-relaxed text-chip-warning-fg">
            This role can do things you cannot, so it is not yours to change.
          </p>
        )}

        {problem && problem.field !== "name" && (
          <p role="alert" className="mt-3 text-base text-danger">
            {problem.message}
          </p>
        )}

        <div className="mt-4">
          <Field label="Role name" error={problem?.field === "name" ? problem.message : null}>
            {(id) => (
              <input
                id={id}
                className={inputClass}
                value={form.name}
                maxLength={40}
                disabled={!mine}
                onChange={(e) => {
                  setForm((f) => ({ ...f, name: e.target.value }));
                  setProblem(null);
                }}
              />
            )}
          </Field>
        </div>

        {permissionGroups.map((g) => (
          <fieldset key={g.title} className="mt-5">
            <legend className="text-meta font-bold uppercase text-fg-faint">{g.title}</legend>
            {g.permissions.map((p) => (
              <Switch
                key={p}
                label={permissionWords[p]}
                on={form.permissions.includes(p)}
                disabled={!mine || !held.includes(p)}
                onChange={(on) => {
                  setForm((f) => toggled(f, p, on));
                  setProblem(null);
                }}
              />
            ))}
            {g.note && <p className="mt-1 rounded bg-chip-warning-bg px-3 py-2.5 text-sm leading-relaxed text-chip-warning-fg">{g.note}</p>}
            {g.permissions.some((p) => !held.includes(p)) && (
              <p className="mt-1 text-sm text-fg-muted">You don't have the greyed ones, so you can't give them.</p>
            )}
          </fieldset>
        ))}

        <div className="mt-6 flex flex-wrap gap-2.5">
          <Button
            className="flex-1"
            disabled={busy || !changed}
            onClick={() => {
              setForm(roleFormFrom(role));
              setProblem(null);
            }}
          >
            Reset
          </Button>
          <Button type="submit" variant="primary" className="flex-1" disabled={busy || !changed || !mine}>
            {busy ? "Saving" : "Save role"}
          </Button>
        </div>
        {role.people === 0 && mine && (
          <Button variant="danger" className="mt-2.5" disabled={busy} onClick={() => run(() => onDelete(role))}>
            Delete this role
          </Button>
        )}
      </form>
    </Card>
  );
}

interface SwitchProps {
  label: string;
  on: boolean;
  disabled: boolean;
  onChange: (on: boolean) => void;
}

/** A switch, as the design draws a permission: the whole row is the target, 48px tall, and it says On or Off as well as showing it. */
function Switch({ label, on, disabled, onChange }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      disabled={disabled}
      onClick={() => onChange(!on)}
      className={cx(
        "flex min-h-tap w-full items-center gap-3 rounded text-left text-base outline-offset-2 outline-line-focus focus-visible:outline-2",
        disabled && "text-fg-muted",
      )}
    >
      <span className="min-w-0 flex-1">{label}</span>
      {/* The word is for the eye; aria-checked says it to a screen reader. */}
      <span aria-hidden="true" className="w-7 text-right text-sm text-fg-muted">
        {on ? "On" : "Off"}
      </span>
      <span
        aria-hidden="true"
        className={cx("flex h-7.5 w-13 shrink-0 items-center rounded-pill p-0.5", on ? "justify-end bg-accent" : "bg-line-strong", disabled && "opacity-50")}
      >
        <span className="size-6 rounded-round bg-raised" />
      </span>
    </button>
  );
}
