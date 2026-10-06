// The chime for a new ticket (the foot's "Sound on"). Two short notes from
// the browser's own oscillator: no sound file to load, so it works with no
// internet and adds nothing to the bundle. A browser lets a page make sound
// only after someone has touched it; turning Sound on is that touch.

import { useEffect, useRef } from "react";
import type { KitchenTicketResponse } from "../../../shared/api-types";
import { arrivals, ticketId } from "./board-prefs";

type AudioCtor = typeof AudioContext;

// One context for the page, made the first time sound is wanted. A browser
// caps how many a page may hold, and each keeps an audio thread alive, so a
// screen that mounts again reuses this one rather than making another.
let shared: AudioContext | null = null;

/** The page's audio context, resumed: made on first use, after a touch. Null where there is no audio. */
export function audioContext(): AudioContext | null {
  if (!shared || shared.state === "closed") {
    const Ctor = (window.AudioContext ?? (window as unknown as { webkitAudioContext?: AudioCtor }).webkitAudioContext) as AudioCtor | undefined;
    try {
      shared = Ctor ? new Ctor() : null;
    } catch {
      shared = null;
    }
  }
  void shared?.resume().catch(() => undefined);
  return shared;
}

/** Two rising notes, about a third of a second in all. */
export function ring(ctx: AudioContext): void {
  [660, 880].forEach((hz, i) => {
    const at = ctx.currentTime + i * 0.16;
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.type = "sine";
    osc.frequency.value = hz;
    gain.gain.setValueAtTime(0.0001, at);
    gain.gain.exponentialRampToValueAtTime(0.35, at + 0.02);
    gain.gain.exponentialRampToValueAtTime(0.0001, at + 0.15);
    osc.connect(gain).connect(ctx.destination);
    osc.start(at);
    osc.stop(at + 0.16);
  });
}

/**
 * Chimes when a ticket arrives in New Orders while sound is on. `fresh` is
 * undefined until the board has been read once, so a display turned on to a
 * full board stays quiet.
 */
export function useChime(on: boolean, fresh: readonly KitchenTicketResponse[] | undefined): void {
  const ctx = useRef<AudioContext | null>(null);
  const seen = useRef<ReadonlySet<string> | null>(null);

  // Made when sound is turned on, which is a tap: the browser allows it then.
  useEffect(() => {
    if (on) ctx.current = audioContext();
  }, [on]);

  useEffect(() => {
    if (!fresh) return;
    const came = arrivals(seen.current, fresh);
    seen.current = new Set(fresh.map(ticketId));
    if (on && came.length > 0 && ctx.current?.state === "running") ring(ctx.current);
  }, [on, fresh]);
}
