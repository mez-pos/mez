// The bar along the bottom of every screen (D03): which mez, whether the
// server is there, which venue, and the time. 40px, as drawn. The design also
// shows Keyboard Shortcuts and Open Cash Drawer here; neither exists, and a
// control that does nothing is not drawn (rebuild.md, rule 3).

import { useClock } from "../../../shared/use-clock";
import { clockText, versionText } from "../../../shared/venue-text";
import { ConnectionWord } from "./ConnectionWord";

interface StatusBarProps {
  version: string | null;
  venueName: string | null;
  /** The venue's IANA zone, so the clock is the venue's even on a tablet set wrong. */
  timezone: string | null;
}

export function StatusBar({ version, venueName, timezone }: StatusBarProps) {
  const now = useClock(1000);

  return (
    <footer className="flex h-10 shrink-0 items-center gap-6 border-t border-line bg-chrome px-7 text-sm text-fg-muted">
      <span>{versionText(version)}</span>
      <ConnectionWord />
      {venueName && <span className="truncate">{venueName}</span>}
      <span className="ml-auto font-mono tabular-nums">{clockText(new Date(now), timezone)}</span>
    </footer>
  );
}
