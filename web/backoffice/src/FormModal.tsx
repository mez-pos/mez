// A modal holding one form: Cancel and Save in the footer, Enter saves, and a
// refusal that belongs to no one field above the fields. The back office's
// form modals are built on it, so they all save and refuse alike.

import type { ReactNode } from "react";
import { Button, Modal } from "../../shared/ui";

export interface FormModalProps {
  title: string;
  size?: "sm" | "md";
  busy: boolean;
  /** Checks and saves the form. Not called again while busy. */
  onSave: () => void;
  onClose: () => void;
  /** The refusal that belongs to no one field, shown above them all. */
  refusal?: string | null;
  children: ReactNode;
}

export function FormModal({ title, size = "sm", busy, onSave, onClose, refusal, children }: FormModalProps) {
  return (
    <Modal
      open
      onClose={onClose}
      title={title}
      size={size}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={busy} onClick={onSave}>
            {busy ? "Saving" : "Save"}
          </Button>
        </>
      }
    >
      {/* The server checks every field and says which; the browser's own
          bubbles would say it differently, in the browser's language. */}
      <form
        className="flex flex-col gap-5"
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          if (!busy) onSave();
        }}
      >
        {refusal && (
          <p role="alert" className="text-base text-danger">
            {refusal}
          </p>
        )}
        {children}
        {/* Save sits in the footer, outside the form. Without a submit
            button of its own, a form of two or more fields ignores Enter. */}
        <button type="submit" hidden />
      </form>
    </Modal>
  );
}
