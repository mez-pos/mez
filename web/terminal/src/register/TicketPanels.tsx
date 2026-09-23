// The cart's sheets (Task 4), one open at a time: a line's own sheet, and
// from it void, comp and discount; a discount or note on the whole order.
// Each reads its line from the live order, so a sheet never acts on a copy
// the stream has since replaced.

import type { LineResponse, OrderResponse } from "../../../shared/api-types";
import { DiscountSheet } from "./DiscountSheet";
import { LineSheet, type LineAction } from "./LineSheet";
import { NoteSheet } from "./NoteSheet";
import type { Ticket } from "./use-ticket";
import { VoidCompSheet } from "./VoidCompSheet";

export type Panel =
  | { readonly kind: "line"; readonly lineId: number }
  | { readonly kind: "void" | "comp"; readonly lineId: number }
  | { readonly kind: "discount"; readonly lineId: number | null }
  | { readonly kind: "note" };

interface TicketPanelsProps {
  panel: Panel | null;
  onPanel: (next: Panel | null) => void;
  ticket: Ticket;
  /** Whether a line's item has choices, so it can be modified. */
  hasChoices: (line: LineResponse) => boolean;
  /** Opens the modifier sheet on a line. */
  onModify: (line: LineResponse) => void;
}

export function TicketPanels({ panel, onPanel, ticket, hasChoices, onModify }: TicketPanelsProps) {
  const order = ticket.order;
  if (!panel || !order) return null;
  const close = () => onPanel(null);
  const line = "lineId" in panel && panel.lineId !== null ? order.lines.find((l) => l.id === panel.lineId) : undefined;

  switch (panel.kind) {
    case "line": {
      if (!line) return null;
      const act = (action: LineAction) => {
        if (action === "modify") {
          onPanel(null);
          onModify(line);
        } else if (action === "discount") onPanel({ kind: "discount", lineId: line.id });
        else onPanel({ kind: action, lineId: line.id });
      };
      return (
        <LineSheet
          key={line.id}
          line={line}
          notes={order.notes.filter((n) => n.scope === "line" && n.line_id === line.id)}
          hasChoices={hasChoices(line)}
          onSave={async ({ qty, note }) => {
            if (qty !== null) await ticket.changeQty(line.id, line.qty, qty);
            if (note !== null) await ticket.addNote({ scope: "line", line_id: line.id, text: note });
          }}
          onAction={act}
          onClose={close}
        />
      );
    }
    case "void":
    case "comp": {
      if (!line) return null;
      const send = panel.kind === "void" ? ticket.voidLine : ticket.compLine;
      return (
        <VoidCompSheet
          key={`${panel.kind}-${line.id}`}
          kind={panel.kind}
          line={line}
          onSubmit={(reason, approval) => send(line.id, reason, approval)}
          onClose={close}
        />
      );
    }
    case "discount": {
      if (panel.lineId !== null && !line) return null;
      return (
        <DiscountSheet
          key={line ? `discount-${line.id}` : "discount-order"}
          target={line ? { lineId: line.id, name: line.name } : "order"}
          worth={line ? lineWorth(line) : order.totals.subtotal}
          onSubmit={ticket.discount}
          onClose={close}
        />
      );
    }
    case "note":
      return <NoteSheet onSave={(text) => ticket.addNote({ scope: "order", text })} onClose={close} />;
  }
}

/** What a line discount can still come off: its value after its own discounts. */
function lineWorth(line: LineResponse): OrderResponse["totals"]["gross"] {
  return { ...line.gross, minor: line.gross.minor - line.discount.minor };
}
