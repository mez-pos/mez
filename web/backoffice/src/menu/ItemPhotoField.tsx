// An item's photo in its form (backoffice-menu.md, "Photos"): the one it
// has, a new one chosen and shrunk in the browser, or none. Nothing is
// uploaded until the item is saved.

import { useEffect, useId, useState } from "react";
import { photoUrl } from "../../../shared/menu";
import { Button } from "../../../shared/ui";
import { shrinkPhoto } from "./photo";

/** What Save does to the photo. */
export type PhotoChange = { kind: "keep" } | { kind: "new"; blob: Blob } | { kind: "remove" };

interface ItemPhotoFieldProps {
  /** The stored photo's name, or empty. */
  current: string;
  change: PhotoChange;
  onChange: (c: PhotoChange) => void;
  error: string | null;
}

export function ItemPhotoField({ current, change, onChange, error }: ItemPhotoFieldProps) {
  const inputId = useId();
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const preview = usePreview(change.kind === "new" ? change.blob : null);
  const shown = change.kind === "new" ? preview : change.kind === "keep" && current ? photoUrl(current) : null;

  const choose = async (file: File | undefined) => {
    if (!file) return;
    setBusy(true);
    setProblem(null);
    try {
      onChange({ kind: "new", blob: await shrinkPhoto(file) });
    } catch (err) {
      setProblem(err instanceof Error ? err.message : "That photo cannot be used.");
    } finally {
      setBusy(false);
    }
  };
  const shownProblem = problem ?? error;

  return (
    <div className="flex flex-col gap-1.5">
      <p className="text-base font-semibold">Photo</p>
      <div className="flex flex-wrap items-center gap-4">
        <div className="flex size-24 shrink-0 items-center justify-center overflow-hidden rounded border border-line bg-sunken">
          {shown ? <img src={shown} alt="" className="size-full object-cover" /> : <span className="text-sm text-fg-muted">None</span>}
        </div>
        <div className="flex flex-wrap gap-tap-gap">
          <label
            htmlFor={inputId}
            className="inline-flex min-h-tap cursor-pointer items-center rounded border border-line-strong bg-raised-high px-4 text-base font-semibold focus-within:outline-2 focus-within:outline-line-focus"
          >
            {busy ? "Preparing" : shown ? "Choose another" : "Choose a photo"}
            <input
              id={inputId}
              type="file"
              accept="image/jpeg,image/png,image/webp"
              className="sr-only"
              disabled={busy}
              onChange={(e) => {
                void choose(e.target.files?.[0]);
                e.target.value = "";
              }}
            />
          </label>
          {shown && (
            <Button variant="ghost" onClick={() => onChange(current ? { kind: "remove" } : { kind: "keep" })}>
              Remove
            </Button>
          )}
        </div>
      </div>
      <p className="text-sm text-fg-muted">Shown on the item's card on the order screen. It is made smaller here before it is sent.</p>
      {shownProblem && (
        <p role="alert" className="text-sm text-danger">
          {shownProblem}
        </p>
      )}
    </div>
  );
}

/** An object URL for a chosen photo, freed when it changes or the form closes. */
function usePreview(blob: Blob | null): string | null {
  const [url, setUrl] = useState<string | null>(null);
  useEffect(() => {
    if (!blob) {
      setUrl(null);
      return;
    }
    const next = URL.createObjectURL(blob);
    setUrl(next);
    return () => URL.revokeObjectURL(next);
  }, [blob]);
  return url;
}
