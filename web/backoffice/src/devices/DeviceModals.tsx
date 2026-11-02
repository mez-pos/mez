// The three forms of the Devices screen (backoffice-devices.md): pair a device
// and be shown its code, rename one, and confirm unpairing one.

import { useState } from "react";
import type { DeviceKind, DeviceRecordResponse, PairingCodeResponse, PairingPlaceResponse } from "../../../shared/api-types";
import { Button, Modal } from "../../../shared/ui";
import { useVenueClock } from "../../../shared/use-venue";
import { Choice, Field, inputClass } from "../FormFields";
import { FormModal } from "../FormModal";
import type { Problem } from "../settings/problem";
import {
  PAIRABLE,
  actionProblemOf,
  appOffer,
  nameProblem,
  pairProblemOf,
  pairingPlaces,
  revokeWords,
  timeText,
  typedAddress,
  type AppOffer,
  type PairField,
} from "./devices-logic";

/** An SVG as something an <img> can show. */
const svgSrc = (svg: string): string => `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;

/** A QR code with what it is for under it, numbered as a step. */
function StepCode({ svg, step, alt }: { svg: string; step: string; alt: string }) {
  return (
    <figure className="flex shrink-0 flex-col items-center gap-1.5">
      <img src={svgSrc(svg)} alt={alt} width={132} height={132} className="size-33 rounded" />
      <figcaption className="text-meta font-bold uppercase text-fg-muted">{step}</figcaption>
    </figure>
  );
}

/**
 * One place to open mez on the new device (ADR-367): the QR code to scan, which
 * carries the pairing code as well, beside the address to type. `named` says
 * which network it is on, for a computer on more than one. With `app`, the
 * tablet app's QR at this address comes first, beside it (ADR-421): a tablet
 * installs the app, then pairs. The codes sit on white whatever the surface:
 * a camera reads dark on light.
 */
function PlaceBand({ place, named, app }: { place: PairingPlaceResponse; named: boolean; app: boolean }) {
  const address = typedAddress(place.url);
  const appAddress = app && place.app_url && place.app_svg ? typedAddress(place.app_url) : null;
  return (
    <div className="flex flex-wrap items-center gap-5 rounded-card bg-sunken px-5 py-4" data-testid="pairing-place">
      {appAddress && place.app_svg && <StepCode svg={place.app_svg} step="1 · Install the app" alt={`QR code for the mez app at ${appAddress}`} />}
      {place.svg &&
        (app ? (
          <StepCode svg={place.svg} step="2 · Pair" alt={`QR code for ${address}`} />
        ) : (
          <img src={svgSrc(place.svg)} alt={`QR code for ${address}`} width={132} height={132} className="size-33 shrink-0 rounded" />
        ))}
      <div className="min-w-0 flex-1">
        {named && place.adapter && <p className="text-meta font-bold uppercase text-fg-muted">On {place.adapter}</p>}
        <p className="break-all font-mono text-lg font-semibold tabular-nums text-fg-heading">{address}</p>
        {appAddress && <p className="mt-1 break-all font-mono text-sm tabular-nums text-fg-muted">The app: {appAddress}</p>}
      </div>
    </div>
  );
}

/**
 * What the modal says of the tablet app (ADR-421) when it is not drawn as a
 * QR: where it is in words, or that this build has none.
 */
function AppWords({ offer }: { offer: AppOffer }) {
  if (offer === "typed") {
    return (
      <p className="text-sm text-fg-muted">
        On an Android tablet, install the mez app first: open this computer&apos;s address in its browser with{" "}
        <span className="font-mono tabular-nums">/app/mez.apk</span> on the end. Download it though the browser warns the file might be harmful, and allow
        installs from the browser when Android asks.
      </p>
    );
  }
  if (offer === "not_built") {
    return <p className="text-sm text-fg-muted">This mez was built without the tablet app, so a tablet pairs in its browser.</p>;
  }
  return null;
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
    const offer = appOffer(kind, code.app_version, main);
    return (
      <Modal
        open
        onClose={onClose}
        title={`Pair ${name.trim()}`}
        size={offer === "offered" ? "lg" : main.length > 0 ? "md" : "sm"}
        footer={
          <Button variant="primary" onClick={onClose}>
            Done
          </Button>
        }
      >
        <div className="flex flex-col gap-5 py-2">
          {main.length > 0 ? (
            <div className="flex flex-col gap-3">
              {offer === "offered" ? (
                <>
                  <p className="text-base">
                    On an Android tablet, first scan <strong>1</strong> with its camera to get the mez app. The browser warns the file might be harmful:
                    download it anyway, and open it. Android asks to allow installs from the browser: allow it, then install. Then open mez and scan{" "}
                    <strong>2</strong>.
                  </p>
                  <p className="text-sm text-fg-muted">
                    Any other device: scan <strong>2</strong>, or open the address in its browser. Both must be on this network.
                  </p>
                </>
              ) : (
                <p className="text-base">On the new device, scan this with its camera, or open the address in its browser. Both must be on this network.</p>
              )}
              {main.map((place) => (
                <PlaceBand key={place.url} place={place} named={main.length > 1} app={offer === "offered"} />
              ))}
              <AppWords offer={offer} />
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
          {main.length === 0 && <AppWords offer={offer} />}
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
