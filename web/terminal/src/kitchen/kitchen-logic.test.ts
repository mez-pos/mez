import { describe, expect, it } from "vitest";
import type { KitchenBoardResponse, KitchenTicketResponse } from "../../../shared/api-types";
import { agoText, allDay, boardColumns, lastBumped, latePercent, metaText, readStation, timerText, timerTone, toMake, whereText } from "./kitchen-logic";

const ticket = (extra: Partial<KitchenTicketResponse> = {}): KitchenTicketResponse => ({
  order_id: "ord_NMRNWWW1",
  ticket_seq: 3,
  station: "kitchen",
  status: "fired",
  sent_at: "2026-09-28T12:00:00Z",
  channel: "counter",
  service: "dine_in",
  covers: 2,
  notes: [],
  lines: [],
  ...extra,
});

const board = (tickets: KitchenTicketResponse[]): KitchenBoardResponse => ({
  station: "kitchen",
  tickets,
  recall_minutes: 30,
  late_after_minutes: 20,
});

describe("timers", () => {
  it("escalate at half the late time, then at it", () => {
    expect(timerTone(9 * 60_000, 20)).toBe("calm");
    expect(timerTone(10 * 60_000, 20)).toBe("waiting");
    expect(timerTone(20 * 60_000, 20)).toBe("late");
    expect(timerTone(60_000, 0)).toBe("late"); // no threshold set: a minute is late
  });
  it("read as minutes and seconds", () => {
    expect(timerText(42_000)).toBe("0:42");
    expect(timerText(425_000)).toBe("7:05");
    expect(timerText(3_729_000)).toBe("1:02:09");
    expect(timerText(-5)).toBe("0:00");
  });
});

describe("the board", () => {
  const older = ticket({ ticket_seq: 5, sent_at: "2026-09-28T11:50:00Z" });
  const bumped = ticket({ ticket_seq: 7, status: "ready", ready_at: "2026-09-28T12:10:00Z" });
  const bumpedLater = ticket({ ticket_seq: 9, status: "ready", ready_at: "2026-09-28T12:12:00Z" });

  it("makes the fired tickets, oldest first", () => {
    expect(toMake(board([ticket(), bumped, older])).map((t) => t.ticket_seq)).toEqual([5, 3]);
    expect(toMake(undefined)).toEqual([]);
    // A ticket a cook has started is still to make: it stays on the board.
    const started = ticket({ ticket_seq: 8, status: "in_progress", sent_at: "2026-09-28T12:01:00Z", started_at: "2026-09-28T12:03:00Z" });
    expect(toMake(board([started, bumped])).map((t) => t.ticket_seq)).toEqual([8]);
    expect(allDay([{ ...started, lines: [{ id: 1, name: "Upma", qty: 2, course: 1, modifiers: [], notes: [], status: "active" }] }])).toEqual([
      { name: "Upma", qty: 2 },
    ]);
  });

  it("sorts the board into new, preparing and ready", () => {
    const started = ticket({ ticket_seq: 8, status: "in_progress", sent_at: "2026-09-28T11:40:00Z", started_at: "2026-09-28T11:45:00Z" });
    const cols = boardColumns(board([ticket(), bumped, older, started, bumpedLater]));
    // To make: oldest first. On the pass: the one just called first.
    expect(cols.fresh.map((t) => t.ticket_seq)).toEqual([5, 3]);
    expect(cols.preparing.map((t) => t.ticket_seq)).toEqual([8]);
    expect(cols.ready.map((t) => t.ticket_seq)).toEqual([9, 7]);
    expect(boardColumns(undefined)).toEqual({ fresh: [], preparing: [], ready: [] });
  });

  it("says who a ticket is for and how it leaves", () => {
    expect(metaText(ticket())).toBe("2 guests · Dine-in");
    expect(metaText(ticket({ covers: 1 }))).toBe("1 guest · Dine-in");
    expect(metaText(ticket({ covers: 0, service: "takeaway" }))).toBe("Takeaway");
  });

  it("fills the bar towards late and no further", () => {
    expect(latePercent(0, 20)).toBe(0);
    expect(latePercent(5 * 60_000, 20)).toBe(25);
    expect(latePercent(45 * 60_000, 20)).toBe(100);
    expect(latePercent(-1000, 20)).toBe(0);
  });

  it("says how long a ticket has been on the pass", () => {
    expect(agoText(30_000)).toBe("now");
    expect(agoText(4 * 60_000)).toBe("4m");
    expect(agoText(65 * 60_000)).toBe("1h 05m");
  });

  it("recalls the ticket bumped last", () => {
    expect(lastBumped(board([bumped, bumpedLater, ticket()]))?.ticket_seq).toBe(9);
    expect(lastBumped(board([ticket()]))).toBeNull();
  });

  it("says where each goes", () => {
    expect(whereText(ticket({ table: "T4 + T7", table_id: 4 }))).toBe("T4 + T7");
    expect(whereText(ticket({ table_id: 4 }))).toBe("Table 4");
    expect(whereText(ticket())).toBe("Counter · NMRN");
  });
});

describe("allDay", () => {
  it("counts what is still to make, most first, voids and bumped tickets aside", () => {
    const line = (name: string, qty: number, status: "active" | "voided" | "comped" = "active") => ({
      id: 1,
      name,
      qty,
      course: 1,
      modifiers: [],
      notes: [],
      status,
    });
    const counts = allDay([
      ticket({ lines: [line("Masala dosa", 2), line("Upma", 1), line("Idli vada", 1, "voided")] }),
      ticket({ lines: [line("Masala dosa", 1), line("Upma", 1, "comped")] }),
      ticket({ status: "ready", lines: [line("Masala dosa", 5)] }),
    ]);
    expect(counts).toEqual([
      { name: "Masala dosa", qty: 3 },
      { name: "Upma", qty: 2 },
    ]);
  });
});

describe("readStation", () => {
  it("keeps a station code, and nothing that is not one", () => {
    const store = (v: string | null) => ({ getItem: () => v });
    expect(readStation(store("kitchen"))).toBe("kitchen");
    expect(readStation(store("<script>"))).toBeNull();
    expect(readStation(null)).toBeNull();
    expect(
      readStation({
        getItem: () => {
          throw new Error("blocked");
        },
      }),
    ).toBeNull();
  });
});
