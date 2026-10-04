// The tables screen (D05): how to look at the tables down the left, the floor
// in the middle, the table picked on the right. A tap picks a table; the panel
// seats a party at it, shows its order, sends it, settles it. Moving a party
// and joining tables start from the panel's More Actions and turn the next taps
// into steps of that, until Done (ADR-088). ADR-155 records what this draws.

import { useState } from "react";
import { useLocation } from "wouter";
import type { LiveTableResponse } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { failureOf, wordsOf } from "../register/order-messages";
import { route } from "../shell/routes";
import { useTerminal } from "../shell/terminal-context";
import { FloorLegend, FloorPlan } from "./FloorPlan";
import { areaToShow, areasShown, beginMode, countsText, idle, modeText, panelTitle, partyTable, stepMode, type AreaChoice, type Mode } from "./floor-logic";
import { TableList } from "./TableList";
import type { PartyMode } from "./TableOrderView";
import { TablePanel } from "./TablePanel";
import { TablesSidebar, type TablesView } from "./TablesSidebar";

export function TablesScreen() {
  const { floor, ticket, names } = useTerminal();
  const [, navigate] = useLocation();
  const [view, setView] = useState<TablesView>("plan");
  const [areaChoice, setAreaChoice] = useState<AreaChoice>(null);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [mode, setMode] = useState<Mode>(idle);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const now = useClock(30_000);

  const data = floor.data;
  const tables = data?.tables ?? [];
  const areas = data ? areasShown(data, areaChoice) : [];
  const found = tables.find((t) => t.id === selectedId);
  // The party's own table, even if the one picked was joined to it since.
  const selected = found ? partyTable(found, tables) : null;
  const banner = modeText(mode);
  const party = mode.kind === "move" ? mode.from : mode.kind === "join" ? mode.to : null;
  // While moving or joining, the tile outlined is the party's, not the panel's.
  const picked = mode.kind === "idle" ? selectedId : (party?.id ?? null);

  /** A tap while Move, Join or Part is on: the mode decides, the server does. */
  const tapInMode = (t: LiveTableResponse) => {
    const step = stepMode(mode, t, names);
    setMode(step.mode);
    setError(step.refused ?? null);
    const action = step.action;
    if (!action) return;
    setBusy(true);
    floor
      .act(action)
      .then(() => {
        // The panel follows the party to its new table.
        if (action.kind === "move") setSelectedId(action.to);
      })
      .catch((err: unknown) => setError(wordsOf(err)))
      .finally(() => setBusy(false));
  };

  const tap = (t: LiveTableResponse) => {
    // A move on its way to the server decides where the panel goes next.
    if (busy) return;
    if (mode.kind !== "idle") {
      tapInMode(t);
      return;
    }
    setError(null);
    setSelectedId(partyTable(t, tables).id);
  };

  /** Move and Join start from the table in the panel: its first tap is made. */
  const startMode = (kind: PartyMode) => {
    if (kind === "part") {
      setMode({ kind: "part" });
      setError(null);
      return;
    }
    if (!selected) return;
    const step = beginMode(kind, selected, names);
    setMode(step.mode);
    setError(step.refused ?? null);
  };

  const body = () => {
    if (floor.loading) return <p className="py-12 text-center text-base text-fg-muted">Loading the floor</p>;
    if (!data) return <EmptyState icon={<InboxIcon />} line={failureOf(floor.error)} action={{ label: "Try again", onClick: floor.refetch }} />;
    if (tables.length === 0) return <EmptyState icon={<InboxIcon />} line="No tables yet. Lay out the floor in the back office." />;
    const shared = { data, areas, now, names, picked, onTap: tap };
    return view === "plan" ? <FloorPlan {...shared} /> : <TableList {...shared} />;
  };

  return (
    <div className="flex min-w-0 flex-1">
      <TablesSidebar view={view} onView={setView} />

      <section aria-label="Floor" className="flex min-w-0 flex-1 flex-col gap-4 px-6 py-5">
        {data && tables.length > 0 && (
          <div className="flex flex-wrap items-center gap-2.5">
            {data.areas.length > 1 && (
              <AreaPills areas={data.areas} chosen={areaChoice === "all" ? "all" : areaToShow(data, areaChoice)} onChoose={setAreaChoice} />
            )}
            <p className="ml-auto text-sm text-fg-muted">
              {tables.length} {tables.length === 1 ? "table" : "tables"} · {countsText(tables)}
            </p>
          </div>
        )}

        {banner && (
          <div role="status" className="flex flex-wrap items-center gap-tap-gap rounded border border-accent bg-accent-tint px-4 py-2 text-accent-text">
            <p className="flex-1 text-base font-semibold">{banner}</p>
            <Button
              variant="primary"
              onClick={() => {
                setMode(idle);
                setError(null);
              }}
            >
              Done
            </Button>
          </div>
        )}
        {error && (
          <p role="alert" className="text-base text-danger">
            {error}
          </p>
        )}

        {body()}

        {view === "plan" && data && tables.length > 0 && <FloorLegend />}
      </section>

      <TablePanel
        table={selected}
        title={selected ? panelTitle(selected, tables) : ""}
        now={now}
        lateAfter={ticket.open?.late_after_minutes ?? 20}
        tickets={ticket.open?.orders ?? []}
        onSeat={async (t, covers) => {
          // The party is seated and the floor stays: Add Items takes the order.
          // Waiting for the floor keeps Seat shut until the table reads seated:
          // a second tap would open a second order at it.
          await ticket.seat({ channel: "counter", table_id: t.id, covers });
          await floor.refresh();
        }}
        onClear={(t) => floor.clear(t.id)}
        onRelease={(t) => floor.release(t.id)}
        onAddItems={async (orderId, held) => {
          if (held) await ticket.resume(orderId);
          else ticket.switchTo(orderId);
          navigate(route.order);
        }}
        onSettle={(orderId) => navigate(route.pay(orderId))}
        onMode={startMode}
        sendWaits={(orderId) => ticket.orderId === orderId && ticket.pending.length > 0}
      />
    </div>
  );
}

interface AreaPillsProps {
  areas: readonly { id: number; name: string }[];
  chosen: number | "all" | null;
  onChoose: (choice: AreaChoice) => void;
}

function AreaPills({ areas, chosen, onChoose }: AreaPillsProps) {
  const pills: readonly { id: number | "all"; name: string }[] = [...areas, { id: "all", name: "All Areas" }];
  return (
    <div role="group" aria-label="Areas" className="flex flex-wrap gap-2.5">
      {pills.map((a) => (
        <button
          key={a.id}
          type="button"
          aria-pressed={a.id === chosen}
          onClick={() => onChoose(a.id)}
          className={cx(
            "min-h-tap rounded-pill border px-5 text-base outline-offset-2 outline-attention focus-visible:outline-3",
            a.id === chosen
              ? "border-attention bg-attention font-semibold text-attention-fg"
              : "border-line-strong bg-raised font-medium text-fg hover:bg-raised-high",
          )}
        >
          {a.name}
        </button>
      ))}
    </div>
  );
}
