// The wizard's page (D24): the brand and the welcome, the numbered steps, one
// card for whatever is being asked, and a script line along the foot. It
// scrolls as a page, where the design is a fixed card: a 768px screen has no
// room for 880px of it (setup-wizard.md).
//
// On a short screen the head gives way first: no small-caps line, a smaller
// logo, and the lead only where a run of steps begins. The step's own row of
// buttons, the last thing in the card, stays at the foot of the window while
// the card runs past it, so Continue is never a scroll away (SET-4).

import { CheckIcon, MinusIcon } from "lucide-react";
import { useRef, type ReactNode } from "react";
import { Logo } from "../../ui";
import { cx } from "../../ui/cx";
import { useStepArrival } from "./use-step-arrival";
import { stroke } from "../../ui/stroke";

export interface FrameStep {
  readonly key: string;
  readonly name: string;
}

interface SetupFrameProps {
  /** Under the welcome: how long this is, or what is going on instead. */
  lead: string;
  /** Whether the lead still shows on a short screen. It says what a run of steps is; once into it, the room is the card's. */
  leadWhenShort?: boolean;
  /** The steps and which is in hand; omitted where there are no questions to show. */
  steps?: readonly FrameStep[];
  current?: number;
  /** Steps passed over without doing them, by key: marked skipped, never done (SET-2). */
  skipped?: ReadonlySet<string>;
  /** Go back to a step already done. Omitted while nothing may be changed. */
  onStep?: ((index: number) => void) | undefined;
  children: ReactNode;
}

// Every step ends in its row of buttons (Continue, Skip for now, Start
// selling), the last child of its form. That row sticks to the foot of the
// page while the card runs past it, on the card's own ground and over its
// padding, so at rest it looks as it always has. The page keeps room for it
// at its foot (scroll-pb-28) when a field takes the focus.
const stickyFoot = cx(
  "[&>*>:last-child]:sticky [&>*>:last-child]:bottom-0 [&>*>:last-child]:bg-raised",
  "[&>*>:last-child]:-mx-6 [&>*>:last-child]:-mb-6 [&>*>:last-child]:px-6 [&>*>:last-child]:pb-6 [&>*>:last-child]:rounded-b-card",
);

export function SetupFrame({ lead, leadWhenShort = true, steps, current = 0, skipped = noneSkipped, onStep, children }: SetupFrameProps) {
  const page = useRef<HTMLElement>(null);
  const card = useRef<HTMLElement>(null);
  useStepArrival(steps ? `${steps.map((s) => s.key).join(" ")}#${current}` : null, page, card);

  return (
    <main ref={page} className="relative h-screen overflow-y-auto overflow-x-hidden scroll-pb-28 bg-sunken text-fg">
      <div aria-hidden="true" className="absolute -right-20 -top-14 size-104 rounded-round bg-line opacity-60" />
      <div className="relative mx-auto flex min-h-screen w-full max-w-232 flex-col gap-6 p-6 short:gap-4 short:p-4 md:p-8">
        <header>
          <Logo className="h-12 short:h-9" />
          <p className="mt-2 text-meta font-bold uppercase text-fg-muted short:hidden">Good food brings people together</p>
          <h1 className="mt-6 font-serif text-total leading-tight short:mt-3">Welcome. Let&rsquo;s set up your venue.</h1>
          <p className={cx("mt-2 max-w-xl text-base text-fg", !leadWhenShort && "short:hidden")}>{lead}</p>
        </header>

        {steps && <Steps steps={steps} current={current} skipped={skipped} onStep={onStep} />}

        <section ref={card} className={cx("flex flex-1 flex-col rounded-card border border-line bg-raised p-6 shadow-card [&_h2]:outline-none", stickyFoot)}>
          {children}
        </section>

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

const noneSkipped: ReadonlySet<string> = new Set();

interface StepsProps {
  steps: readonly FrameStep[];
  current: number;
  skipped: ReadonlySet<string>;
  onStep?: ((index: number) => void) | undefined;
}

function Steps({ steps, current, skipped, onStep }: StepsProps) {
  return (
    <ol aria-label="Steps" className="flex">
      {steps.map((step, i) => {
        // A skipped step is behind, but not done: a dashed round with a dash, and the word (SET-2).
        const skip = i < current && skipped.has(step.key);
        const done = i < current && !skip;
        const here = i === current;
        const last = i === steps.length - 1;
        const round = cx(
          "flex size-10 shrink-0 items-center justify-center rounded-round font-mono text-base font-bold",
          skip
            ? "border-2 border-dashed border-line-strong bg-raised text-fg-muted"
            : done || here
              ? "bg-accent text-accent-fg"
              : "bg-raised-high text-fg-muted",
        );
        return (
          <li key={step.key} aria-current={here ? "step" : undefined} className={cx("flex flex-col gap-2", !last && "flex-1")}>
            <div className="flex items-center">
              {done && onStep ? (
                <button
                  type="button"
                  onClick={() => onStep(i)}
                  aria-label={`Back to step ${i + 1}, ${step.name}`}
                  className={cx(round, "outline-offset-2 outline-focus hover:bg-accent-hover focus-visible:outline-3")}
                >
                  <CheckIcon aria-hidden="true" size={20} strokeWidth={stroke.bold} />
                </button>
              ) : (
                <span className={round}>
                  {skip ? (
                    <MinusIcon aria-hidden="true" size={20} strokeWidth={stroke.bold} />
                  ) : done ? (
                    <CheckIcon aria-hidden="true" size={20} strokeWidth={stroke.bold} />
                  ) : (
                    i + 1
                  )}
                </span>
              )}
              {!last && <span aria-hidden="true" className={cx("h-0.75 flex-1", i < current ? "bg-accent" : "bg-line-strong")} />}
            </div>
            <span className={cx("pr-3 text-sm", here ? "font-bold text-accent-text" : "font-semibold text-fg-2")}>
              <span className="sr-only">{skip ? "Skipped: " : done ? "Done: " : here ? "Now: " : ""}</span>
              {step.name}
              {skip && (
                <span aria-hidden="true" className="block font-normal text-fg-muted">
                  Skipped
                </span>
              )}
            </span>
          </li>
        );
      })}
    </ol>
  );
}
