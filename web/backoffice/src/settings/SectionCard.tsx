// The frame every settings section is drawn in, so each saves, discards and
// says it saved the same way (docs/design/screens/backoffice-settings.md,
// "How a section saves"). A section brings its fields and its own checks.

import type { FormEvent, ReactNode } from "react";
import { Button, Card } from "../../../shared/ui";

export interface SectionCardProps {
  title: string;
  description: string;
  /** The draft differs from what the server holds. */
  dirty: boolean;
  /** A save is out. */
  busy: boolean;
  /** The last save went through and nothing has been typed since. */
  saved: boolean;
  /** A refusal about no field of the form. */
  error: string | null;
  onSave: () => void;
  onDiscard: () => void;
  children: ReactNode;
}

export function SectionCard({ title, description, dirty, busy, saved, error, onSave, onDiscard, children }: SectionCardProps) {
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (dirty && !busy) onSave();
  };
  // One live region whose words change: a screen reader hears each in turn.
  const state = busy ? "Saving" : dirty ? "Unsaved changes" : saved ? "Saved" : "";

  return (
    <Card padding="lg" className="min-w-0 flex-1 md:p-8">
      <form onSubmit={submit} noValidate>
        <h2 className="font-serif text-xl">{title}</h2>
        <p className="mt-1 text-base text-fg-2">{description}</p>
        <div className="mt-6 flex max-w-xl flex-col gap-5">{children}</div>
        <div className="mt-8 flex flex-wrap items-center justify-between gap-4 border-t border-line pt-6">
          <div className="flex min-h-tap flex-col justify-center">
            <p role="status" className={saved && !dirty && !busy ? "flex items-center gap-1.5 text-base font-semibold text-accent-text" : "text-base text-fg-2"}>
              {saved && !dirty && !busy && <Check />}
              {state}
            </p>
            {error && (
              <p role="alert" className="text-sm text-danger">
                {error}
              </p>
            )}
          </div>
          <div className="flex gap-tap-gap">
            <Button disabled={!dirty || busy} onClick={onDiscard}>
              Discard
            </Button>
            <Button type="submit" variant="primary" disabled={!dirty || busy}>
              {busy ? "Saving" : "Save"}
            </Button>
          </div>
        </div>
      </form>
    </Card>
  );
}

function Check() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M5 12.5l4.5 4.5L19 7.5" />
    </svg>
  );
}
