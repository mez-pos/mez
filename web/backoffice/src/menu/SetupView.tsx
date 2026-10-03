// The Stations and courses view of the menu editor (backoffice-menu.md): where
// an item is made and when it is fired. Each list saves a change at once, as
// the categories do; a retire says where what the row carries goes.

import { useState } from "react";
import type { MenuEditor } from "../../../shared/api-types";
import { useToast } from "../../../shared/ui";
import { moved } from "./menu-logic";
import { SetupCard } from "./SetupCard";
import { NameModal, RetireModal } from "./SetupModals";
import { courseRetiredText, courseRows, setupProblemOf, stationRetiredText, stationRows, type SetupKind, type SetupRow } from "./setup-logic";
import type { MenuEditorApi } from "./use-menu-editor";

type Editing = { what: SetupKind; action: "add" } | { what: SetupKind; action: "rename"; row: SetupRow } | { what: SetupKind; action: "retire"; row: SetupRow };

interface SetupViewProps {
  menu: MenuEditor;
  api: MenuEditorApi;
  onProblem: (message: string) => void;
}

export function SetupView({ menu, api, onProblem }: SetupViewProps) {
  const toast = useToast();
  const [editing, setEditing] = useState<Editing | null>(null);
  const [busy, setBusy] = useState(false);
  const stations = stationRows(menu);
  const courses = courseRows(menu);
  const rowsOf = (what: SetupKind) => (what === "station" ? stations : courses);

  // A change that is not in a form: ordering, bringing back.
  const quietly = (run: () => Promise<void>) => {
    setBusy(true);
    onProblem("");
    run()
      .catch((err: unknown) => onProblem(setupProblemOf(err).message))
      .finally(() => setBusy(false));
  };
  const move = (what: SetupKind, row: SetupRow, delta: -1 | 1) => {
    const rows = rowsOf(what);
    const ids = moved(
      rows.map((r) => r.id),
      rows.findIndex((r) => r.id === row.id),
      delta,
    );
    if (ids) quietly(() => (what === "station" ? api.orderStations(ids) : api.orderCourses(ids)));
  };
  const restore = (what: SetupKind, row: SetupRow) =>
    quietly(async () => {
      await (what === "station" ? api.restoreStation(row.id) : api.restoreCourse(row.id));
      toast(`${row.name} is back in service.`, { tone: "success" });
    });

  return (
    <div className="flex max-w-4xl flex-col gap-6">
      <SetupCard
        title="Stations"
        description={<>Where an item is made: each station has its own kitchen display, and prints on the printer you set for it under Printers.</>}
        addLabel="Add a station"
        empty="No stations yet. Add the first, then the items made there."
        rows={stations}
        busy={busy}
        onAdd={() => setEditing({ what: "station", action: "add" })}
        onRename={(row) => setEditing({ what: "station", action: "rename", row })}
        onMove={(row, delta) => move("station", row, delta)}
        onRetire={(row) => setEditing({ what: "station", action: "retire", row })}
        onRestore={(row) => restore("station", row)}
      />
      <SetupCard
        title="Courses"
        description="When an item is fired: course 1 first, then the table asks for the next. Putting courses in a new order is refused while any ticket is open, since tickets keep the number they were rung up at."
        addLabel="Add a course"
        empty="No courses yet. Add the first: every item sits in one."
        rows={courses}
        busy={busy}
        onAdd={() => setEditing({ what: "course", action: "add" })}
        onRename={(row) => setEditing({ what: "course", action: "rename", row })}
        onMove={(row, delta) => move("course", row, delta)}
        onRetire={(row) => setEditing({ what: "course", action: "retire", row })}
        onRestore={(row) => restore("course", row)}
      />

      {editing && (
        <EditingModal
          editing={editing}
          rows={rowsOf(editing.what)}
          api={api}
          onDone={(message) => toast(message, { tone: "success" })}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}

interface EditingModalProps {
  editing: Editing;
  rows: readonly SetupRow[];
  api: MenuEditorApi;
  onDone: (message: string) => void;
  onClose: () => void;
}

function EditingModal({ editing, rows, api, onDone, onClose }: EditingModalProps) {
  const { what } = editing;
  if (editing.action === "add") {
    return (
      <NameModal
        title={what === "station" ? "Add a station" : "Add a course"}
        initial=""
        code={what === "station" ? { mode: "choose" } : null}
        onSave={async (name, code) => {
          await (what === "station" ? api.createStation({ code, name }) : api.createCourse(name));
          onDone("Saved.");
        }}
        onClose={onClose}
      />
    );
  }
  const row = editing.row;
  if (editing.action === "rename") {
    return (
      <NameModal
        title={`Rename ${row.name}`}
        initial={row.name}
        code={what === "station" ? { mode: "fixed", value: row.tag } : null}
        onSave={async (name) => {
          await (what === "station" ? api.renameStation(row.id, name) : api.renameCourse(row.id, name));
          onDone("Saved.");
        }}
        onClose={onClose}
      />
    );
  }
  return (
    <RetireModal
      kind={what}
      row={row}
      rows={rows}
      onRetire={async (moveTo) => {
        const to = rows.find((r) => r.id === moveTo) ?? null;
        onDone(
          what === "station"
            ? stationRetiredText(row.name, to, await api.retireStation(row.id, moveTo))
            : courseRetiredText(row.name, to, await api.retireCourse(row.id, moveTo)),
        );
      }}
      onClose={onClose}
    />
  );
}
