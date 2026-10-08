// The paired devices as a table: each with its kind, when it was paired and
// last heard from, and a chip for where it stands. Rename and revoke are
// under the row's "...". A revoked device stays on the list, with nothing
// left to do to it.

import { ChefHatIcon, LaptopIcon, MonitorIcon, SmartphoneIcon, type LucideIcon } from "lucide-react";
import type { DeviceKind, DeviceRecordResponse } from "../../../shared/api-types";
import { Card, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useAway } from "../../../shared/use-realtime";
import { RowMenu } from "../RowMenu";
import { dateText, kindWords, lastSeenText, revokedText, stateOf } from "./devices-logic";

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

export function DeviceTable({ devices, now, onRename, onRevoke }: DeviceTableProps) {
  // From away a device can be unpaired, a lost phone say, and not renamed (ADR-308).
  const away = useAway();
  return (
    <Card padding="lg" className="min-w-0">
      {/* Under 1280px it scrolls sideways; kept tall enough there for a row's menu to open inside. */}
      <div className="min-h-80 overflow-x-auto xl:min-h-0 xl:overflow-visible">
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
              const Icon = icons[d.kind];
              const revoked = d.revoked_at !== undefined;
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
                          {kindWords[d.kind]}
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
                    {!revoked && !(away && d.this) && (
                      <span className="flex justify-end">
                        <RowMenu
                          name={d.name}
                          disabled={false}
                          up={row >= 3 && row >= devices.length - 2}
                          actions={[
                            ...(away ? [] : [{ label: "Rename", run: () => onRename(d) }]),
                            ...(d.this ? [] : [{ label: "Revoke", run: () => onRevoke(d) }]),
                          ]}
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
