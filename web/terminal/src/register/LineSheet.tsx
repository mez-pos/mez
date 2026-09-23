// What a line can still have done to it (Task 4), from a tap on it in the
// ticket: its quantity and a note, saved together, and the way into modify,
// discount, comp and void. What the kitchen has already seen cannot change
// quantity; it is voided and rung up again (ADR-072).

import { useState } from "react";
import type { LineResponse, NoteResponse } from "../../../shared/api-types";
import { Button, Money, Qty, Sheet } from "../../../shared/ui";
import { lineActions } from "./cart-logic";

export type LineAction = "modify" | "discount" | "comp" | "void";

interface LineSheetProps {
  line: LineResponse;
  notes: readonly NoteResponse[];
  /** Whether its item has choices to modify. */
  hasChoices: boolean;
  onSave: (change: { qty: number | null; note: string | null }) => Promise<void>;
  onAction: (action: LineAction) => void;
  onClose: () => void;
}

export function LineSheet({ line, notes, hasChoices, onSave, onAction, onClose }: LineSheetProps) {
  const can = lineActions(line);
  const [qty, setQty] = useState(line.qty);
  const [note, setNote] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const trimmed = note.trim();
  const dirty = qty !== line.qty || trimmed !== "";

  const save = async () => {
    setBusy(true);
    setError(null);
    try {
      await onSave({ qty: qty !== line.qty ? qty : null, note: trimmed || null });
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
      <Button variant={dirty ? "primary" : "secondary"} size="charge" className="w-full" disabled={busy} onClick={dirty ? () => void save() : onClose}>
        {dirty ? "Save" : "Done"}
      </Button>
    </div>
  );

  return (
    <Sheet open onClose={onClose} title={line.name} footer={footer}>
      <div className="flex flex-col gap-6">
        <div className="flex items-center justify-between gap-4">
          {can.change ? (
            <Qty value={qty} onChange={setQty} label={line.name} />
          ) : (
            <p className="text-base text-fg-2">
              <span className="font-mono tabular-nums">{line.qty}</span> ×{" "}
              {line.status === "active" ? "with the kitchen: void to change" : line.status}
            </p>
          )}
          <Money amount={line.gross} />
        </div>

        {notes.length > 0 && (
          <ul className="flex flex-col gap-1 text-sm text-fg-2">
            {notes.map((n) => (
              <li key={n.seq}>Note: {n.text}</li>
            ))}
          </ul>
        )}
        <label className="flex flex-col gap-2">
          <span className="text-sm text-fg-muted">Add a note for the kitchen</span>
          <textarea
            value={note}
            onChange={(e) => setNote(e.target.value)}
            maxLength={200}
            rows={2}
            // Controls are 16px on every surface (tokens.css), so no size here.
            className="rounded border border-line-strong bg-raised-high px-3 py-2 text-fg outline-offset-2 outline-line-focus focus-visible:outline-2"
          />
        </label>

        <div className="grid grid-cols-2 gap-tap-gap">
          {can.change && hasChoices && <Button onClick={() => onAction("modify")}>Modify</Button>}
          {can.discount && <Button onClick={() => onAction("discount")}>Discount</Button>}
          {can.comp && <Button onClick={() => onAction("comp")}>Comp</Button>}
          {can.void && (
            <Button variant="danger" onClick={() => onAction("void")}>
              Void
            </Button>
          )}
        </div>
      </div>
    </Sheet>
  );
}
