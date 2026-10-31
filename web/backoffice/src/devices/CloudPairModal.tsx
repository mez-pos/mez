// Connect to mez Cloud (docs/design/screens/backoffice-devices.md): the owner
// types the code from their account, once, and the venue is paired to it
// (ADR-318, ADR-320). A code is good once, so one that could not be a code is
// refused here before it is sent, and after it is accepted the modal says
// what was done and what, if anything, is left.

import { useState } from "react";
import type { CloudPairResponse } from "../../../shared/api-types";
import { Button, Modal } from "../../../shared/ui";
import { Field, inputClass } from "../FormFields";
import type { Problem } from "../settings/problem";
import { backOfficeAddress, cleanCode, cloudProblemOf, codeProblem, pairedLines, type CloudField } from "./cloud-logic";

interface CloudPairModalProps {
  /** Exchanges a cleaned code for a pairing. */
  onPair: (code: string) => Promise<CloudPairResponse>;
  onClose: () => void;
}

export function CloudPairModal({ onPair, onClose }: CloudPairModalProps) {
  const [typed, setTyped] = useState("");
  const [problem, setProblem] = useState<Problem<CloudField> | null>(null);
  const [busy, setBusy] = useState(false);
  const [paired, setPaired] = useState<CloudPairResponse | null>(null);

  const connect = async () => {
    const refused = codeProblem(typed);
    const code = cleanCode(typed);
    if (refused || code === null) {
      setProblem(refused);
      return;
    }
    setBusy(true);
    try {
      setPaired(await onPair(code));
    } catch (err) {
      setProblem(cloudProblemOf(err));
    } finally {
      setBusy(false);
    }
  };

  if (paired) {
    return (
      <Modal
        open
        onClose={onClose}
        title="Connected to mez Cloud"
        footer={
          <Button variant="primary" onClick={onClose}>
            Done
          </Button>
        }
      >
        <div className="flex flex-col gap-3" data-testid="cloud-paired">
          {pairedLines(paired).map((line) => (
            <p key={line} className="text-base">
              {line}
            </p>
          ))}
          <p className="text-base text-fg-2">From a phone or a computer elsewhere, the back office opens at:</p>
          <p className="font-mono text-base font-semibold break-all">{backOfficeAddress(paired.hostname)}</p>
        </div>
      </Modal>
    );
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Connect to mez Cloud"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={busy} onClick={() => void connect()}>
            {busy ? "Connecting" : "Connect"}
          </Button>
        </>
      }
    >
      <form
        className="flex flex-col gap-5"
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          if (!busy) void connect();
        }}
      >
        <p className="text-base text-fg-2">
          Pairing joins this venue to your mez Cloud account: the back office opens from anywhere, guests can order from their phones, and a backup is kept away
          from this computer. The till works the same with or without it.
        </p>
        {problem && problem.field === null && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
        <Field
          label="Pairing code"
          hint={
            busy
              ? "Connecting, and starting the backup. This can take up to twenty seconds."
              : "From your mez Cloud account, under this venue. It works once, for an hour."
          }
          error={problem?.field === "code" ? problem.message : null}
        >
          {(id, aria) => (
            <input
              id={id}
              {...aria}
              className={`${inputClass} font-mono tracking-widest uppercase`}
              value={typed}
              // Eight characters, and room for the spaces or the hyphen a person types between them.
              maxLength={16}
              autoComplete="off"
              autoCapitalize="characters"
              spellCheck={false}
              autoFocus
              onChange={(e) => {
                setTyped(e.target.value);
                setProblem(null);
              }}
            />
          )}
        </Field>
      </form>
    </Modal>
  );
}
