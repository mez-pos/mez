// The venue settings frame (docs/design/screens/backoffice-settings.md): a
// rail of sections beside the one section open. Each section keeps its own
// draft and saves on its own; the frame only knows whether the open one has
// unsaved changes, so it can ask before leaving it.

import { useCallback, useEffect, useState, type MouseEvent } from "react";
import { Link, Redirect, useLocation } from "wouter";
import { Button, EmptyState, InboxIcon, Modal } from "../../../shared/ui";
import { DEFAULT_SECTION, SETTINGS_SECTIONS, builtSectionFor, problemOf, type SettingsSection } from "./settings-logic";
import { useSettings } from "./use-settings";
import { ShiftsSection } from "./ShiftsSection";
import { TaxSection } from "./TaxSection";
import { VenueSection } from "./VenueSection";

export function SettingsScreen({ path }: { path: string }) {
  const section = builtSectionFor(path);
  if (!section) return <Redirect to={DEFAULT_SECTION.path} replace />;
  return <Frame section={section} />;
}

function Frame({ section }: { section: SettingsSection }) {
  const api = useSettings();
  const [, navigate] = useLocation();
  // Which forms of the open section differ from what is saved: a section may
  // have several, each saving on its own.
  const [dirtyForms, setDirtyForms] = useState<ReadonlySet<string>>(new Set());
  const [leaving, setLeaving] = useState<string | null>(null);
  const dirty = dirtyForms.size > 0;
  const reportDirty = useCallback((form: string, d: boolean) => {
    setDirtyForms((prev) => {
      if (prev.has(form) === d) return prev;
      const next = new Set(prev);
      if (d) next.add(form);
      else next.delete(form);
      return next;
    });
  }, []);
  const venueDirty = useCallback((d: boolean) => reportDirty("venue", d), [reportDirty]);

  // The browser's own prompt when the tab is closed or reloaded on a draft.
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  const goTo = (e: MouseEvent, target: SettingsSection) => {
    if (dirty && target.path !== section.path) {
      e.preventDefault();
      setLeaving(target.path);
    }
  };

  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6">
      <h1 className="font-serif text-total">Venue settings</h1>
      <div className="flex flex-col gap-6 md:flex-row md:items-start">
        <Rail current={section} onGo={goTo} />
        {section.id === "venue" && (
          <>
            {api.loading && <p className="text-base text-fg-muted">Loading the settings</p>}
            {!api.loading && api.error !== null && (
              <EmptyState icon={<InboxIcon />} line={problemOf(api.error).message} action={{ label: "Try again", onClick: api.refetch }} />
            )}
            {api.data && <VenueSection venue={api.data.venue} onSave={api.saveVenue} onDirty={venueDirty} />}
          </>
        )}
        {section.id === "tax" && <TaxSection onDirty={reportDirty} />}
        {section.id === "shifts" && <ShiftsSection onDirty={reportDirty} />}
      </div>

      <Modal
        open={leaving !== null}
        onClose={() => setLeaving(null)}
        title="Leave without saving?"
        footer={
          <>
            <Button onClick={() => setLeaving(null)}>Keep editing</Button>
            <Button
              variant="primary"
              onClick={() => {
                const to = leaving;
                setLeaving(null);
                if (to) navigate(to);
              }}
            >
              Leave
            </Button>
          </>
        }
      >
        Your changes to {section.label} are not saved.
      </Modal>
    </div>
  );
}

// The same entries as the shell's navigation: the current one terracotta, a
// section not built yet muted with Soon, and not a link.
const entry = "flex min-h-tap items-center justify-between gap-3 rounded-pill px-4 text-base";

function Rail({ current, onGo }: { current: SettingsSection; onGo: (e: MouseEvent, target: SettingsSection) => void }) {
  return (
    <nav aria-label="Settings sections" className="shrink-0 md:w-56">
      <ul className="flex flex-wrap gap-1 md:flex-col">
        {SETTINGS_SECTIONS.map((s) => (
          <li key={s.id}>
            {s.built ? (
              <Link
                href={s.path}
                onClick={(e) => onGo(e, s)}
                aria-current={s.id === current.id ? "page" : undefined}
                className={
                  s.id === current.id
                    ? `${entry} bg-attention font-semibold text-attention-fg`
                    : `${entry} text-fg-2 hover:bg-attention-tint focus-visible:bg-attention-tint`
                }
              >
                {s.label}
              </Link>
            ) : (
              <span className={`${entry} text-fg-muted`}>
                {s.label}
                <span className="text-meta font-semibold uppercase text-fg-faint">Soon</span>
              </span>
            )}
          </li>
        ))}
      </ul>
    </nav>
  );
}
