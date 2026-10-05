// One card of the dashboard (D12): a serif title, a link to the screen that
// has the rest, and the card's rows. While it reads, or when the read fails,
// the card says so in the rows' place, with a way to try again.

import type { ReactNode } from "react";
import { Link } from "wouter";
import { Button, Card } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";

interface PanelProps {
  title: string;
  /** The screen that has the whole of it, or a way to the rest of it further down this one. */
  more?: { label: string; href: string } | { label: string; onClick: () => void };
  /** Something beside the title: the chart's own note. */
  aside?: ReactNode;
  className?: string;
  children: ReactNode;
}

const moreClass =
  "-my-3 inline-flex min-h-tap shrink-0 items-center rounded-tight text-base font-bold text-attention-text outline-offset-2 outline-line-focus hover:underline focus-visible:outline-2";

export function Panel({ title, more, aside, className, children }: PanelProps) {
  return (
    <Card padding="lg" className={cx("flex min-w-0 flex-col", className)}>
      <div className="flex items-center justify-between gap-4">
        <h2 className="font-serif text-xl">{title}</h2>
        {aside}
        {more && "href" in more && (
          <Link href={more.href} className={moreClass}>
            {more.label}
          </Link>
        )}
        {more && "onClick" in more && (
          <button type="button" onClick={more.onClick} className={moreClass}>
            {more.label}
          </button>
        )}
      </div>
      <div className="mt-3 min-w-0 flex-1">{children}</div>
    </Card>
  );
}

interface ReadStateProps {
  /** What is being read, as the sentence's object: "today's sales". */
  what: string;
  failed: boolean;
  onRetry: () => void;
}

/** In a card's place while its figures are on their way, or did not come. */
export function ReadState({ what, failed, onRetry }: ReadStateProps) {
  if (!failed) return <p className="py-3 text-sm text-fg-muted">Reading {what}</p>;
  return (
    <div role="alert" className="flex flex-wrap items-center gap-3 py-3">
      <p className="text-sm text-fg-2">Could not read {what}.</p>
      <Button size="tap" onClick={onRetry}>
        Try again
      </Button>
    </div>
  );
}

/** A card with nothing to list says so in a sentence. */
export function Nothing({ children }: { children: ReactNode }) {
  return <p className="py-3 text-sm text-fg-2">{children}</p>;
}
