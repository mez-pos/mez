// Staff and roles (D20): the staff in a table, and beside it the role of
// whoever is picked, with what it may do as switches. What you may give is
// what you hold (ADR-115): the server decides, the screen shows it.
//
// Left out of the design: the roster and the Shift column (there are no
// timesheets), "on shift now", a phone number, and the PIN column: a PIN is
// kept as a hash and none of it can be shown. A new PIN is set from a row's
// menu, and so is Unlock for someone too many wrong PINs locked (ADR-260).

import { UserPlusIcon } from "lucide-react";
import { useState } from "react";
import type { Person, Role } from "../../../shared/api-types";
import { initials, roleText } from "../../../shared/auth/format";
import { useAuth } from "../../../shared/auth/use-auth";
import { OnAMirror } from "../../../shared/shell/OnAMirror";
import { Button, Card, Confirm, EmptyState, InboxIcon, StatusChip, useToast } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { useOnMirror } from "../../../shared/use-mirror";
import { CheckField } from "../FormFields";
import { RowMenu } from "../RowMenu";
import { usePermissions } from "../shell/BackOfficeShell";
import { PinModal, RoleModal, StaffModal } from "./PeopleForms";
import {
  chosenRole,
  deactivateWords,
  lockedNote,
  peopleSummary,
  problemOf,
  sinceText,
  staffActions,
  staffRows,
  staffStateOf,
  type StaffAction,
} from "./people-logic";
import { RolePanel } from "./RolePanel";
import { usePeople, type PeopleApi } from "./use-people";

type Open =
  | { kind: "add-staff" }
  | { kind: "edit-staff"; person: Person }
  | { kind: "pin"; person: Person }
  | { kind: "deactivate"; person: Person }
  | { kind: "unlock"; person: Person }
  | { kind: "add-role" };

export function PeopleScreen() {
  const api = usePeople();
  const toast = useToast();
  const held = usePermissions();
  // A mirror shows who is on the staff and what each role may do, and changes neither (ADR-330).
  const mirror = useOnMirror();
  const [open, setOpen] = useState<Open | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [wanted, setWanted] = useState<number | null>(null);
  const [showOff, setShowOff] = useState(false);
  const data = api.list.data;
  const role = data ? chosenRole(data.roles, wanted) : null;
  const off = data ? data.staff.filter((p) => !p.active).length : 0;
  const now = new Date(useClock(30_000));

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Staff &amp; roles</h1>
          <p className="mt-1 text-base text-fg-muted">{data ? peopleSummary(data.staff) : "Reading the staff"}</p>
        </div>
        {mirror ? (
          <OnAMirror />
        ) : (
          <div className="flex flex-wrap gap-2.5">
            <Button disabled={!data} onClick={() => setOpen({ kind: "add-role" })}>
              Add a role
            </Button>
            <Button variant="attention" disabled={!data} onClick={() => setOpen({ kind: "add-staff" })}>
              <UserPlusIcon aria-hidden="true" size={18} />
              Add staff
            </Button>
          </div>
        )}
      </div>

      {problem && (
        <p role="alert" className="text-base text-danger">
          {problem}
        </p>
      )}
      {api.list.isPending && <p className="text-base text-fg-muted">Loading the people</p>}
      {api.list.error && (
        <EmptyState icon={<InboxIcon />} line={problemOf(api.list.error).message} action={{ label: "Try again", onClick: () => void api.list.refetch() }} />
      )}
      {data && (
        <div className="grid grid-cols-1 items-start gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
          <div className="flex min-w-0 flex-col gap-3">
            <StaffTable
              staff={staffRows(data.staff, showOff)}
              roleId={role?.id ?? null}
              now={now}
              api={api}
              readOnly={mirror}
              onRole={setWanted}
              onOpen={setOpen}
              onProblem={setProblem}
            />
            {off > 0 && (
              <div className="max-w-md">
                <CheckField label={`Show the ${off} deactivated`} checked={showOff} onChange={setShowOff} />
              </div>
            )}
          </div>
          {role && (
            <RolePanel
              roles={data.roles}
              role={role}
              staff={data.staff}
              held={held}
              readOnly={mirror}
              onRole={setWanted}
              onSave={async (id, input) => {
                await api.updateRole(id, input);
                toast("Saved. It applies to everyone with this role from their next step.", { tone: "success" });
              }}
              onDelete={async (r) => {
                await api.deleteRole(r.id);
                setWanted(null);
                toast(`Deleted the ${r.name} role.`, { tone: "success" });
              }}
            />
          )}
        </div>
      )}
      {data && open && <Opened open={open} roles={data.roles} api={api} onRole={setWanted} onClose={() => setOpen(null)} />}
    </div>
  );
}

interface StaffTableProps {
  staff: readonly Person[];
  /** The role shown beside the table: its people are marked. */
  roleId: number | null;
  /** For how long ago a lock fell. */
  now: Date;
  api: PeopleApi;
  /** Nobody is changed: a row has no menu. */
  readOnly: boolean;
  onRole: (id: number) => void;
  onOpen: (o: Open) => void;
  onProblem: (message: string | null) => void;
}

const head = "pb-3 text-meta font-bold uppercase text-fg-muted";
// A Record over the actions, so one added in the logic fails the build until it has a label.
const actionLabels: Record<StaffAction, string> = { unlock: "Unlock", edit: "Edit", pin: "New PIN", deactivate: "Deactivate", "bring-back": "Bring back" };

function StaffTable({ staff, roleId, now, api, readOnly, onRole, onOpen, onProblem }: StaffTableProps) {
  const status = useAuth();
  const me = status.kind === "signed_in" ? status.staff.id : "";
  const bringBack = (p: Person) => {
    onProblem(null);
    api.setActive(p.id, true).catch((err: unknown) => onProblem(problemOf(err).message));
  };
  const run = (action: StaffAction, p: Person) => {
    switch (action) {
      case "unlock":
        return onOpen({ kind: "unlock", person: p });
      case "edit":
        return onOpen({ kind: "edit-staff", person: p });
      case "pin":
        return onOpen({ kind: "pin", person: p });
      case "deactivate":
        return onOpen({ kind: "deactivate", person: p });
      case "bring-back":
        return bringBack(p);
    }
  };
  if (staff.length === 0) {
    return (
      <Card>
        <EmptyState icon={<InboxIcon />} line="Nobody is on the staff list." />
      </Card>
    );
  }
  return (
    <Card padding="lg" className="min-w-0">
      {/* Under 1280px it scrolls sideways; kept tall enough there for a row's menu to open inside. */}
      <div className="min-h-80 relative overflow-x-auto xl:min-h-0 xl:overflow-visible">
        <table aria-label="Staff" className="w-full min-w-136 text-base">
          <thead>
            <tr className="border-b border-line text-left">
              <th scope="col" className={head}>
                Person
              </th>
              <th scope="col" className={head}>
                Role
              </th>
              <th scope="col" className={head}>
                State
              </th>
              <th scope="col" className={head}>
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {staff.map((p, row) => {
              const shown = p.role_id === roleId;
              const lock = lockedNote(p, now);
              const actions = staffActions(p, me, readOnly);
              return (
                <tr key={p.id} className={cx("border-b border-line-soft last:border-b-0", shown && "bg-accent-tint")}>
                  <td className="py-1">
                    {/* The person is the button: picking them shows their role beside the table. */}
                    <button
                      type="button"
                      onClick={() => onRole(p.role_id)}
                      className="flex min-h-tap w-full items-center gap-3 rounded px-2 text-left outline-offset-2 outline-line-focus focus-visible:outline-2"
                    >
                      <span
                        aria-hidden="true"
                        className="flex size-10 shrink-0 items-center justify-center rounded-round bg-raised-high text-sm font-bold text-fg-2"
                      >
                        {initials(p.name) || "?"}
                      </span>
                      <span className="min-w-0">
                        <strong className={cx("block truncate", !p.active && "font-semibold text-fg-muted")}>
                          {p.name}
                          {p.id === me && <span className="font-normal text-fg-muted"> (you)</span>}
                        </strong>
                        <span className="block font-mono text-sm tabular-nums text-fg-muted">{sinceText(p)}</span>
                        <span className="sr-only">{shown ? ". Their role is shown beside the table." : ". Show their role."}</span>
                      </span>
                    </button>
                  </td>
                  <td className="py-1 text-fg-2">{roleText(p.role_name)}</td>
                  <td className="py-1">
                    <StatusChip status={{ kind: "staff", status: staffStateOf(p) }} />
                    {lock && <span className="mt-1 block text-sm text-fg-muted">{lock}</span>}
                  </td>
                  <td className="py-1">
                    {actions.length > 0 && (
                      <span className="flex justify-end">
                        <RowMenu
                          name={p.name}
                          disabled={false}
                          up={row >= 3 && row >= staff.length - 2}
                          actions={actions.map((action) => ({ label: actionLabels[action], run: () => run(action, p) }))}
                        />
                      </span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

interface OpenedProps {
  open: Open;
  roles: readonly Role[];
  api: PeopleApi;
  /** Shows a role just made. */
  onRole: (id: number) => void;
  onClose: () => void;
}

function Opened({ open, roles, api, onRole, onClose }: OpenedProps) {
  const held = usePermissions();
  const toast = useToast();
  switch (open.kind) {
    case "add-staff":
      return (
        <StaffModal
          person={null}
          roles={roles}
          held={held}
          onSave={async (input) => {
            await api.addStaff(input);
            toast(`Added ${input.name}.`, { tone: "success" });
          }}
          onClose={onClose}
        />
      );
    case "edit-staff":
      return (
        <StaffModal
          person={open.person}
          roles={roles}
          held={held}
          onSave={async (input) => {
            await api.updateStaff(open.person.id, input);
            toast("Saved.", { tone: "success" });
          }}
          onClose={onClose}
        />
      );
    case "pin":
      return (
        <PinModal
          person={open.person}
          roles={roles}
          onSave={async (pin) => {
            await api.resetPIN(open.person.id, pin);
            toast(`${open.person.name} has a new PIN and is signed out.`, { tone: "success" });
          }}
          onClose={onClose}
        />
      );
    case "deactivate":
      return <DeactivateConfirm person={open.person} api={api} onClose={onClose} />;
    case "unlock":
      return <UnlockConfirm person={open.person} api={api} onClose={onClose} />;
    case "add-role":
      return (
        <RoleModal
          role={null}
          held={held}
          onSave={async (input) => {
            onRole(await api.addRole(input));
            toast(`Added the ${input.name} role.`, { tone: "success" });
          }}
          onClose={onClose}
        />
      );
  }
}

function DeactivateConfirm({ person, api, onClose }: { person: Person; api: PeopleApi; onClose: () => void }) {
  const toast = useToast();
  const [problem, setProblem] = useState<string | null>(null);
  return (
    <Confirm
      open
      title={`Deactivate ${person.name}?`}
      confirmLabel="Deactivate"
      onClose={onClose}
      onConfirm={() => {
        api
          .setActive(person.id, false)
          .then(() => {
            toast(`${person.name} is deactivated.`, { tone: "success" });
            onClose();
          })
          .catch((err: unknown) => setProblem(problemOf(err).message));
      }}
    >
      <p className="text-base">{deactivateWords(person)}</p>
      {problem && (
        <p role="alert" className="mt-3 text-base text-danger">
          {problem}
        </p>
      )}
    </Confirm>
  );
}

function UnlockConfirm({ person, api, onClose }: { person: Person; api: PeopleApi; onClose: () => void }) {
  const toast = useToast();
  const [problem, setProblem] = useState<string | null>(null);
  return (
    <Confirm
      open
      title={`Unlock ${person.name}?`}
      confirmLabel="Unlock"
      onClose={onClose}
      onConfirm={() => {
        api
          .unlock(person.id)
          .then(() => {
            toast(`${person.name} is unlocked.`, { tone: "success" });
            onClose();
          })
          .catch((err: unknown) => setProblem(problemOf(err).message));
      }}
    >
      <p className="text-base">
        Their PIN was typed wrong too many times, so they were signed out and cannot sign in. Unlocked, they are back on the sign-in screen with the PIN they
        have. If it was not them typing, give them a new PIN as well.
      </p>
      {problem && (
        <p role="alert" className="mt-3 text-base text-danger">
          {problem}
        </p>
      )}
    </Confirm>
  );
}
