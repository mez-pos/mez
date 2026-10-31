// A note on the whole order: "birthday, bring the cake out with a candle".
// A note on one line is written from its LineSheet.

import { useState } from "react";
import { Button, Sheet } from "../../../shared/ui";
import { fieldClass } from "../../../shared/ui/field";
import { cx } from "../../../shared/ui/cx";

interface NoteSheetProps {
  onSave: (text: string) => Promise<void>;
  onClose: () => void;
}

export function NoteSheet({ onSave, onClose }: NoteSheetProps) {
  const [text, setText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const trimmed = text.trim();

  const save = async () => {
    setBusy(true);
    setError(null);
    try {
      await onSave(trimmed);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  };

  const footer = (
    <div className="flex flex-col gap-2">
      <p className="min-h-6 text-sm text-danger" role="status">
        {error}
      </p>
      <Button variant="primary" size="charge" className="w-full" disabled={trimmed === "" || busy} onClick={() => void save()}>
        Add note
      </Button>
    </div>
  );

  return (
    <Sheet open onClose={onClose} title="Note on the order" footer={footer} holdsForm={text !== ""}>
      <label className="flex flex-col gap-2">
        <span className="text-sm text-fg-muted">Everyone working this order sees it</span>
        <textarea value={text} onChange={(e) => setText(e.target.value)} maxLength={200} rows={3} className={cx(fieldClass, "px-4 py-3")} />
      </label>
    </Sheet>
  );
}
