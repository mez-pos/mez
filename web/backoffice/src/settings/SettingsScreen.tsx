// The settings frame (D22): a rail of sections with an icon each, beside the
// one section open. Each section keeps its own draft and saves on its own; the
// frame only knows whether the open one has unsaved changes, so it can ask
// before leaving it.
//
// The design draws every section on one screen with one Save. Here a section
// is a page and saves itself (ADR-129): a refusal is about the form in front
// of you. Its Printers & devices and Tables & QR codes are screens of their
// own, so the rail links out to them. Integrations and Language & theme are
// not drawn: there are none, and the kitchen's night theme is the display's
// own switch.

import { ArmchairIcon, DatabaseIcon, MonitorSmartphoneIcon, PercentIcon, PrinterIcon, ReceiptTextIcon, StoreIcon, type LucideIcon } from "lucide-react";
import { useCallback, useEffect, useState, type MouseEvent } from "react";
import { Link, Redirect, useLocation } from "wouter";
import { Button, Card, EmptyState, InboxIcon, Modal } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useServerVersion } from "../../../shared/use-venue";
import { versionText } from "../../../shared/venue-text";
import { usePermissions } from "../shell/BackOfficeShell";
import {
  DEFAULT_SECTION,
  SETTINGS_SECTIONS,
  builtSectionFor,
  problemOf,
  relatedScreens,
  type RelatedScreen,
  type SettingsSection,
  type SettingsSectionId,
} from "./settings-logic";
import { BackupSection } from "./BackupSection";
import { PaymentsCard } from "./PaymentsCard";
import { ShiftsSection } from "./ShiftsSection";
import { TaxSection } from "./TaxSection";
import { usePayments } from "./use-payments";
import { useSettings } from "./use-settings";
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
  const paymentsDirty = useCallback((d: boolean) => reportDirty("payments", d), [reportDirty]);

  // The browser's own prompt when the tab is closed or reloaded on a draft.
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  // Any way out of the open section from the rail asks first when it has a draft.
  const goTo = (e: MouseEvent, path: string) => {
    // A click that opens another tab leaves this one, and its draft, where it is.
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    if (dirty && path !== section.path) {
      e.preventDefault();
      setLeaving(path);
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <h1 className="font-serif text-total">Settings</h1>
      <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-[var(--aside)_minmax(0,1fr)]">
        <Rail current={section} onGo={goTo} />
        <div className="flex min-w-0 flex-col gap-4">
          {section.id === "venue" && (
            <>
              {api.loading && <p className="text-base text-fg-muted">Loading the settings</p>}
              {!api.loading && api.error !== null && (
                <EmptyState icon={<InboxIcon />} line={problemOf(api.error).message} action={{ label: "Try again", onClick: api.refetch }} />
              )}
              {api.data && <VenueSection venue={api.data.venue} onSave={api.saveVenue} onDirty={venueDirty} />}
              {api.data && <Payments onDirty={paymentsDirty} />}
            </>
          )}
          {section.id === "tax" && <TaxSection onDirty={reportDirty} />}
          {section.id === "shifts" && <ShiftsSection onDirty={reportDirty} />}
          {section.id === "backup" && <BackupSection onDirty={reportDirty} />}
        </div>
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
                // The draft is given up: nothing is dirty where we are going.
                setDirtyForms(new Set());
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

// Records over the ids, so a section or a screen added to the lists fails the build until it has an icon.
const sectionIcons: Record<SettingsSectionId, LucideIcon> = { venue: StoreIcon, tax: PercentIcon, shifts: ReceiptTextIcon, backup: DatabaseIcon };
const relatedIcons: Record<RelatedScreen["id"], LucideIcon> = { printers: PrinterIcon, devices: MonitorSmartphoneIcon, floor: ArmchairIcon };

// The same entries as the shell's navigation: the current one terracotta, a
// section not built yet muted with Soon, and not a link.
const entry = "flex min-h-tap items-center gap-3 rounded px-4 text-base outline-offset-2 outline-line-focus focus-visible:outline-2";

function Rail({ current, onGo }: { current: SettingsSection; onGo: (e: MouseEvent, path: string) => void }) {
  const related = relatedScreens(usePermissions());
  const version = useServerVersion();
  return (
    <Card className="flex flex-col gap-4 p-4">
      <nav aria-label="Settings sections">
        <ul className="flex flex-wrap gap-1 lg:flex-col">
          {SETTINGS_SECTIONS.map((s) => {
            const Icon = sectionIcons[s.id];
            return (
              <li key={s.id}>
                {s.built ? (
                  <Link
                    href={s.path}
                    onClick={(e) => onGo(e, s.path)}
                    aria-current={s.id === current.id ? "page" : undefined}
                    className={cx(entry, s.id === current.id ? "bg-attention font-semibold text-attention-fg" : "hover:bg-raised-high")}
                  >
                    <Icon aria-hidden="true" size={19} className="shrink-0" />
                    {s.label}
                  </Link>
                ) : (
                  <span className={cx(entry, "text-fg-muted")}>
                    <Icon aria-hidden="true" size={19} className="shrink-0" />
                    {s.label}
                    <span className="ml-auto text-meta font-semibold uppercase text-fg-faint">Soon</span>
                  </span>
                )}
              </li>
            );
          })}
        </ul>
      </nav>

      {related.length > 0 && (
        <nav aria-label="Settings on other screens" className="border-t border-line-soft pt-3">
          <h2 className="px-4 pb-1 text-meta font-bold uppercase text-fg-faint">On their own screens</h2>
          <ul className="flex flex-wrap gap-1 lg:flex-col">
            {related.map((r) => {
              const Icon = relatedIcons[r.id];
              return (
                <li key={r.id}>
                  <Link href={r.path} onClick={(e) => onGo(e, r.path)} className={cx(entry, "hover:bg-raised-high")}>
                    <Icon aria-hidden="true" size={19} className="shrink-0" />
                    {r.label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>
      )}

      <div className="rounded-card bg-raised-high p-4">
        <p className="text-sm font-bold">{versionText(version)}</p>
        <p className="mt-1 text-sm leading-relaxed text-fg-muted">Open source · AGPL-3.0 · self-hosted on your own hardware.</p>
      </div>
    </Card>
  );
}

// Where the venue is paid from a guest's phone, under its details. It reads
// and saves on its own, so a failure here leaves the venue's card usable.
function Payments({ onDirty }: { onDirty: (dirty: boolean) => void }) {
  const payments = usePayments();
  if (payments.data) return <PaymentsCard payments={payments.data} onSave={payments.save} onDirty={onDirty} />;
  if (payments.loading) return <p className="text-base text-fg-muted">Loading the payment settings</p>;
  return <EmptyState icon={<InboxIcon />} line={problemOf(payments.error).message} action={{ label: "Try again", onClick: payments.refetch }} />;
}
