// The three forms of the Devices screen (backoffice-devices.md): pair a device
// and be shown its code, rename one, and confirm unpairing one.

import { useState } from "react";
import type { DeviceKind, DeviceRecordResponse, PairingCodeResponse, PairingPlaceResponse } from "../../../shared/api-types";
import { Button, Modal } from "../../../shared/ui";
import { useVenueClock } from "../../../shared/use-venue";
import { Choice, Field, inputClass } from "../FormFields";
import { FormModal } from "../FormModal";
import type { Problem } from "../settings/problem";
import { PAIRABLE, actionProblemOf, nameProblem, pairProblemOf, pairingPlaces, revokeWords, timeText, typedAddress, type PairField } from "./devices-logic";

/** An SVG as something an <img> can show. */
const svgSrc = (svg: string): string => `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;

/**
 * One place to open mez on the new device (ADR-367): the QR code to scan, which
 * carries the pairing code as well, beside the address to type. `named` says
 * which network it is on, for a computer on more than one. The code sits on
 * white whatever the surface: a camera reads dark on light.
 */
function PlaceBand({ place, named }: { place: PairingPlaceResponse; named: boolean }) {
  const address = typedAddress(place.url);
  return (
    <div className="flex flex-wrap items-center gap-5 rounded-card bg-sunken px-5 py-4" data-testid="pairing-place">
      {place.svg && <img src={svgSrc(place.svg)} alt={`QR code for ${address}`} width={132} height={132} className="size-33 shrink-0 rounded" />}
      <div className="min-w-0 flex-1">
        {named && place.adapter && <p className="text-meta font-bold uppercase text-fg-muted">On {place.adapter}</p>}
        <p className="break-all font-mono text-lg font-semibold tabular-nums text-fg-heading">{address}</p>
      </div>
    </div>
  );
}

interface PairModalProps {
  /** Makes the code for a name and kind. */
  onPair: (name: string, kind: DeviceKind) => Promise<PairingCodeResponse>;
  onClose: () => void;
}

export function PairModal({ onPair, onClose }: PairModalProps) {
  const { timezone } = useVenueClock();
  const [name, setName] = useState("");
  const [kind, setKind] = useState<DeviceKind>("terminal");
  const [problem, setProblem] = useState<Problem<PairField> | null>(null);
  const [busy, setBusy] = useState(false);
  const [code, setCode] = useState<PairingCodeResponse | null>(null);

  const make = async () => {
    const refused = nameProblem(name);
    if (refused) {
      setProblem(refused);
      return;
    }
    setBusy(true);
    try {
      setCode(await onPair(name.trim(), kind));
    } catch (err) {
      setProblem(pairProblemOf(err));
    } finally {
      setBusy(false);
    }
  };
  const on = (field: PairField) => (problem?.field === field ? problem.message : null);

  if (code) {
    const { main, others } = pairingPlaces(code.open);
    return (
      <Modal
        open
        onClose={onClose}
        title={`Pair ${name.trim()}`}
        size={main.length > 0 ? "md" : "sm"}
        footer={
          <Button variant="primary" onClick={onClose}>
            Done
          </Button>
        }
      >
        <div className="flex flex-col gap-5 py-2">
          {main.length > 0 ? (
            <div className="flex flex-col gap-3">
              <p className="text-base">On the new device, scan this with its camera, or open the address in its browser. Both must be on this network.</p>
              {main.map((place) => (
                <PlaceBand key={place.url} place={place} named={main.length > 1} />
              ))}
              {code.named_url && (
                <p className="text-sm text-fg-muted">
                  By name, on an iPhone, iPad, Mac or Windows PC: <span className="font-mono tabular-nums">{typedAddress(code.named_url)}</span>. The name stays
                  the same when the computer&apos;s address changes.
                </p>
              )}
              {others.length > 0 && (
                <p className="text-sm text-fg-muted">
                  Also at{" "}
                  {others.map((p, i) => (
                    <span key={p.url}>
                      {i > 0 && ", "}
                      <span className="font-mono tabular-nums">{typedAddress(p.url)}</span>
                      {p.adapter && ` (${p.adapter})`}
                    </span>
                  ))}
                  , which other devices are unlikely to reach.
                </p>
              )}
            </div>
          ) : (
            <p className="text-base">
              On the new device, open this computer&apos;s address in a browser, with <span className="font-mono tabular-nums">:7777</span> on the end. mez
              could not tell this computer&apos;s address on the network; Settings, Network on this computer shows it.
            </p>
          )}
          <div className="flex flex-col items-center gap-2 text-center">
            <p className="text-meta font-bold uppercase text-fg-muted">Pairing code</p>
            <p
              aria-label={`Pairing code ${code.code.replace("-", " ")}`}
              className="font-mono text-display tracking-widest text-fg-heading"
              data-testid="pairing-code"
            >
              {code.code}
            </p>
            <p className="text-base">
              {main.length > 0 ? "Scanned, it is typed in by itself; opened by address, type it in." : "Type it in on the pairing screen."} It works once and
              runs out at {timeText(code.expires_at, timezone)}.
            </p>
          </div>
        </div>
      </Modal>
    );
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Pair a device"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" disabled={busy} onClick={() => void make()}>
            {busy ? "Making a code" : "Make a code"}
          </Button>
        </>
      }
    >
      <form
        className="flex flex-col gap-5"
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          if (!busy) void make();
        }}
      >
        {problem && problem.field === null && (
          <p role="alert" className="text-base text-danger">
            {problem.message}
          </p>
        )}
        <Field label="Name" hint="What the device will be called: Counter 1, Pass, Office." error={on("name")}>
          {(id, aria) => (
            <input
              id={id}
              {...aria}
              className={inputClass}
              value={name}
              maxLength={64}
              autoComplete="off"
              onChange={(e) => {
                setName(e.target.value);
                setProblem(null);
              }}
            />
          )}
        </Field>
        <Choice
          label="What it is"
          value={kind}
          options={PAIRABLE}
          onChange={(k: DeviceKind) => {
            setKind(k);
            setProblem(null);
          }}
          error={on("kind")}
        />
      </form>
    </Modal>
  );
}

interface RenameModalProps {
  device: DeviceRecordResponse;
  onRename: (name: string) => Promise<void>;
  onClose: () => void;
}

export function RenameModal({ device, onRename, onClose }: RenameModalProps) {
  const [name, setName] = useState(device.name);
  const [problem, setProblem] = useState<Problem<PairField> | null>(null);
  const [busy, setBusy] = useState(false);
  const save = async () => {
    const refused = nameProblem(name);
    if (refused) {
      setProblem(refused);
      return;
    }
    setBusy(true);
    try {
      await onRename(name.trim());
      onClose();
    } catch (err) {
      setProblem(pairProblemOf(err));
      setBusy(false);
    }
  };

  return (
    <FormModal
      title={`Rename ${device.name}`}
      busy={busy}
      onSave={() => void save()}
      onClose={onClose}
      refusal={problem && problem.field !== "name" ? problem.message : null}
    >
      <Field label="Name" hint="Its label on every screen and in the history. It keeps working." error={problem?.field === "name" ? problem.message : null}>
        {(id, aria) => (
          <input
            id={id}
            {...aria}
            className={inputClass}
            value={name}
            maxLength={64}
            autoComplete="off"
            onChange={(e) => {
              setName(e.target.value);
              setProblem(null);
            }}
          />
        )}
      </Field>
    </FormModal>
  );
}

interface RevokeModalProps {
  device: DeviceRecordResponse;
  onRevoke: () => Promise<void>;
  onClose: () => void;
}

export function RevokeModal({ device, onRevoke, onClose }: RevokeModalProps) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const words = revokeWords(device);
  const confirm = async () => {
    setBusy(true);
    try {
      await onRevoke();
      onClose();
    } catch (err) {
      setError(actionProblemOf(err));
      setBusy(false);
    }
  };

  return (
    <Modal
      open
      onClose={onClose}
      title={`Revoke ${device.name}?`}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="danger" disabled={busy} onClick={() => void confirm()}>
            {busy ? "Revoking" : words.confirm}
          </Button>
        </>
      }
    >
      <p className="text-base">{words.body}</p>
      {error && (
        <p role="alert" className="mt-3 text-base text-danger">
          {error}
        </p>
      )}
    </Modal>
  );
}
