// Whether the mez server is there, in a word with its icon: "Online", or what
// is wrong. The status bar says it on a desktop; a waiter's frame has no status
// bar (T01), so their top bar says it instead. Colour plus a word, never colour
// alone (tokens.md).

import { WifiIcon, WifiOffIcon } from "lucide-react";
import { cx } from "../ui/cx";
import { useClock } from "../use-clock";
import { useMirror } from "../use-mirror";
import { useRealtime } from "../use-realtime";
import { connectionText, venueSilent } from "./status-logic";
import { stroke } from "../ui/stroke";

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
  const mirror = useMirror();
  const link = connectionText(status, now, mirror);
  // Away is reached too: through the relay, with no stream (ADR-309). A
  // mirror whose venue has gone quiet is not: it is the venue the icon is of.
  const reaching =
    status.state === "live" || status.state === "syncing" || status.state === "connecting" || (status.state === "away" && !venueSilent(status, mirror));
  const Wifi = reaching ? WifiIcon : WifiOffIcon;

  return (
    <span className={cx("inline-flex items-center gap-1.5 whitespace-nowrap font-semibold", toneClass[link.tone], className)}>
      <Wifi aria-hidden="true" size={15} strokeWidth={stroke.bold} className="shrink-0" />
      {/* In a bar with no room it is cut short with an ellipsis and never pushes its neighbours out.
          Only the word is a live region: it changes when the connection does, and is said once.
          The detail beside it counts down every second, and is read only when it is reached (TECH-11). */}
      <span role="status" aria-live="polite" className="min-w-0 truncate">
        {link.word}
      </span>
      {link.detail && !brief && <span className="font-normal text-fg-muted">{link.detail}</span>}
    </span>
  );
}
