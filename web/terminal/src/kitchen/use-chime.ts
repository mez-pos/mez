// The chime for a new ticket (the foot's "Sound on"). Two rising notes from
// the browser's own oscillator, twice: no sound file to load, so it works with
// no internet and adds nothing to the bundle. A browser lets a page make sound
// only after someone has touched it, so a display that reloads with sound on
// is blocked until a tap: the board says so, and any tap on it unblocks the
// chime (KDS-1, ADR-391).

import { useEffect, useRef, useSyncExternalStore } from "react";
import type { KitchenTicketResponse } from "../../../shared/api-types";
import { arrivals, ticketId } from "./board-prefs";

type AudioCtor = typeof AudioContext;

// One context for the page, made the first time sound is wanted. A browser
// caps how many a page may hold, and each keeps an audio thread alive, so a
// screen that mounts again reuses this one rather than making another.
let shared: AudioContext | null = null;
// Told when the context is made or its state changes, so the board can say it is blocked.
const listeners = new Set<() => void>();
const tell = () => listeners.forEach((l) => l());

/** The page's audio context, resumed: made on first use, after a touch. Null where there is no audio. */
export function audioContext(): AudioContext | null {
  if (!shared || shared.state === "closed") {
    const Ctor = (window.AudioContext ?? (window as unknown as { webkitAudioContext?: AudioCtor }).webkitAudioContext) as AudioCtor | undefined;
    try {
      shared = Ctor ? new Ctor() : null;
    } catch {
      shared = null;
    }
    shared?.addEventListener("statechange", tell);
    tell();
  }
  void shared?.resume().catch(() => undefined);
  return shared;
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** Whether the page can sound: "running", "suspended" until a tap, or "none" before a context is made. */
function audioState(): AudioContextState | "none" {
  return shared?.state ?? "none";
}

/**
 * Two rising notes, then again half a second on, about a second in all. A
 * pass is loud, and one missed note was the ticket missed: so twice, and
 * louder than a notification, in a triangle wave that carries over a fan.
 */
export function ring(ctx: AudioContext): void {
  [0, 0.5].forEach((start) => [660, 880].forEach((hz, i) => note(ctx, hz, ctx.currentTime + start + i * 0.16)));
}

function note(ctx: AudioContext, hz: number, at: number): void {
  const osc = ctx.createOscillator();
  const gain = ctx.createGain();
  osc.type = "triangle";
  osc.frequency.value = hz;
  gain.gain.setValueAtTime(0.0001, at);
  gain.gain.exponentialRampToValueAtTime(0.7, at + 0.02);
  gain.gain.exponentialRampToValueAtTime(0.0001, at + 0.15);
  osc.connect(gain).connect(ctx.destination);
  osc.start(at);
  osc.stop(at + 0.16);
}

export interface Chime {
  /** Sound is on and the browser is holding it back until someone taps. */
  blocked: boolean;
  /** Called from a tap: the browser lets the chime sound from then on. */
  unblock: () => void;
}

/**
 * Chimes when a ticket arrives in New Orders while sound is on. `fresh` is
 * undefined until the board has been read once, so a display turned on to a
 * full board stays quiet.
 */
export function useChime(on: boolean, fresh: readonly KitchenTicketResponse[] | undefined): Chime {
  const ctx = useRef<AudioContext | null>(null);
  const seen = useRef<ReadonlySet<string> | null>(null);
  const state = useSyncExternalStore(subscribe, audioState);

  // Made when sound is on: by a tap, or on load, when the browser holds it
  // suspended until the next tap.
  useEffect(() => {
    if (on) ctx.current = audioContext();
  }, [on]);

  // While it is held back, the first tap anywhere on the board lets it sound:
  // a cook touches the board soon enough, and the band says to.
  useEffect(() => {
    if (!on || state === "running" || state === "none") return;
    const touch = () => void audioContext();
    window.addEventListener("pointerdown", touch, { capture: true });
    return () => window.removeEventListener("pointerdown", touch, { capture: true });
  }, [on, state]);

  useEffect(() => {
    if (!fresh) return;
    const came = arrivals(seen.current, fresh);
    seen.current = new Set(fresh.map(ticketId));
    if (on && came.length > 0 && ctx.current?.state === "running") ring(ctx.current);
  }, [on, fresh]);

  return { blocked: on && state !== "running" && state !== "none", unblock: () => void audioContext() };
}
