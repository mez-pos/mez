// The bar along the bottom of every screen (D03): which mez, whether the
// server is there, which venue, and the time. 40px, as drawn. The design also
// shows Keyboard Shortcuts and Open Cash Drawer here; neither exists, and a
// control that does nothing is not drawn (rebuild.md, rule 3).

import { WifiIcon, WifiOffIcon } from "lucide-react";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { useRealtime } from "../../../shared/use-realtime";
import { clockText, versionText } from "../../../shared/venue-text";
import { connectionText } from "./status-logic";

interface StatusBarProps {
  version: string | null;
  venueName: string | null;
  /** The venue's IANA zone, so the clock is the venue's even on a tablet set wrong. */
  timezone: string | null;
}

const toneClass = {
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  muted: "text-fg-muted",
} as const;

export function StatusBar({ version, venueName, timezone }: StatusBarProps) {
  const { status } = useRealtime();
  const now = useClock(1000);
  const link = connectionText(status, now);
  const reaching = status.state === "live" || status.state === "syncing" || status.state === "connecting";
  const Wifi = reaching ? WifiIcon : WifiOffIcon;

  return (
    <footer className="flex h-10 shrink-0 items-center gap-6 border-t border-line bg-chrome px-7 text-sm text-fg-muted">
      <span>{versionText(version)}</span>
      <span role="status" aria-live="polite" className={cx("inline-flex items-center gap-1.5 font-semibold", toneClass[link.tone])}>
        <Wifi aria-hidden="true" size={15} strokeWidth={2} />
        {link.word}
        {link.detail && <span className="font-normal text-fg-muted">{link.detail}</span>}
      </span>
      {venueName && <span className="truncate">{venueName}</span>}
      <span className="ml-auto font-mono tabular-nums">{clockText(new Date(now), timezone)}</span>
    </footer>
  );
}
