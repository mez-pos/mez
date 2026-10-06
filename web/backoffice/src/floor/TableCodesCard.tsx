// Ordering from the table (backoffice-floor.md, ADR-244): each table's code,
// the address its sticker says, and renew, turn off and show. A code is a
// guest's only credential, so renewing and turning off each ask first: both
// stop a sticker that may be on a table now.

import { useEffect, useState } from "react";
import type { PickupCodeResponse, TableCodeResponse } from "../../../shared/api-types";
import { Button, Card, Confirm, Modal, useToast } from "../../../shared/ui";
import { useVenue } from "../../../shared/use-venue";
import { CodeSheet } from "./CodeSheet";
import { RowMenu } from "../RowMenu";
import {
  actionLabels,
  actionsFor,
  canPrint,
  codeText,
  codesProblem,
  codesSummary,
  confirmWords,
  missingCount,
  noAddressWords,
  pickupConfirmWords,
  pickupOffWords,
  pickupState,
  printableCount,
  type CodeAction,
  type ConfirmWords,
} from "./codes-logic";
import { useTableCodes } from "./use-table-codes";

// table is null for the venue's pickup code, which is no table's.
type Asking = { action: "renew" | "revoke"; table: TableCodeResponse | null } | { action: "show"; table: TableCodeResponse | null };

const head = "pb-3 text-meta font-bold uppercase text-fg-muted";

export function TableCodesCard() {
  const api = useTableCodes();
  const toast = useToast();
  const [asking, setAsking] = useState<Asking | null>(null);
  const [printing, setPrinting] = useState(false);
  const [busy, setBusy] = useState(false);
  const venue = useVenue();
  const data = api.data;

  const run = (fn: () => Promise<void>, done: string) => {
    setBusy(true);
    fn()
      .then(() => toast(done, { tone: "success" }))
      .catch((err: unknown) => toast(codesProblem(err), { tone: "warning" }))
      .finally(() => setBusy(false));
  };
  const act = (action: CodeAction, table: TableCodeResponse) => {
    // Giving a code to a table with none stops nothing, so it asks nothing.
    if (action === "give") run(() => api.renew(table.table_id), `${table.table_name} has a code.`);
    else setAsking({ action, table });
  };
  const actPickup = (action: CodeAction) => {
    // Making the first code stops nothing, so it asks nothing.
    if (action === "give") run(api.renewPickup, "Ordering to collect has a code.");
    else setAsking({ action, table: null });
  };
  // The door's card is on the sheet only where orders to collect are taken.
  const doorPrints = data !== undefined && pickupState(data.pickup) === "on";

  return (
    <Card padding="lg" className="flex min-w-0 flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 className="font-serif text-xl">Ordering from the table</h2>
          <p className="mt-1 text-base text-fg-2">
            Each table has a code. A guest scans it to see the menu and order to that table. Anyone with a table's code can order to it.
          </p>
          {data && <p className="mt-1 text-base text-fg-muted">{codesSummary(data)}</p>}
        </div>
        <div className="flex flex-wrap gap-tap-gap">
          {data && missingCount(data) > 0 && (
            <Button disabled={busy} onClick={() => run(api.makeMissing, "Every table has a code.")}>
              Make the missing codes
            </Button>
          )}
          {/* Nothing to print without the venue's public address, or with no code. */}
          {data && canPrint(data) && (printableCount(data) > 0 || doorPrints) && (
            <Button disabled={busy} onClick={() => setPrinting(true)}>
              Print the codes
            </Button>
          )}
        </div>
      </div>

      {api.loading && <p className="text-base text-fg-muted">Reading the codes</p>}
      {!api.loading && api.error !== null && data === undefined && (
        <div className="flex flex-wrap items-center gap-tap-gap">
          <p role="alert" className="text-base text-danger">
            {codesProblem(api.error)}
          </p>
          <Button onClick={api.refetch}>Try again</Button>
        </div>
      )}

      {data && !canPrint(data) && (
        <p role="status" className="rounded border border-chip-warning-border bg-chip-warning-bg p-3 text-base text-chip-warning-fg">
          {noAddressWords}
        </p>
      )}

      {data && data.tables.length > 0 && (
        <div className="min-h-64 overflow-x-auto xl:min-h-0 xl:overflow-visible">
          <table aria-label="Table codes" className="w-full min-w-160 text-base">
            <thead>
              <tr className="border-b border-line text-left">
                <th scope="col" className={head}>
                  Table
                </th>
                <th scope="col" className={head}>
                  Code
                </th>
                <th scope="col" className={head}>
                  Address on the sticker
                </th>
                <th scope="col" className={head}>
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {data.tables.map((t, row) => (
                <tr key={t.table_id} className="border-b border-line-soft last:border-b-0">
                  <td className="py-2 pr-4 font-semibold">{t.table_name}</td>
                  <td className="whitespace-nowrap py-2 pr-4">
                    {t.code === "" ? <span className="text-fg-muted">No code: no orders by phone</span> : <span className="font-mono">{codeText(t.code)}</span>}
                  </td>
                  <td className="py-2 pr-4 text-sm text-fg-2">{t.url === "" ? "" : <span className="font-mono break-all">{t.url}</span>}</td>
                  <td className="h-14 py-2">
                    <span className="flex justify-end">
                      <RowMenu
                        name={t.table_name}
                        disabled={busy}
                        up={row >= 3 && row >= data.tables.length - 2}
                        actions={actionsFor(t, canPrint(data)).map((a) => ({ label: actionLabels[a], run: () => act(a, t) }))}
                      />
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {data && <PickupCode pickup={data.pickup} printable={canPrint(data)} busy={busy} onAct={actPickup} />}

      {asking && asking.action !== "show" && (
        <ConfirmCode
          words={asking.table === null ? pickupConfirmWords(asking.action) : confirmWords(asking.action, asking.table.table_name)}
          onClose={() => setAsking(null)}
          onConfirm={() => {
            const { action, table } = asking;
            setAsking(null);
            if (table === null) run(action === "renew" ? api.renewPickup : api.revokePickup, pickupConfirmWords(action).done);
            else run(() => (action === "renew" ? api.renew(table.table_id) : api.revoke(table.table_id)), confirmWords(action, table.table_name).done);
          }}
        />
      )}
      {asking?.action === "show" && asking.table !== null && (
        <QRModal
          key={asking.table.table_id}
          title={`The code for ${asking.table.table_name}`}
          alt={`QR code for ordering at ${asking.table.table_name}`}
          line={`Scan it with a phone to check it opens ${asking.table.table_name}'s menu.`}
          read={(
            (id) => () =>
              api.qr(id)
          )(asking.table.table_id)}
          onClose={() => setAsking(null)}
        />
      )}
      {asking?.action === "show" && asking.table === null && (
        <QRModal
          title="The code for ordering to collect"
          alt="QR code for ordering to collect"
          line="Scan it with a phone to check it opens the menu with no table."
          read={api.pickupQr}
          onClose={() => setAsking(null)}
        />
      )}
      {printing && <CodeSheet venueName={venue?.name ?? ""} read={api.sheet} onClose={() => setPrinting(false)} />}
    </Card>
  );
}

/**
 * Ordering to collect (ADR-252): the venue's one pickup code, for a poster
 * on the door or a card on the counter. It leads to the venue and no table.
 */
function PickupCode({
  pickup,
  printable,
  busy,
  onAct,
}: {
  pickup: PickupCodeResponse;
  printable: boolean;
  busy: boolean;
  onAct: (action: CodeAction) => void;
}) {
  const state = pickupState(pickup);
  return (
    <section aria-labelledby="pickup-code-title" className="flex flex-col gap-3 border-t border-line pt-4">
      <div>
        <h3 id="pickup-code-title" className="font-serif text-lg">
          Ordering to collect
        </h3>
        <p className="mt-1 text-base text-fg-2">
          One code for the venue, for the door or the counter. A guest scans it, orders and pays on their phone, and collects at the counter when the code they
          are given is called. Anyone who has it can order: nothing is made until it is paid for.
        </p>
      </div>
      {state === "no_orders" && (
        <p role="status" className="rounded border border-chip-warning-border bg-chip-warning-bg p-3 text-base text-chip-warning-fg">
          {pickupOffWords}
        </p>
      )}
      <div className="flex flex-wrap items-center justify-between gap-tap-gap">
        {state === "none" ? (
          <p className="text-base text-fg-muted">No code: nobody can order to collect by phone.</p>
        ) : (
          <div className="min-w-0">
            <p className="font-mono text-base">{codeText(pickup.code)}</p>
            {pickup.url !== "" && <p className="mt-1 font-mono text-sm break-all text-fg-2">{pickup.url}</p>}
          </div>
        )}
        <div className="flex flex-wrap gap-tap-gap">
          {actionsFor(pickup, printable).map((a) => (
            <Button key={a} disabled={busy} onClick={() => onAct(a)}>
              {a === "give" ? "Make a code" : actionLabels[a]}
            </Button>
          ))}
        </div>
      </div>
    </section>
  );
}

function ConfirmCode({ words, onClose, onConfirm }: { words: ConfirmWords; onClose: () => void; onConfirm: () => void }) {
  return (
    <Confirm open onClose={onClose} onConfirm={onConfirm} title={words.title} confirmLabel={words.confirm} tone="danger">
      <p className="text-base">{words.body}</p>
    </Confirm>
  );
}

/**
 * A code as a QR, a table's or the venue's pickup code, to check a sticker
 * against or to scan from the screen. The picture is the server's SVG shown
 * as an image, never put in the page as markup.
 */
function QRModal({
  title,
  alt,
  line,
  read,
  onClose,
}: {
  title: string;
  alt: string;
  line: string;
  read: () => Promise<{ url: string; svg: string }>;
  onClose: () => void;
}) {
  const [qr, setQr] = useState<{ url: string; svg: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let gone = false;
    read()
      .then((got) => !gone && setQr(got))
      .catch((err: unknown) => !gone && setError(codesProblem(err)));
    return () => {
      gone = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- read once for the code it was opened on
  }, []);

  return (
    <Modal open onClose={onClose} title={title} footer={<Button onClick={onClose}>Close</Button>}>
      {qr === null && error === null && <p className="text-base text-fg-muted">Drawing the code</p>}
      {error && (
        <p role="alert" className="text-base text-danger">
          {error}
        </p>
      )}
      {qr && (
        <div className="flex flex-col items-center gap-3 text-center">
          <img src={`data:image/svg+xml;charset=utf-8,${encodeURIComponent(qr.svg)}`} alt={alt} width={240} height={240} className="size-60 rounded bg-white" />
          <p className="font-mono text-sm break-all text-fg-2">{qr.url}</p>
          <p className="text-base text-fg-2">{line}</p>
        </div>
      )}
    </Modal>
  );
}
