import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { KitchenBoardResponse, KitchenTicketResponse } from "../../../shared/api-types";
import {
  agoText,
  allDay,
  boardColumns,
  comingRows,
  isNewTicket,
  latePercent,
  metaText,
  minutesText,
  moveFailure,
  readFailure,
  readStation,
  staleText,
  statRows,
  tapHeld,
  timerText,
  timerTone,
  toMake,
  whereParts,
  whereText,
} from "./kitchen-logic";

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
  stats: { sent: 0, in_kitchen: 0, ready: 0, average_seconds: 0 },
  coming: [],
  coming_more: 0,
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

  it("says where each goes", () => {
    expect(whereText(ticket({ table: "T4 + T7", table_id: 4 }))).toBe("T4 + T7");
    expect(whereText(ticket({ table_id: 4 }))).toBe("Table 4");
    expect(whereText(ticket())).toBe("Counter · NMRN");
  });

  it("puts a counter order's short id in the big type and a table whole", () => {
    const cases: readonly { name: string; t: KitchenTicketResponse; lead: string | null; main: string }[] = [
      { name: "a table by name", t: ticket({ table: "T4", table_id: 4 }), lead: null, main: "T4" },
      { name: "tables joined", t: ticket({ table: "T4 + T7", table_id: 4 }), lead: null, main: "T4 + T7" },
      { name: "a table the menu has no name for", t: ticket({ table_id: 9 }), lead: null, main: "Table 9" },
      { name: "a counter order", t: ticket(), lead: "Counter", main: "NMRN" },
      { name: "a counter order with a short id", t: ticket({ order_id: "ord_AB" }), lead: "Counter", main: "AB" },
    ];
    for (const c of cases) {
      expect(whereParts(c.t), c.name).toEqual({ lead: c.lead, main: c.main });
      expect(whereText(c.t), c.name).toBe(c.lead ? `${c.lead} · ${c.main}` : c.main);
    }
  });
});

describe("today's stats", () => {
  it("writes a time to the nearest minute", () => {
    expect(minutesText(0)).toBe("under 1 min");
    expect(minutesText(59)).toBe("under 1 min");
    expect(minutesText(60)).toBe("1 min");
    expect(minutesText(11 * 60 + 29)).toBe("11 min");
    expect(minutesText(11 * 60 + 30)).toBe("12 min");
  });
  it("lists the day in the design's order, with the slowest by its dish", () => {
    const rows = statRows({ sent: 28, in_kitchen: 3, ready: 22, average_seconds: 660, slowest: { name: "Biryani", more: 0, seconds: 19 * 60 } });
    expect(rows.map((r) => [r.label, r.value])).toEqual([
      ["Total tickets", "28"],
      ["In progress", "3"],
      ["Completed", "22"],
      ["Avg. prep time", "11 min"],
      ["Slowest today", "Biryani · 19 min"],
    ]);
    expect(rows.filter((r) => r.timing).map((r) => r.label)).toEqual(["Avg. prep time", "Slowest today"]);
  });
  it("counts the other dishes on the slowest ticket", () => {
    const rows = statRows({ sent: 1, in_kitchen: 0, ready: 1, average_seconds: 420, slowest: { name: "Masala dosa", more: 2, seconds: 420 } });
    expect(rows.at(-1)?.value).toBe("Masala dosa +2 · 7 min");
  });
  it("has no times to give until a ticket is ready", () => {
    const rows = statRows({ sent: 2, in_kitchen: 2, ready: 0, average_seconds: 0 });
    expect(rows.slice(3).map((r) => r.value)).toEqual(["Not yet", "Not yet"]);
  });
});

describe("coming next", () => {
  const names = (course: number) => (course === 2 ? "Mains" : null);
  it("says where, which course and what is on it", () => {
    const rows = comingRows(
      [
        {
          order_id: "ord_NMRNWWW1",
          service: "dine_in",
          table_id: 4,
          table: "T4",
          covers: 2,
          course: 2,
          lines: [
            { name: "Butter chicken", qty: 1 },
            { name: "Dal", qty: 2 },
          ],
        },
        { order_id: "ord_ZZZZ2222", service: "takeaway", covers: 0, course: 3, lines: [{ name: "Kulfi", qty: 1 }] },
      ],
      names,
    );
    expect(rows).toEqual([
      { id: "ord_NMRNWWW1-2", where: "T4", course: "Mains", items: "1 × Butter chicken, 2 × Dal" },
      { id: "ord_ZZZZ2222-3", where: "Counter · ZZZZ", course: "Course 3", items: "1 × Kulfi" },
    ]);
  });
  it("is empty when nothing is held back", () => {
    expect(comingRows([], names)).toEqual([]);
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

describe("a board to trust (ADR-391)", () => {
  it("says it may be out of date once the server has been silent 10 seconds (KDS-2)", () => {
    expect(staleText(null)).toBeNull();
    expect(staleText(9_999)).toBeNull();
    expect(staleText(10_000)).toBe("Not up to date: no word from the server for 10 seconds. New tickets may be missing.");
    expect(staleText(119_000)).toMatch(/for 119 seconds\./);
    expect(staleText(185_000)).toMatch(/for 3 minutes\./);
  });

  it("takes no tap for 700 ms after the board moves, so a double tap does not land on the next card (KDS-3)", () => {
    expect(tapHeld(1_000, 1_699)).toBe(true);
    expect(tapHeld(1_000, 1_700)).toBe(false);
  });

  it("says New for a minute after a ticket arrives, and never for one there at the first read (KDS-1)", () => {
    expect(isNewTicket(undefined, 5_000)).toBe(false);
    expect(isNewTicket(1_000, 60_999)).toBe(true);
    expect(isNewTicket(1_000, 61_000)).toBe(false);
  });

  it("says a failed move in the kitchen's words, never the till's (KDS-14)", () => {
    expect(moveFailure(new ApiError("network", "offline"), "T4")).toBe("T4 did not move: no answer from the server. Tap again.");
    expect(moveFailure(new ApiError("invalid_transition", "x", 409), "T4")).toBe("T4 has already moved on another screen.");
    expect(moveFailure(new ApiError("ticket_not_found", "x", 404), "Counter · NMRN")).toBe("Counter · NMRN is no longer on the board.");
    expect(moveFailure(new ApiError("forbidden", "x", 403), "T4")).toMatch(/^T4 did not move: the server refused it\./);
    expect(moveFailure(new Error("boom"), "T4")).toBe("T4 did not move. Tap again.");
    for (const err of [new ApiError("network", "x"), new Error("x")]) expect(moveFailure(err, "T4")).not.toMatch(/Not sent/);
  });

  it("says the board could not be read, and that it is trying again", () => {
    expect(readFailure(new ApiError("timeout", "x"))).toBe("The tickets could not be read: no answer from the server. Trying again.");
    expect(readFailure(new ApiError("forbidden", "x", 403))).toBe("The tickets could not be read. Trying again.");
  });
});
