// The left of the sign-in screen (D01): the brand, a headline and the script
// line under it, and the version along the bottom. Drawn on the sunken ground
// with a pale arch behind, as the design has it.
//
// The design's illustration of a chef at the pass is a labelled placeholder for
// artwork that does not exist yet, and so is the dark script card beside it:
// neither is drawn. The panel shows from 1280px wide: on a 1024 tablet it took
// half the screen, and the profiles, the pad and the foot did not fit beside it.

import { Logo } from "../../ui";
import { versionText } from "../../venue-text";

export function BrandPanel({ version }: { version: string | null }) {
  return (
    <aside aria-hidden="true" className="relative hidden w-[46%] shrink-0 flex-col overflow-hidden bg-sunken p-10 xl:flex 2xl:p-14">
      {/* The arch runs to the foot of the panel, however tall: a fixed square ended in a hard edge. */}
      <div className="absolute -right-36 -top-20 bottom-0 w-[620px] rounded-t-[999px] bg-line" />
      <div className="relative">
        <Logo className="h-14" />
        <p className="mt-3 text-meta font-bold uppercase tracking-[.22em] text-fg-muted">Good food brings people together</p>
        <h2 className="mt-14 font-serif text-display leading-[1.02] 2xl:text-hero">
          Great Food
          <br />
          Brighter Days
        </h2>
        <p className="mt-5 font-script text-display leading-tight text-fg-2">
          Same Tables.
          <br />
          New Stories. ♡
        </p>
      </div>
      <p className="relative mt-auto flex gap-10 pt-8 text-meta font-bold uppercase tracking-[.14em] text-fg-muted">
        <span>{versionText(version)}</span>
        <span>A small place for big moments</span>
      </p>
    </aside>
  );
}
