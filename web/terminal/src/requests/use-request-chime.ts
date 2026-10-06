// The sound for a guest's request (ADR-209, ADR-212): the kitchen's chime,
// when a request arrives and again every half minute while one waits.
//
// A browser lets a page make sound only after someone has touched it. A
// terminal is touched to sign in, so by the time anyone is in the frame it
// has been; the first tap or key anywhere is taken as that touch. A page
// nobody has touched since it loaded stays silent, and the pill is what says
// a request is waiting.

import { useEffect, useRef } from "react";
import type { GuestRequestResponse } from "../../../shared/api-types";
import { audioContext, ring } from "../kitchen/use-chime";
import { arrived, remindEveryMs } from "./request-logic";

export function useRequestChime(requests: readonly Pick<GuestRequestResponse, "id">[], loaded: boolean): void {
  const ctx = useRef<AudioContext | null>(null);
  const seen = useRef<ReadonlySet<string> | null>(null);

  useEffect(() => {
    const touch = () => {
      ctx.current = audioContext();
    };
    window.addEventListener("pointerdown", touch);
    window.addEventListener("keydown", touch);
    return () => {
      window.removeEventListener("pointerdown", touch);
      window.removeEventListener("keydown", touch);
    };
  }, []);

  const chime = () => {
    if (ctx.current?.state === "running") ring(ctx.current);
  };

  useEffect(() => {
    if (!loaded) return;
    const came = arrived(seen.current, requests);
    seen.current = new Set(requests.map((r) => r.id));
    if (came.length > 0) chime();
  }, [loaded, requests]);

  const waiting = requests.length > 0;
  useEffect(() => {
    if (!waiting) return;
    const t = setInterval(chime, remindEveryMs);
    return () => clearInterval(t);
  }, [waiting]);
}
