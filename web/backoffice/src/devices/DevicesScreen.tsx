// The venue's paired devices (docs/design/screens/backoffice-devices.md): who is
// in service and when each was last heard from, and pair, rename and revoke.

import { PlusIcon } from "lucide-react";
import { useEffect, useState } from "react";
import type { DeviceRecordResponse } from "../../../shared/api-types";
import { Button, EmptyState, InboxIcon, useToast } from "../../../shared/ui";
import { Panel } from "../dashboard/Panel";
import { PairModal, RenameModal, RevokeModal } from "./DeviceModals";
import { DeviceTable } from "./DeviceTable";
import { actionProblemOf, devicesSummary, kindWords, listed, runsOutText, split } from "./devices-logic";
import { useDevices } from "./use-devices";

type Editing = { kind: "pair" } | { kind: "rename"; device: DeviceRecordResponse } | { kind: "revoke"; device: DeviceRecordResponse };

export function DevicesScreen() {
  const api = useDevices();
  const toast = useToast();
  const now = useNow(api.data);
  const [editing, setEditing] = useState<Editing | null>(null);
  const data = api.data;
  const waiting = data?.waiting ?? [];

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-serif text-total">Devices</h1>
          {data && <p className="mt-1 text-base text-fg-muted">{devicesSummary(data.devices, now)}</p>}
        </div>
        <Button variant="attention" disabled={!data} onClick={() => setEditing({ kind: "pair" })}>
          <PlusIcon aria-hidden="true" size={18} />
          Pair a device
        </Button>
      </div>

      {api.loading && <p className="text-base text-fg-muted">Loading the devices</p>}
      {!api.loading && api.error !== null && data === undefined && (
        <EmptyState icon={<InboxIcon />} line={actionProblemOf(api.error)} action={{ label: "Try again", onClick: api.refetch }} />
      )}

      {data && (
        <div className={waiting.length > 0 ? "grid grid-cols-1 items-start gap-4 xl:grid-cols-[minmax(0,1fr)_380px]" : "flex flex-col"}>
          <div className="flex min-w-0 flex-col gap-3">
            <DeviceTable
              devices={listed(data.devices)}
              now={now}
              onRename={(device) => setEditing({ kind: "rename", device })}
              onRevoke={(device) => setEditing({ kind: "revoke", device })}
            />
            {split(data.devices).inService.length <= 1 && (
              <p className="text-base text-fg-muted">Pair a terminal, a kitchen display or another back office to start.</p>
            )}
          </div>
          {waiting.length > 0 && (
            <Panel title="Waiting to be paired">
              <ul className="flex flex-col divide-y divide-line-soft">
                {waiting.map((w, i) => (
                  <li key={`${w.name}-${i}`} className="flex flex-wrap items-baseline justify-between gap-x-4 py-3">
                    <p className="text-base">
                      <span className="font-semibold">{w.name}</span> <span className="text-fg-2">({kindWords[w.kind]})</span>
                    </p>
                    <p className="text-sm text-fg-muted">{runsOutText(w.expires_at, now)}</p>
                  </li>
                ))}
              </ul>
            </Panel>
          )}
        </div>
      )}

      {editing?.kind === "pair" && <PairModal onPair={(name, kind) => api.pair({ name, kind })} onClose={() => setEditing(null)} />}
      {editing?.kind === "rename" && (
        <RenameModal
          device={editing.device}
          onRename={async (name) => {
            await api.rename(editing.device.id, name);
            toast("Saved.", { tone: "success" });
          }}
          onClose={() => setEditing(null)}
        />
      )}
      {editing?.kind === "revoke" && (
        <RevokeModal
          device={editing.device}
          onRevoke={async () => {
            await api.revoke(editing.device.id);
            toast(`${editing.device.name} revoked.`, { tone: "success" });
          }}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}

/**
 * The time, for "12 min ago" and "runs out in 7 min": moving on each 30 seconds,
 * and taken again whenever the list is, so a code made a moment ago does not
 * read a half minute older than it is.
 */
function useNow(data: unknown): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 30_000);
    return () => clearInterval(t);
  }, []);
  useEffect(() => setNow(new Date()), [data]);
  return now;
}
