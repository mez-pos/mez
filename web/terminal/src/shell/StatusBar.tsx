// The bar along the bottom of every screen (D03): which mez, whether the
// server is there, which venue, and the time. 40px, as drawn. The design also
// shows Keyboard Shortcuts and Open Cash Drawer here; neither exists, and a
// control that does nothing is not drawn (rebuild.md, rule 3).

import { useClock } from "../../../shared/use-clock";
import type { VenueClock } from "../../../shared/venue-clock";
import { barClock, versionText } from "../../../shared/venue-text";
import { ConnectionWord } from "../../../shared/shell/ConnectionWord";

interface StatusBarProps {
  version: string | null;
  venueName: string | null;
  /** The venue's zone and day start, so the clock is the venue's even on a tablet set wrong, and says the trading day after midnight. */
  clock: VenueClock;
}

export function StatusBar({ version, venueName, clock }: StatusBarProps) {
  const now = useClock(1000);
  const bar = barClock(new Date(now), clock);

  return (
    <footer className="flex h-10 shrink-0 items-center gap-6 border-t border-line bg-chrome px-7 text-sm text-fg-muted">
      <span>{versionText(version)}</span>
      <ConnectionWord />
      {venueName && <span className="truncate">{venueName}</span>}
      <span className="ml-auto font-mono tabular-nums" title={bar.label}>
        {bar.text}
      </span>
    </footer>
  );
}
