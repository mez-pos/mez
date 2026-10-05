// Seating a party from the waiter's home (ADR-171): how many guests, then "Seat
// and order", which goes on to the menu with the table's order in front, or
// plain "Seat", which leaves the order for later. The same two choices as the
// floor's panel, with the waiter's usual one in green.

import { ArrowRightIcon } from "lucide-react";
import { useState } from "react";
import type { LiveTableResponse } from "../../../shared/api-types";
import { Button, Modal, Qty } from "../../../shared/ui";
import { startingCovers } from "../floor/floor-logic";
import { wordsOf } from "../register/order-messages";

interface SeatTableProps {
  table: LiveTableResponse;
  /** Seats the party. With `thenOrder` it goes on to the menu. Rejects with the server's refusal. */
  onSeat: (covers: number, thenOrder: boolean) => Promise<void>;
  onClose: () => void;
}

export function SeatTable({ table, onSeat, onClose }: SeatTableProps) {
  const [covers, setCovers] = useState(() => startingCovers(table.seats));
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);

  const seat = (thenOrder: boolean) => {
    setBusy(true);
    setFailure(null);
    onSeat(covers, thenOrder).catch((err: unknown) => {
      setFailure(wordsOf(err));
      setBusy(false);
    });
  };

  return (
    <Modal
      open
      onClose={onClose}
      title={`Seat Table ${table.name}`}
      footer={
        <div className="flex w-full flex-col gap-3">
          <Button variant="primary" size="charge" className="w-full" disabled={busy} onClick={() => seat(true)}>
            Seat and order
            <ArrowRightIcon aria-hidden="true" size={18} />
          </Button>
          <Button className="w-full" disabled={busy} onClick={() => seat(false)}>
            Seat {covers} {covers === 1 ? "guest" : "guests"}
          </Button>
        </div>
      }
    >
      <p>This table is free. It seats {table.seats}.</p>
      <div className="mt-5 flex items-center justify-between gap-4">
        <span className="text-base font-semibold text-fg">Guests</span>
        <Qty value={covers} onChange={setCovers} label="Guests" bounds={{ min: 1, max: 99 }} />
      </div>
      {failure && (
        <p role="alert" className="mt-4 text-base text-danger">
          {failure}
        </p>
      )}
    </Modal>
  );
}
