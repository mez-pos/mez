// The venue's paired devices (docs/design/screens/backoffice-devices.md): who is
// in service and when each was last heard from, and pair, rename and revoke.

import { useEffect, useState } from "react";
import type { DeviceRecordResponse } from "../../../shared/api-types";
import { Button, Card, EmptyState, InboxIcon, useToast } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { PairModal, RenameModal, RevokeModal } from "./DeviceModals";
import { actionProblemOf, isStale, kindWords, lastSeenText, pairedText, revokedText, runsOutText, split } from "./devices-logic";
import { useDevices } from "./use-devices";

type Editing = { kind: "pair" } | { kind: "rename"; device: DeviceRecordResponse } | { kind: "revoke"; device: DeviceRecordResponse };

export function DevicesScreen() {
  const api = useDevices();
  const toast = useToast();
  const now = useNow(api.data);
  const [editing, setEditing] = useState<Editing | null>(null);
  const list = api.data ? split(api.data.devices) : null;

  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-6">
      <div className="flex flex-wrap items-baseline justify-between gap-4">
        <h1 className="font-serif text-total">Devices</h1>
        <Button variant="primary" disabled={!api.data} onClick={() => setEditing({ kind: "pair" })}>
          Pair a device
        </Button>
      </div>

      {api.loading && <p className="text-base text-fg-muted">Loading the devices</p>}
      {!api.loading && api.error !== null && api.data === undefined && (
        <EmptyState icon={<InboxIcon />} line={actionProblemOf(api.error)} action={{ label: "Try again", onClick: api.refetch }} />
      )}

      {api.data && list && (
        <>
          <Card padding="lg" className="flex flex-col gap-2">
            <h2 className="font-serif text-xl">In service</h2>
            <ul className="flex flex-col divide-y divide-line-soft">
              {list.inService.map((d) => (
                <InService key={d.id} device={d} now={now} onRename={() => setEditing({ kind: "rename", device: d })} onRevoke={() => setEditing({ kind: "revoke", device: d })} />
              ))}
            </ul>
            {list.inService.length <= 1 && (
              <p className="pt-2 text-base text-fg-muted">Pair a till, a kitchen display or another back office to start.</p>
            )}
          </Card>

          {api.data.waiting.length > 0 && (
            <Card padding="lg" className="flex flex-col gap-2">
              <h2 className="font-serif text-xl">Waiting to be paired</h2>
              <ul className="flex flex-col divide-y divide-line-soft">
                {api.data.waiting.map((w, i) => (
                  <li key={`${w.name}-${i}`} className="flex flex-wrap items-baseline justify-between gap-x-4 py-3">
                    <p className="text-base">
                      <span className="font-semibold">{w.name}</span> <span className="text-fg-2">({kindWords[w.kind]})</span>
                    </p>
                    <p className="text-sm text-fg-muted">{runsOutText(w.expires_at, now)}</p>
                  </li>
                ))}
              </ul>
            </Card>
          )}

          {list.revoked.length > 0 && (
            <Card padding="lg" className="flex flex-col gap-2">
              <h2 className="font-serif text-xl text-fg-2">Revoked</h2>
              <ul className="flex flex-col divide-y divide-line-soft">
                {list.revoked.map((d) => (
                  <li key={d.id} className="flex flex-wrap items-baseline justify-between gap-x-4 py-3">
                    <p className="text-base">
                      <span className="text-fg-muted line-through">{d.name}</span> <span className="text-fg-muted">({kindWords[d.kind]})</span>
                    </p>
                    <p className="text-sm text-fg-muted">{revokedText(d)}</p>
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </>
      )}

      {editing?.kind === "pair" && (
        <PairModal onPair={(name, kind) => api.pair({ name, kind })} onClose={() => setEditing(null)} />
      )}
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

interface InServiceProps {
  device: DeviceRecordResponse;
  now: Date;
  onRename: () => void;
  onRevoke: () => void;
}

function InService({ device: d, now, onRename, onRevoke }: InServiceProps) {
  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-3 py-4">
      <div className="min-w-0 flex-1">
        <p className="flex flex-wrap items-baseline gap-x-3 text-base">
          <span className="font-semibold">{d.name}</span>
          {d.this && <span className="text-sm font-semibold text-accent-text">This device</span>}
        </p>
        <p className="flex flex-wrap gap-x-4 text-sm text-fg-2">
          <span>{kindWords[d.kind]}</span>
          <span>{pairedText(d)}</span>
          <span className={cx(isStale(d, now) && "font-semibold text-chip-warning-fg")}>{lastSeenText(d, now)}</span>
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Button size="md" onClick={onRename}>
          Rename
        </Button>
        <Button size="md" variant="danger" disabled={d.this} onClick={onRevoke}>
          Revoke
        </Button>
      </div>
      {d.this && <p className="basis-full text-sm text-fg-muted">You are using it. Revoke it from another back office.</p>}
    </li>
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
