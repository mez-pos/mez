// The foot of the kitchen's board (D10, D11): what this display is set to.
// Change station, the chime for a new ticket, a table's tickets kept together,
// and espresso night. Each is the device's own choice (board-prefs.ts).
//
// Left out, and why. Settings: a display has one setting besides these, its
// station, so the button says that. Auto advance: nothing here moves a ticket
// on its own, and a ticket that left the board unmade would be a lost dish.
// Mark all complete: no ticket is "complete" on this board (a ready one waits
// to be collected, ADR-090), and one tap that clears a pass has no undo.

import { LockIcon, Settings2Icon, Volume2Icon, VolumeXIcon } from "lucide-react";
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

const plain =
  "inline-flex min-h-tap items-center gap-2.5 rounded-pill px-4 text-base font-semibold outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-4";

export function BoardFoot({ prefs, onPrefs, onChangeStation, signedIn }: BoardFootProps) {
  const Sound = prefs.sound ? Volume2Icon : VolumeXIcon;
  return (
    <footer className="flex min-h-19 shrink-0 flex-wrap items-center gap-x-5 gap-y-1 border-t border-line bg-chrome px-7 py-2">
      <button type="button" onClick={onChangeStation} className={plain}>
        <Settings2Icon aria-hidden="true" size={19} strokeWidth={1.85} />
        Change station
      </button>
      <button type="button" aria-pressed={prefs.sound} onClick={() => onPrefs({ ...prefs, sound: !prefs.sound })} className={plain}>
        <Sound aria-hidden="true" size={19} strokeWidth={1.85} />
        {prefs.sound ? "Sound on" : "Sound off"}
      </button>
      <Switch label="Group by table" on={prefs.groupByTable} onChange={(groupByTable) => onPrefs({ ...prefs, groupByTable })} />
      <Switch label="Night mode" on={prefs.night} onChange={(night) => onPrefs({ ...prefs, night })} />
      {signedIn ? (
        <button type="button" onClick={signedIn.onLock} className={cx(plain, "ml-auto border border-line-strong")}>
          <LockIcon aria-hidden="true" size={19} strokeWidth={1.85} />
          Lock · {signedIn.name}
        </button>
      ) : (
        <p aria-hidden="true" className="ml-auto hidden font-script text-xl text-fg-2 xl:block">
          Great Food, Happier People ♡
        </p>
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
      <span className="w-7 text-left text-sm font-semibold text-fg-muted">{on ? "On" : "Off"}</span>
    </button>
  );
}
