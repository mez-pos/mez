// The paired devices as a table: each with its kind, when it was paired and
// last heard from, and a chip for where it stands. Rename and revoke are
// under the row's "...". A revoked device stays on the list, with nothing
// left to do to it.

import { ChefHatIcon, CloudIcon, LaptopIcon, MonitorIcon, SmartphoneIcon, type LucideIcon } from "lucide-react";
import type { DeviceKind, DeviceRecordResponse } from "../../../shared/api-types";
import { Card, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useOnMirror } from "../../../shared/use-mirror";
import { useAway } from "../../../shared/use-realtime";
import { RowMenu } from "../RowMenu";
import { dateText, deviceActions, kindText, lastSeenText, revokedText, stateOf, type DeviceAction } from "./devices-logic";

interface DeviceTableProps {
  /** In the order shown: in service, then revoked. */
  devices: readonly DeviceRecordResponse[];
  now: Date;
  onRename: (device: DeviceRecordResponse) => void;
  onRevoke: (device: DeviceRecordResponse) => void;
}

const head = "pb-3 text-meta font-bold uppercase text-fg-muted";
// A Record over the kinds, so a kind added in Go fails the build until it has an icon.
const icons: Record<DeviceKind, LucideIcon> = { terminal: MonitorIcon, kitchen: ChefHatIcon, back_office: LaptopIcon, guest: SmartphoneIcon };
// A Record over the actions, so one added in the logic fails the build until it has a label.
const actionLabels: Record<DeviceAction, string> = { rename: "Rename", revoke: "Revoke" };

export function DeviceTable({ devices, now, onRename, onRevoke }: DeviceTableProps) {
  // From away a device can be unpaired, a lost phone say, and not renamed (ADR-308).
  const away = useAway();
  const mirror = useOnMirror();
  return (
    <Card padding="lg" className="min-w-0">
      {/* Under 1280px it scrolls sideways; kept tall enough there for a row's menu to open inside. */}
      {/* Positioned, so the labels only a screen reader hears, which are absolute, are clipped
          with the table and do not make the whole page scroll sideways on a phone. */}
      <div className="min-h-80 relative overflow-x-auto xl:min-h-0 xl:overflow-visible">
        <table aria-label="Devices" className="w-full min-w-176 text-base">
          <thead>
            <tr className="border-b border-line text-left">
              <th scope="col" className={head}>
                Device
              </th>
              <th scope="col" className={head}>
                Paired
              </th>
              <th scope="col" className={head}>
                Last heard from
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
            {devices.map((d, row) => {
              const Icon = d.cloud ? CloudIcon : icons[d.kind];
              const revoked = d.revoked_at !== undefined;
              const actions = deviceActions(d, away, mirror);
              return (
                <tr key={d.id} className="border-b border-line-soft last:border-b-0">
                  <td className="py-2 pr-4">
                    <span className="flex items-center gap-2.5">
                      <span aria-hidden="true" className="flex size-9 shrink-0 items-center justify-center rounded-tight bg-raised-high text-fg-muted">
                        <Icon size={17} />
                      </span>
                      <span className="min-w-0">
                        <span className="flex flex-wrap items-baseline gap-x-3">
                          <strong className={cx(revoked && "font-semibold text-fg-muted")}>{d.name}</strong>
                          {d.this && <span className="text-sm font-semibold text-accent-text">This device</span>}
                        </span>
                        <span className="block text-sm text-fg-muted">
                          {kindText(d)}
                          {revoked && `. ${revokedText(d)}`}
                          {d.this && ". You are using it: revoke it from another back office."}
                        </span>
                      </span>
                    </span>
                  </td>
                  <td className="whitespace-nowrap py-2 pr-4 text-fg-2">{dateText(d.paired_at)}</td>
                  <td className="py-2 pr-4 text-fg-2">{lastSeenText(d, now)}</td>
                  <td className="py-2 pr-4">
                    <StatusChip status={{ kind: "device", status: stateOf(d, now) }} />
                  </td>
                  {/* The height of a row's menu, kept where there is none, so the rows are one height. */}
                  <td className="h-16 py-2">
                    {actions.length > 0 && (
                      <span className="flex justify-end">
                        <RowMenu
                          name={d.name}
                          disabled={false}
                          up={row >= 3 && row >= devices.length - 2}
                          actions={actions.map((action) => ({ label: actionLabels[action], run: () => (action === "rename" ? onRename(d) : onRevoke(d)) }))}
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
