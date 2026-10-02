// The People screen (docs/design/screens/backoffice-people.md): staff with
// their roles, and the roles with what each may do, in words. What you may
// give is what you hold (ADR-115): the server decides, the screen shows it.

import { useState } from "react";
import type { Person, Role } from "../../../shared/api-types";
import { useAuth } from "../../../shared/auth/use-auth";
import { Button, Card, Confirm, EmptyState, InboxIcon, useToast } from "../../../shared/ui";
import { DataTable, type Column } from "../../../shared/ui/DataTable";
import { cx } from "../../../shared/ui/cx";
import { usePermissions } from "../shell/BackOfficeShell";
import { PinModal, RoleModal, StaffModal } from "./PeopleForms";
import { problemOf, roleWords } from "./people-logic";
import { usePeople, type PeopleApi } from "./use-people";

type Open =
  | { kind: "add-staff" }
  | { kind: "edit-staff"; person: Person }
  | { kind: "pin"; person: Person }
  | { kind: "deactivate"; person: Person }
  | { kind: "add-role" }
  | { kind: "edit-role"; role: Role };

type View = "staff" | "roles";

export function PeopleScreen() {
  const api = usePeople();
  const [view, setView] = useState<View>("staff");
  const [open, setOpen] = useState<Open | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const data = api.list.data;

  return (
    <div className="flex max-w-6xl flex-col gap-6">
      <div className="flex flex-wrap items-baseline justify-between gap-4">
        <h1 className="font-serif text-total">People</h1>
        <Button variant="primary" disabled={!data} onClick={() => setOpen({ kind: view === "staff" ? "add-staff" : "add-role" })}>
          {view === "staff" ? "Add a person" : "Add a role"}
        </Button>
      </div>
      <div role="group" aria-label="Show" className="flex gap-tap-gap">
        {(["staff", "roles"] as const).map((v) => (
          <Button
            key={v}
            size="md"
            variant={view === v ? "primary" : "secondary"}
            aria-pressed={view === v}
            onClick={() => {
              setView(v);
              setProblem(null);
            }}
          >
            {v === "staff" ? "Staff" : "Roles"}
          </Button>
        ))}
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
      {data && view === "staff" && <StaffTable staff={data.staff} api={api} onOpen={setOpen} onProblem={setProblem} />}
      {data && view === "roles" && <RoleCards roles={data.roles} api={api} onOpen={setOpen} onProblem={setProblem} />}
      {data && open && <Opened open={open} roles={data.roles} api={api} onClose={() => setOpen(null)} />}
    </div>
  );
}

interface ListProps {
  api: PeopleApi;
  onOpen: (o: Open) => void;
  onProblem: (message: string | null) => void;
}

function StaffTable({ staff, api, onOpen, onProblem }: ListProps & { staff: readonly Person[] }) {
  const status = useAuth();
  const me = status.kind === "signed_in" ? status.staff.id : "";
  const bringBack = (p: Person) => {
    onProblem(null);
    api.setActive(p.id, true).catch((err: unknown) => onProblem(problemOf(err).message));
  };
  const columns: Column<Person>[] = [
    {
      id: "name",
      header: "Name",
      cell: (p) => (
        <span className="flex items-center gap-2">
          <span className={cx("font-semibold", !p.active && "text-fg-muted line-through")}>{p.name}</span>
          {p.id === me && <span className="text-fg-3">(you)</span>}
          {!p.active && <span className="text-meta uppercase text-fg-faint">Deactivated</span>}
        </span>
      ),
    },
    { id: "role", header: "Role", cell: (p) => p.role_name },
    { id: "since", header: "Since", align: "right", cell: (p) => <span className="font-mono tabular-nums">{p.since.slice(0, 10)}</span> },
    {
      id: "actions",
      header: "",
      align: "right",
      cell: (p) => (
        <span className="flex flex-wrap justify-end gap-tap-gap">
          <Button onClick={() => onOpen({ kind: "edit-staff", person: p })}>Edit</Button>
          {p.active && <Button onClick={() => onOpen({ kind: "pin", person: p })}>New PIN</Button>}
          {p.active && p.id !== me && <Button onClick={() => onOpen({ kind: "deactivate", person: p })}>Deactivate</Button>}
          {!p.active && <Button onClick={() => bringBack(p)}>Bring back</Button>}
        </span>
      ),
    },
  ];
  return <DataTable label="Staff" columns={columns} rows={staff} rowKey={(p) => p.id} />;
}

function RoleCards({ roles, api, onOpen, onProblem }: ListProps & { roles: readonly Role[] }) {
  const remove = (r: Role) => {
    onProblem(null);
    api.deleteRole(r.id).catch((err: unknown) => onProblem(problemOf(err).message));
  };
  return (
    <ul className="grid gap-4 md:grid-cols-2">
      {roles.map((r) => {
        const groups = roleWords(r.permissions);
        return (
          <li key={r.id}>
            <Card className="flex h-full flex-col gap-3">
              <div className="flex items-baseline justify-between gap-4">
                <h2 className="text-lg font-semibold">{r.name}</h2>
                <span className="text-sm text-fg-3">
                  <span className="font-mono tabular-nums">{r.people}</span> {r.people === 1 ? "person" : "people"}
                </span>
              </div>
              {groups.length === 0 && <p className="text-sm text-fg-muted">It grants nothing.</p>}
              <dl className="flex flex-col gap-1.5 text-sm">
                {groups.map((g) => (
                  <div key={g.title}>
                    <dt className="text-meta font-semibold uppercase text-fg-faint">{g.title}</dt>
                    <dd className="text-fg-2">{g.words.join(", ")}</dd>
                  </div>
                ))}
              </dl>
              <div className="mt-auto flex gap-tap-gap">
                <Button onClick={() => onOpen({ kind: "edit-role", role: r })}>Edit</Button>
                {r.people === 0 && <Button onClick={() => remove(r)}>Delete</Button>}
              </div>
            </Card>
          </li>
        );
      })}
    </ul>
  );
}

function Opened({ open, roles, api, onClose }: { open: Open; roles: readonly Role[]; api: PeopleApi; onClose: () => void }) {
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
          onSave={async (pin) => {
            await api.resetPIN(open.person.id, pin);
            toast(`${open.person.name} has a new PIN and is signed out.`, { tone: "success" });
          }}
          onClose={onClose}
        />
      );
    case "deactivate":
      return <DeactivateConfirm person={open.person} api={api} onClose={onClose} />;
    case "add-role":
      return (
        <RoleModal
          role={null}
          held={held}
          onSave={async (input) => {
            await api.addRole(input);
            toast(`Added the ${input.name} role.`, { tone: "success" });
          }}
          onClose={onClose}
        />
      );
    case "edit-role":
      return (
        <RoleModal
          role={open.role}
          held={held}
          onSave={async (input) => {
            await api.updateRole(open.role.id, input);
            toast("Saved. It applies to everyone with this role from their next step.", { tone: "success" });
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
      <p className="text-base">
        They leave the sign-in screen and are signed out now. Their name stays on everything they did. You can bring them back.
      </p>
      {problem && (
        <p role="alert" className="mt-3 text-base text-danger">
          {problem}
        </p>
      )}
    </Confirm>
  );
}
