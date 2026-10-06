// Whether this screen is talking to the server, at a glance. Colour and word,
// and a "retry now" control while it is down: a full tap target, because the
// person who wants it has a tray in the other hand.

import { describeConnection, type Tone } from "./connection-status";
import { useClock } from "./use-clock";
import { useRealtime } from "./use-realtime";

const toneClass: Record<Tone, string> = {
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  muted: "text-fg-muted",
};

export function ConnectionIndicator() {
  const { status, reconnectNow } = useRealtime();
  // Once a second while a countdown is showing, and not otherwise.
  const now = useClock(1000, status.state === "reconnecting");
  const { tone, word, detail, retryable } = describeConnection(status, now);

  const body = (
    <>
      <span aria-hidden="true" className={`size-2 shrink-0 rounded-round bg-current ${toneClass[tone]}`} />
      <span className={`font-semibold ${toneClass[tone]}`}>{word}</span>
      {detail && <span className="text-fg-muted">{detail}</span>}
    </>
  );
  const shape = "inline-flex min-h-tap items-center gap-2 rounded-pill border border-line bg-raised px-4 text-base";

  return (
    <div role="status" aria-live="polite" className="inline-flex items-center gap-tap-gap">
      {retryable ? (
        <button type="button" onClick={reconnectNow} className={`${shape} hover:bg-raised-high`}>
          {body}
          <span className="font-semibold text-fg">Retry now</span>
        </button>
      ) : (
        <span className={shape}>{body}</span>
      )}
    </div>
  );
}
