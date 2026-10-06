// The setup wizard (docs/design/screens/setup-wizard.md, D24): the questions
// that make a venue, asked once on the computer mez runs on (ADR-267). It is
// shown only while the server says there is no venue and that this browser is
// where one can be made (ADR-266).

import { ArrowRightIcon } from "lucide-react";
import { useMemo, useRef, useState, type FormEvent } from "react";
import { Redirect, useLocation } from "wouter";
import { paths } from "../../auth/route";
import { useAuthStore } from "../../auth/use-auth";
import { Button } from "../../ui";
import { Splash } from "../Splash";
import { PrinterStep } from "./PrinterStep";
import { ReadyStep } from "./ReadyStep";
import { SetupFrame } from "./SetupFrame";
import {
  allSteps,
  browserTimezone,
  checkStep,
  finishSetup,
  notStarted,
  stageWords,
  startingAnswers,
  stepOf,
  steps,
  timezoneList,
  withProfile,
  type Answers,
  type Failure,
  type Problems,
  type Progress,
  type TaxProfile,
} from "./setup-logic";
import { TaxesStep, VenueStep, YouStep } from "./steps";
import { useSetupStatus } from "./use-setup-status";

const lead = `${allSteps.length} steps. You can change everything later in Settings.`;

export function SetupWizard() {
  const status = useSetupStatus();
  // Once Create has begun the venue exists or is about to, and the server's
  // "no setup needed" is then this wizard's own doing, not a reason to leave.
  const [begun, setBegun] = useState(false);

  if (!begun) {
    if (status.isPending) return <Splash />;
    if (status.isError) {
      return (
        <SetupFrame lead="mez could not be reached.">
          <Notice title="Can't reach mez" body={`The server at ${window.location.host} did not answer. Check that mez is running on this computer.`}>
            <Button variant="primary" onClick={() => void status.refetch()}>
              Try again
            </Button>
          </Notice>
        </SetupFrame>
      );
    }
    if (!status.data.needed) return <Redirect to={paths.home} replace />;
    if (!status.data.here) {
      return (
        <SetupFrame lead="It takes a few minutes, on the computer mez runs on.">
          <Notice
            title="This venue is not set up yet"
            body="Setting up is done on the computer mez is installed on, not from another device. Open mez there, from its shortcut on the desktop or at localhost in a browser, and come back here to pair this device afterwards."
          >
            <Button variant="secondary" onClick={() => void status.refetch()}>
              Check again
            </Button>
          </Notice>
        </SetupFrame>
      );
    }
  }
  return <Questions onBegin={() => setBegun(true)} />;
}

function Questions({ onBegin }: { onBegin: () => void }) {
  const store = useAuthStore();
  const [, navigate] = useLocation();
  const timezones = useMemo(() => timezoneList(browserTimezone()), []);
  const [answers, setAnswers] = useState<Answers>(() => startingAnswers(browserTimezone()));
  // What the owner set by hand, so choosing a tax profile does not undo it.
  const touched = useRef(new Set<keyof Answers>());
  const [step, setStep] = useState(0);
  const [problems, setProblems] = useState<Problems>({});
  const [progress, setProgress] = useState<Progress>(notStarted);
  const [failure, setFailure] = useState<Failure | null>(null);
  // Which of the steps after Create is showing.
  const [after, setAfter] = useState<"printer" | "ready">("printer");
  const [busy, setBusy] = useState(false);

  const current = steps[step] ?? steps[0];
  const last = step === steps.length - 1;
  // After the venue is made its answers are spent: there is nothing to go back and change.
  const created = progress.made !== null;

  const change = <K extends keyof Answers>(key: K, value: Answers[K]) => {
    touched.current.add(key);
    setAnswers((a) => ({ ...a, [key]: value }));
    setProblems({});
  };
  const chooseProfile = (profile: TaxProfile) => {
    setAnswers((a) => withProfile(a, profile, touched.current));
    setProblems({});
  };

  const create = async () => {
    setBusy(true);
    setFailure(null);
    onBegin();
    const out = await finishSetup(
      { api: store.api, pair: (code) => store.pair(code), signIn: (id, pin) => store.signIn(id, pin) },
      answers,
      progress,
      window.location.host,
      setProgress,
    );
    setProgress(out.progress);
    setBusy(false);
    // Done: the venue exists and the owner is signed in. What is left is the printer.
    if (out.progress.stage === "done") return;
    const failed = out.failure;
    if (failed?.kind === "field" && failed.field) {
      // A refused answer: back to its step, with the server's words under it.
      const at = steps.findIndex((s) => s.key === stepOf(failed.field ?? ""));
      if (at >= 0) setStep(at);
      setProblems({ [failed.field]: failed.message });
      return;
    }
    setFailure(failed);
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (busy) return;
    const found = checkStep(current.key, answers, timezones);
    if (Object.keys(found).length > 0) {
      setProblems(found);
      return;
    }
    setProblems({});
    if (last) void create();
    else setStep(step + 1);
  };

  const leave = () => navigate(paths.home, { replace: true });

  if (progress.stage === "done") {
    return (
      <SetupFrame
        lead="Your venue is ready. Two things more, if you have them to hand."
        steps={allSteps}
        current={after === "printer" ? steps.length : steps.length + 1}
      >
        {after === "printer" ? <PrinterStep api={store.api} onDone={() => setAfter("ready")} /> : <ReadyStep api={store.api} onDone={leave} />}
      </SetupFrame>
    );
  }

  if (busy || created || failure) {
    return (
      <SetupFrame lead={lead} steps={allSteps} current={steps.length - 1}>
        <Creating progress={progress} failure={failure} busy={busy} onRetry={() => void create()} onLeave={leave} onEdit={() => setFailure(null)} />
      </SetupFrame>
    );
  }

  const stepProps = { answers, problems, onChange: change };
  return (
    <SetupFrame lead={lead} steps={allSteps} current={step} onStep={setStep}>
      <form onSubmit={submit} noValidate className="flex flex-1 flex-col">
        <h2 className="font-serif text-xl">{current.name}</h2>
        <p className="mt-1 text-sm text-fg-muted">{blurbs[current.key]}</p>
        <div className="mt-5">
          {current.key === "venue" && <VenueStep {...stepProps} timezones={timezones} />}
          {current.key === "taxes" && <TaxesStep {...stepProps} onProfile={chooseProfile} />}
          {current.key === "you" && <YouStep {...stepProps} />}
        </div>
        <div className="mt-auto flex flex-wrap items-center gap-3 pt-6">
          <p className="text-sm text-fg-muted">
            Step {step + 1} of {allSteps.length}
          </p>
          <div className="ml-auto flex gap-3">
            {step > 0 && (
              <Button
                className="min-w-32"
                onClick={() => {
                  setProblems({});
                  setStep(step - 1);
                }}
              >
                Back
              </Button>
            )}
            <Button type="submit" variant="primary" className="min-w-44">
              {last ? "Create the venue" : "Continue"}
              <ArrowRightIcon aria-hidden="true" size={18} strokeWidth={2.2} />
            </Button>
          </div>
        </div>
      </form>
    </SetupFrame>
  );
}

const blurbs: Readonly<Record<(typeof steps)[number]["key"], string>> = {
  venue: "What the venue is called, and where and in what money it trades.",
  taxes: "How tax is charged and how invoices are numbered. Ask your accountant if you are unsure: both can be changed until the first invoice.",
  you: "The first person on the till. Everyone else is added afterwards, each with a PIN of their own.",
};

interface CreatingProps {
  progress: Progress;
  failure: Failure | null;
  busy: boolean;
  onRetry: () => void;
  onLeave: () => void;
  /** Back to the questions: only offered before a venue exists. */
  onEdit: () => void;
}

/** Create, as it runs and where it stops (setup-wizard.md, States). */
function Creating({ progress, failure, busy, onRetry, onLeave, onEdit }: CreatingProps) {
  if (!failure) {
    return (
      <Notice title={stageWords[progress.stage]} body="This takes a moment. Leave this page open.">
        <p role="status" className="text-sm text-fg-muted">
          {busy ? "Working" : "Finishing"}
        </p>
      </Notice>
    );
  }
  const created = progress.made !== null;
  const title =
    failure.kind === "taken" ? "Already set up" : failure.kind === "prefix" ? "Almost there" : created ? "The venue is created" : "That did not work";
  return (
    <Notice title={title} body={failure.message} alert>
      {failure.kind === "retry" && (
        <Button variant="primary" onClick={onRetry}>
          Try again
        </Button>
      )}
      {failure.kind === "retry" && !created && <Button onClick={onEdit}>Back to the questions</Button>}
      {failure.kind === "retry" && progress.stage === "prefix" && <Button onClick={onLeave}>Continue without it</Button>}
      {failure.kind === "prefix" && (
        <Button variant="primary" onClick={onLeave}>
          Continue
        </Button>
      )}
      {(failure.kind === "taken" || failure.kind === "no_code") && (
        <Button variant="primary" onClick={onLeave}>
          Pair this device
        </Button>
      )}
    </Notice>
  );
}

function Notice({ title, body, alert, children }: { title: string; body: string; alert?: boolean; children?: React.ReactNode }) {
  return (
    <div className="flex flex-1 flex-col items-start justify-center gap-3 py-6">
      <h2 className="font-serif text-xl">{title}</h2>
      <p role={alert ? "alert" : undefined} className="max-w-xl text-base text-fg">
        {body}
      </p>
      <div className="mt-3 flex flex-wrap gap-3">{children}</div>
    </div>
  );
}
