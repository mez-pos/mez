// The next invoice and credit note numbers a draft would write, as it is typed
// (backoffice-settings.md, "Invoice numbering"). Asked of the server once typing
// pauses, so the answer is the one a save would give: the same template engine,
// the same counters, the same refusals.

import { useEffect, useState } from "react";
import type { NumberingRequest } from "../../../shared/api-types";
import { problemWith } from "./problem";
import type { NumberingApi } from "./use-tax";

export type Preview =
  | { state: "idle" }
  | { state: "loading" }
  | { state: "ok"; invoice: string; credit: string }
  | { state: "error"; message: string };

/** How long typing must pause before the server is asked. */
export const PREVIEW_DELAY_MS = 350;

/**
 * `input` is the draft as it would be sent, or null when it cannot be sent yet.
 * `key` changes exactly when the draft does. While `active` is false (nothing
 * typed) nothing is asked.
 */
export function useNumberingPreview(active: boolean, input: NumberingRequest | null, key: string, ask: NumberingApi["preview"]): Preview {
  const [preview, setPreview] = useState<Preview>({ state: "idle" });

  useEffect(() => {
    if (!active) {
      setPreview({ state: "idle" });
      return;
    }
    if (input === null) {
      setPreview({ state: "error", message: "Fix the rounding step to see the next numbers." });
      return;
    }
    const controller = new AbortController();
    const timer = setTimeout(() => {
      setPreview({ state: "loading" });
      ask(input, controller.signal)
        .then((n) => {
          if (!controller.signal.aborted) setPreview({ state: "ok", invoice: n.next_invoice_no, credit: n.next_credit_no });
        })
        .catch((err: unknown) => {
          if (!controller.signal.aborted) setPreview({ state: "error", message: problemWith(err, []).message });
        });
    }, PREVIEW_DELAY_MS);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
    // `key` stands for `input`: the object is new on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, key, ask]);

  return preview;
}
