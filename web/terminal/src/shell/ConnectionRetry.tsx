// The way to try the server again, in the top bar while it is down: a full
// 48px target, because the person who wants it has a tray in the other hand.
// The status bar says what is wrong; this is what to do about it. Nothing
// shows while the link is up.

import { WifiOffIcon } from "lucide-react";
import { describeConnection } from "../../../shared/connection-status";
import { useRealtime } from "../../../shared/use-realtime";

export function ConnectionRetry() {
  const { status, reconnectNow } = useRealtime();
  if (!describeConnection(status, 0).retryable) return null;
  return (
    <button
      type="button"
      onClick={reconnectNow}
      className="inline-flex min-h-tap items-center gap-2 rounded-pill border border-line-strong bg-raised px-4 text-base font-semibold outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
    >
      <WifiOffIcon aria-hidden="true" size={17} />
      Retry now
    </button>
  );
}
