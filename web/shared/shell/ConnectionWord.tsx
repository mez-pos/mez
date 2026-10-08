// Whether the mez server is there, in a word with its icon: "Online", or what
// is wrong. The status bar says it on a desktop; a waiter's frame has no status
// bar (T01), so their top bar says it instead. Colour plus a word, never colour
// alone (tokens.md).

import { WifiIcon, WifiOffIcon } from "lucide-react";
import { cx } from "../ui/cx";
import { useClock } from "../use-clock";
import { useRealtime } from "../use-realtime";
import { connectionText } from "./status-logic";

const toneClass = {
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  muted: "text-fg-muted",
} as const;

interface ConnectionWordProps {
  /** The word alone, without "retrying in 4s": for a top bar with no room to spare. */
  brief?: boolean;
  className?: string;
}

export function ConnectionWord({ brief = false, className }: ConnectionWordProps) {
  const { status } = useRealtime();
  const now = useClock(1000);
  const link = connectionText(status, now);
  // Away is reached too: through the relay, with no stream (ADR-309).
  const reaching = status.state === "live" || status.state === "syncing" || status.state === "connecting" || status.state === "away";
  const Wifi = reaching ? WifiIcon : WifiOffIcon;

  return (
    <span role="status" aria-live="polite" className={cx("inline-flex items-center gap-1.5 whitespace-nowrap font-semibold", toneClass[link.tone], className)}>
      <Wifi aria-hidden="true" size={15} strokeWidth={2} />
      {link.word}
      {link.detail && !brief && <span className="font-normal text-fg-muted">{link.detail}</span>}
    </span>
  );
}
