// Gallery, part two: Sheet, Modal, NumPad, SearchField, Toast, EmptyState,
// DataTable, Timeline (phase 05 Task 5).

import { useState } from "react";
import { DataTable, type Column } from "../../../shared/ui/DataTable";
import { firstPage, nextPage, pageNumber, previousPage, type Sort } from "../../../shared/ui/table-state";
import {
  applyAmountKey,
  applyPinKey,
  Button,
  Confirm,
  EmptyState,
  InboxIcon,
  Modal,
  Money,
  NumPad,
  SearchField,
  Sheet,
  StatusChip,
  Timeline,
  useToast,
} from "../../../shared/ui";
import { inr, sampleEvents, sampleOrders, staffNames, type SampleOrder } from "./samples";
import { Section } from "./Section";

function Overlays() {
  const [modal, setModal] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [sheet, setSheet] = useState(false);
  const toast = useToast();

  return (
    <div className="flex flex-wrap gap-tap-gap">
      <Button onClick={() => setModal(true)}>Open modal</Button>
      <Button variant="danger" onClick={() => setConfirm(true)}>
        Void a line
      </Button>
      <Button onClick={() => setSheet(true)}>Open sheet</Button>
      <Button onClick={() => toast("Ticket sent to bar", { tone: "success" })}>Toast</Button>
      <Button onClick={() => toast("Printer is warming up", { tone: "warning" })}>Warning toast</Button>

      <Modal
        open={modal}
        onClose={() => setModal(false)}
        title="Add a note"
        footer={
          <Button variant="primary" onClick={() => setModal(false)}>
            Save note
          </Button>
        }
      >
        Escape closes this. So does the close control. A tap on the scrim does not.
      </Modal>
      <Confirm
        open={confirm}
        onClose={() => setConfirm(false)}
        onConfirm={() => {
          setConfirm(false);
          toast("Line voided");
        }}
        title="Void Avocado toast?"
        confirmLabel="Void line"
        tone="danger"
      >
        The line stays on the order, struck through, with your name on it.
      </Confirm>
      <Sheet
        open={sheet}
        onClose={() => setSheet(false)}
        title="Table 4"
        footer={
          <Button variant="primary" size="charge">
            Charge <Money amount={inr(1_48_940)} />
          </Button>
        }
      >
        <p className="text-fg-2">From the bottom on a touch screen, from the right with a mouse. Escape, the close control or a tap on the scrim closes it.</p>
        <Timeline events={sampleEvents} staffName={(id) => staffNames[id]} className="mt-4" />
      </Sheet>
    </div>
  );
}

function Pads() {
  const [pin, setPin] = useState("");
  const [minor, setMinor] = useState(0);
  return (
    <div className="flex flex-wrap gap-12">
      <div className="w-64">
        <p aria-live="polite" className="mb-3 flex h-8 items-center justify-center gap-3" aria-label={`${pin.length} digits entered`}>
          {Array.from({ length: 6 }, (_, i) => (
            <span key={i} className={i < pin.length ? "size-3 rounded-round bg-fg" : "size-3 rounded-round border border-line-strong"} />
          ))}
        </p>
        <NumPad layout="pin" label="PIN" onKey={(key) => setPin((p) => applyPinKey(p, key, 6))} />
      </div>
      <div className="w-64">
        <div className="mb-3 flex h-8 items-center justify-end">
          <Money amount={inr(minor)} className="text-xl" />
        </div>
        <NumPad layout="amount" label="Cash tendered" onKey={(key) => setMinor((m) => applyAmountKey(m, key, 99_999_99))} />
      </div>
    </div>
  );
}

const columns: Column<SampleOrder>[] = [
  { id: "id", header: "Order", cell: (o) => <span className="font-mono">{o.id}</span> },
  { id: "table", header: "Table", cell: (o) => o.table, sortable: true },
  { id: "status", header: "Status", cell: (o) => <StatusChip status={{ kind: "order", status: o.status }} /> },
  { id: "opened", header: "Opened", cell: (o) => <span className="font-mono">{o.opened}</span>, align: "right", sortable: true },
  { id: "total", header: "Total", cell: (o) => <Money amount={o.total} />, align: "right", sortable: true },
];

// A stand-in for the server: sorts and slices, and hands back a cursor.
function fetchPage(sort: Sort | null, cursor: string | null, size = 8) {
  const key = (o: SampleOrder) => (sort?.column === "total" ? o.total.minor : sort?.column === "opened" ? o.opened : o.table);
  const sorted = sort ? [...sampleOrders].sort((a, b) => (key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0) * (sort.direction === "asc" ? 1 : -1)) : sampleOrders;
  const start = cursor ? Number(cursor) : 0;
  return { rows: sorted.slice(start, start + size), next: start + size < sorted.length ? String(start + size) : null };
}

function Table() {
  const [sort, setSort] = useState<Sort | null>(null);
  const [pages, setPages] = useState(firstPage);
  const cursor = pages.stack[pages.stack.length - 1] ?? null;
  const { rows, next } = fetchPage(sort, cursor);

  return (
    <DataTable
      label="Today's orders"
      columns={columns}
      rows={rows}
      rowKey={(o) => o.id}
      sort={sort}
      onSortChange={(s) => {
        setSort(s);
        setPages(firstPage); // a new order means starting from the top
      }}
      pagination={{
        page: pageNumber(pages),
        hasPrevious: pages.stack.length > 1,
        hasNext: next !== null,
        onPrevious: () => setPages(previousPage),
        onNext: () => next && setPages((p) => nextPage(p, next)),
      }}
    />
  );
}

export function PartTwo() {
  const [query, setQuery] = useState("");
  return (
    <>
      <Section title="Modal, Sheet, Toast">
        <Overlays />
      </Section>
      <Section title="NumPad">
        <Pads />
      </Section>
      <Section title="SearchField">
        <SearchField label="Search the menu" onSearch={setQuery} className="max-w-sm" />
        <p className="mt-2 text-sm text-fg-muted">Reported: {query === "" ? "(empty)" : `"${query}"`}</p>
      </Section>
      <Section title="EmptyState">
        <EmptyState icon={<InboxIcon />} line="No open tickets. New orders appear here." action={{ label: "Start an order", onClick: () => undefined }} />
      </Section>
      <Section title="DataTable">
        <Table />
      </Section>
      <Section title="Timeline">
        <Timeline events={sampleEvents} staffName={(id) => staffNames[id]} />
      </Section>
    </>
  );
}
