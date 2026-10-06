// The wizard's page (D24): the brand and the welcome, the numbered steps, one
// card for whatever is being asked, and a script line along the foot. It
// scrolls as a page, where the design is a fixed card: a 768px screen has no
// room for 880px of it (setup-wizard.md).

import { CheckIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Logo } from "../../ui";
import { cx } from "../../ui/cx";

export interface FrameStep {
  readonly key: string;
  readonly name: string;
}

interface SetupFrameProps {
  /** Under the welcome: how long this is, or what is going on instead. */
  lead: string;
  /** The steps and which is in hand; omitted where there are no questions to show. */
  steps?: readonly FrameStep[];
  current?: number;
  /** Go back to a step already done. Omitted while nothing may be changed. */
  onStep?: ((index: number) => void) | undefined;
  children: ReactNode;
}

export function SetupFrame({ lead, steps, current = 0, onStep, children }: SetupFrameProps) {
  return (
    <main className="relative min-h-screen overflow-x-hidden bg-sunken text-fg">
      <div aria-hidden="true" className="absolute -right-20 -top-14 size-[26rem] rounded-round bg-line opacity-60" />
      <div className="relative mx-auto flex min-h-screen w-full max-w-[58rem] flex-col gap-6 p-6 short:gap-4 short:p-4 md:p-8">
        <header>
          <Logo className="h-12" />
          <p className="mt-2 text-meta font-bold uppercase tracking-[.2em] text-fg-muted">Good food brings people together</p>
          <h1 className="mt-6 font-serif text-total leading-tight short:mt-4">Welcome. Let&rsquo;s set up your venue.</h1>
          <p className="mt-2 max-w-xl text-base text-fg">{lead}</p>
        </header>

        {steps && <Steps steps={steps} current={current} onStep={onStep} />}

        <section className="flex flex-1 flex-col rounded-card border border-line bg-raised p-6 shadow-card">{children}</section>

        <footer className="flex flex-wrap items-center gap-4">
          <p aria-hidden="true" className="font-script text-xl text-fg-2">
            A small place for big moments ♡
          </p>
          <p className="ml-auto text-sm text-fg-muted">
            <span className="font-mono">{window.location.host}</span>
          </p>
        </footer>
      </div>
    </main>
  );
}

function Steps({ steps, current, onStep }: { steps: readonly FrameStep[]; current: number; onStep?: ((index: number) => void) | undefined }) {
  return (
    <ol aria-label="Steps" className="flex">
      {steps.map((step, i) => {
        const done = i < current;
        const here = i === current;
        const last = i === steps.length - 1;
        const round = cx(
          "flex size-10 shrink-0 items-center justify-center rounded-round font-mono text-base font-bold",
          done || here ? "bg-accent text-accent-fg" : "bg-raised-high text-fg-muted",
        );
        return (
          <li key={step.key} aria-current={here ? "step" : undefined} className={cx("flex flex-col gap-2", !last && "flex-1")}>
            <div className="flex items-center">
              {done && onStep ? (
                <button
                  type="button"
                  onClick={() => onStep(i)}
                  aria-label={`Back to step ${i + 1}, ${step.name}`}
                  className={cx(round, "outline-offset-2 outline-attention hover:bg-accent-hover focus-visible:outline-3")}
                >
                  <CheckIcon aria-hidden="true" size={20} strokeWidth={3} />
                </button>
              ) : (
                <span className={round}>{done ? <CheckIcon aria-hidden="true" size={20} strokeWidth={3} /> : i + 1}</span>
              )}
              {!last && <span aria-hidden="true" className={cx("h-[3px] flex-1", done ? "bg-accent" : "bg-line-strong")} />}
            </div>
            <span className={cx("pr-3 text-sm", here ? "font-bold text-accent-text" : "font-semibold text-fg-2")}>
              <span className="sr-only">{done ? "Done: " : here ? "Now: " : ""}</span>
              {step.name}
            </span>
          </li>
        );
      })}
    </ol>
  );
}
