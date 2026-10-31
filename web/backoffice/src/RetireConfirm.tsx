// The one way a retire is asked (ADR-387): the words from retire-logic.ts in
// a Confirm whose button is the danger one, or, when it cannot be retired
// yet, the reason and Close.

import { Button, Confirm, Modal } from "../../shared/ui";
import type { RetireWords } from "./retire-logic";

interface RetireConfirmProps {
  /** Null while nothing is being asked. */
  words: RetireWords | null;
  onClose: () => void;
  onRetire: () => void;
}

export function RetireConfirm({ words, onClose, onRetire }: RetireConfirmProps) {
  if (words?.confirm === null) {
    return (
      <Modal open onClose={onClose} title={words.title} footer={<Button onClick={onClose}>Close</Button>}>
        <p className="text-base">{words.body}</p>
      </Modal>
    );
  }
  return (
    <Confirm
      open={words !== null}
      onClose={onClose}
      onConfirm={() => {
        onClose();
        onRetire();
      }}
      title={words?.title ?? ""}
      confirmLabel={words?.confirm ?? ""}
      tone="danger"
    >
      <p className="text-base">{words?.body}</p>
    </Confirm>
  );
}
