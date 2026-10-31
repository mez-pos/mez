// The foot of the kitchen's board (D10, D11): what this display is set to.
// Change station, then switches, each saying On or Off: the chime, a table's
// tickets kept together, New Orders straight to ready for a bar, and espresso
// night, which is on until a bright kitchen turns it off. Each is the device's
// own choice (board-prefs.ts, ADR-381, ADR-392).
//
// Left out, and why. Settings: a display has one setting besides these, its
// station, so the button says that. Auto advance: nothing here moves a ticket
// on its own, and a ticket that left the board unmade would be a lost dish.
// Mark all complete: no ticket is "complete" on this board (a ready one waits
// to be collected, ADR-090), and one tap that clears a pass has no undo. D10's
// script line: a working screen carries none (ADR-381).

import { LockIcon, Settings2Icon } from "lucide-react";
import { useEffect, useState } from "react";
import { cx } from "../../../shared/ui/cx";
import type { BoardPrefs } from "./board-prefs";

/** Who is at the board, on a terminal they signed in on (ADR-179). */
export interface SignedIn {
  name: string;
  /** Signs them out: the terminal goes back to the sign-in screen. */
  onLock: () => void;
  /** They may move tickets (kitchen.work, ADR-180); without it the board is to look at. */
  canWork: boolean;
}

interface BoardFootProps {
  prefs: BoardPrefs;
  onPrefs: (prefs: BoardPrefs) => void;
  onChangeStation: () => void;
  /** Absent on the wall display, which nobody signs in on. */
  signedIn?: SignedIn | undefined;
}

const shape = "inline-flex min-h-tap items-center gap-2.5 rounded-pill px-4 text-base font-semibold outline-offset-2 outline-focus focus-visible:outline-3";
const plain = cx(shape, "hover:bg-raised-high");

export function BoardFoot({ prefs, onPrefs, onChangeStation, signedIn }: BoardFootProps) {
  // Leaving the board for the choice of station takes a second tap: a wet
  // hand brushing the foot does not take the tickets off the screen (KDS-15).
  const [armed, setArmed] = useState(false);
  useEffect(() => {
    if (!armed) return;
    const timer = setTimeout(() => setArmed(false), 4000);
    return () => clearTimeout(timer);
  }, [armed]);
  return (
    <footer className="flex min-h-19 shrink-0 flex-wrap items-center gap-x-5 gap-y-1 border-t border-line bg-chrome px-7 py-2 short:min-h-14 short:gap-x-4 short:px-5 short:py-1">
      <button
        type="button"
        onClick={() => {
          if (!armed) return setArmed(true);
          setArmed(false);
          onChangeStation();
        }}
        className={armed ? cx(shape, "bg-attention text-attention-fg hover:bg-attention-hover") : plain}
      >
        <Settings2Icon aria-hidden="true" size={19} strokeWidth={1.85} />
        {armed ? "Tap again to change station" : "Change station"}
      </button>
      {/* A switch like its neighbours: "Sound off" on a button read as a command (KDS-16). */}
      <Switch label="Sound" on={prefs.sound} onChange={(sound) => onPrefs({ ...prefs, sound })} />
      <Switch label="Group by table" on={prefs.groupByTable} onChange={(groupByTable) => onPrefs({ ...prefs, groupByTable })} />
      <Switch label="Skip preparing" on={prefs.skipPreparing} onChange={(skipPreparing) => onPrefs({ ...prefs, skipPreparing })} />
      <Switch label="Night mode" on={prefs.night} onChange={(night) => onPrefs({ ...prefs, night })} />
      {signedIn && (
        <button type="button" onClick={signedIn.onLock} className={cx(plain, "ml-auto border border-line-strong")}>
          <LockIcon aria-hidden="true" size={19} strokeWidth={1.85} />
          Lock · {signedIn.name}
        </button>
      )}
    </footer>
  );
}

interface SwitchProps {
  label: string;
  on: boolean;
  onChange: (on: boolean) => void;
}

/** A switch as the design draws it: the word, then a pill whose knob slides. On or off is said, not only shown. */
function Switch({ label, on, onChange }: SwitchProps) {
  return (
    <button type="button" role="switch" aria-checked={on} onClick={() => onChange(!on)} className={cx(plain, "font-normal")}>
      {label}
      <span
        aria-hidden="true"
        className={cx("flex h-7.5 w-13 items-center rounded-pill p-0.75", on ? "justify-end bg-accent" : "justify-start bg-line-strong")}
      >
        <span className="size-6 rounded-round bg-accent-fg" />
      </span>
      <span className="w-8 text-left text-base font-semibold text-fg-muted">{on ? "On" : "Off"}</span>
    </button>
  );
}
