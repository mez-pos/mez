// The way to try the server again, in the top bar while it is down: a full
// 48px target, because the person who wants it has a tray in the other hand.
// The status bar says what is wrong; this is what to do about it. Nothing
// shows while the link is up.

import { WifiOffIcon } from "lucide-react";
import { describeConnection } from "../../../shared/connection-status";
import { useRealtime } from "../../../shared/use-realtime";
import { Button } from "../../../shared/ui/Button";

export function ConnectionRetry() {
  const { status, reconnectNow } = useRealtime();
  if (!describeConnection(status, 0).retryable) return null;
  return (
    <Button onClick={reconnectNow}>
      <WifiOffIcon aria-hidden="true" size={17} />
      Retry now
    </Button>
  );
}
