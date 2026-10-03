// A preview the server works out for a draft, as it is typed
// (backoffice-settings.md): the next invoice numbers, a sample receipt. Asked
// once typing pauses, so the answer is the one a save would give: the same
// rules, the same refusals. An answer for a draft that is no longer the one on
// screen is dropped.

import { useEffect, useState } from "react";
import { problemWith } from "./problem";

export type Preview<T> =
  | { state: "idle" }
  | { state: "loading" }
  | { state: "ok"; value: T }
  | { state: "error"; message: string };

/** How long typing must pause before the server is asked. */
export const PREVIEW_DELAY_MS = 350;

export interface PreviewOptions<I, T> {
  /** While false (nothing typed, say) nothing is asked. */
  active: boolean;
  /** The draft as it would be sent, or null when it cannot be sent yet. */
  input: I | null;
  /** Said when `input` is null. */
  missing: string;
  /** Changes exactly when the draft does: `input` is a new object every render. */
  key: string;
  ask: (input: I, signal: AbortSignal) => Promise<T>;
}

export function usePreview<I, T>({ active, input, missing, key, ask }: PreviewOptions<I, T>): Preview<T> {
  const [preview, setPreview] = useState<Preview<T>>({ state: "idle" });

  useEffect(() => {
    if (!active) {
      setPreview({ state: "idle" });
      return;
    }
    if (input === null) {
      setPreview({ state: "error", message: missing });
      return;
    }
    const controller = new AbortController();
    const timer = setTimeout(() => {
      // Keep what is on screen while the next answer is worked out, so a receipt
      // does not blink to a message on every keystroke.
      setPreview((p) => (p.state === "ok" ? p : { state: "loading" }));
      ask(input, controller.signal)
        .then((value) => {
          if (!controller.signal.aborted) setPreview({ state: "ok", value });
        })
        .catch((err: unknown) => {
          if (!controller.signal.aborted) setPreview({ state: "error", message: problemWith(err, []).message });
        });
    }, PREVIEW_DELAY_MS);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, key, ask]);

  return preview;
}
