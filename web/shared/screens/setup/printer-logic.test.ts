import { describe, expect, it } from "vitest";
import { ApiError, type ApiClient } from "../../api";
import type { DiscoverResponse, PrinterResponse, PrintersResponse, PrintJobState } from "../../api-types";
import {
  addAndTest,
  addressProblem,
  choiceKey,
  choicesOf,
  forget,
  freeName,
  outcomeOf,
  printerError,
  searchedWords,
  typedChoice,
  watchTest,
  type PrinterChoice,
} from "./printer-logic";

const found: DiscoverResponse = {
  usb: [{ address: "EPSON TM-T82", known: false }],
  network: [
    { address: "192.168.1.50", known: true },
    { address: "192.168.1.60", known: false },
  ],
  searched: ["192.168.1.0/24"],
};

const printer = (over: Partial<PrinterResponse> = {}): PrinterResponse => ({
  id: 1,
  name: "Receipt printer",
  transport: "tcp",
  address: "192.168.1.60",
  paper_mm: 80,
  active: true,
  is_receipt: false,
  stations: [],
  ...over,
});

const withJob = (id: number, state: PrintJobState, error?: string): PrinterResponse =>
  printer({ last_job: { id, state, label: "Test page", at: "2026-10-07T04:30:00Z", ...(error ? { error } : {}) } });

/** A server holding a list of printers, that records what it is asked. */
function server(start: PrinterResponse[] = [], fail: Record<string, unknown> = {}) {
  let printers = [...start];
  const calls: string[] = [];
  const answer = (): PrintersResponse => ({ printers, stations: [] });
  const handle = async (method: string, path: string, body?: unknown): Promise<unknown> => {
    const call = `${method} ${path}`;
    if (call in fail) throw fail[call];
    calls.push(body === undefined ? call : `${call} ${JSON.stringify(body)}`);
    if (call === "POST /api/v1/printers") {
      const s = body as { name: string; transport: "tcp" | "windows"; address: string; paper_mm: number };
      printers = [...printers, printer({ id: printers.length + 1, name: s.name, transport: s.transport, address: s.address, paper_mm: s.paper_mm })];
    }
    const retire = /^POST \/api\/v1\/printers\/(\d+)\/(retire|restore)$/.exec(call);
    if (retire) printers = printers.map((p) => (p.id === Number(retire[1]) ? { ...p, active: retire[2] === "restore" } : p));
    if (/\/test$/.test(path)) return { job_id: 77 };
    return answer();
  };
  const api: ApiClient = {
    get: <T>(path: string) => handle("GET", path) as Promise<T>,
    post: <T>(path: string, body?: unknown) => handle("POST", path, body) as Promise<T>,
    put: <T>(path: string, body?: unknown) => handle("PUT", path, body) as Promise<T>,
    del: <T>(path: string) => handle("DELETE", path) as Promise<T>,
  };
  return { api, calls, list: () => printers };
}

describe("choicesOf", () => {
  it("lists this computer's printers first, then the network's, with what the venue already has", () => {
    const choices = choicesOf(found);
    expect(choices.map((c) => [c.transport, c.address, c.known])).toEqual([
      ["windows", "EPSON TM-T82", false],
      ["tcp", "192.168.1.50", true],
      ["tcp", "192.168.1.60", false],
    ]);
    expect(choices[0]?.label).toBe("EPSON TM-T82");
    expect(choices[1]?.label).toBe("Network printer at 192.168.1.50");
  });

  it("gives each choice a key that a second search finds again", () => {
    expect(choiceKey(typedChoice(" 192.168.1.60:9100 "))).toBe(choiceKey(choicesOf(found)[2] as PrinterChoice));
    expect(choiceKey({ transport: "windows", address: "epson tm-t82" })).toBe(choiceKey(choicesOf(found)[0] as PrinterChoice));
    expect(choiceKey({ transport: "tcp", address: "192.168.1.60:9101" })).not.toBe(choiceKey(choicesOf(found)[2] as PrinterChoice));
  });

  it("says what was searched, or that nothing could be", () => {
    expect(searchedWords(found)).toBe("Searched 192.168.1.0/24 and this computer's own printers.");
    expect(searchedWords({ ...found, searched: ["192.168.1.0/24", "10.0.0.0/24"] })).toContain("192.168.1.0/24 and 10.0.0.0/24");
    expect(searchedWords({ ...found, searched: [] })).toContain("not on a network");
  });
});

describe("addressProblem", () => {
  it.each([
    ["192.168.1.50", false],
    [" 192.168.1.50:9100 ", false],
    ["", true],
    ["printer.local", true],
    ["192.168.1", true],
    ["192.168.1.50:", true],
    ["http://192.168.1.50", true],
  ])("%s", (address, wrong) => {
    expect(addressProblem(address) !== null).toBe(wrong);
  });
});

describe("freeName", () => {
  it("is the plain name until that is taken, by a live printer or a retired one", () => {
    expect(freeName([])).toBe("Receipt printer");
    expect(freeName([printer({ name: "Counter" })])).toBe("Receipt printer");
    expect(freeName([printer({ name: "receipt printer", active: false })])).toBe("Receipt printer 2");
    expect(freeName([printer(), printer({ id: 2, name: "Receipt printer 2" })])).toBe("Receipt printer 3");
  });
});

describe("addAndTest", () => {
  const usb = choicesOf(found)[0] as PrinterChoice;

  it("adds the printer, makes it the receipt printer, and prints a test page", async () => {
    const { api, calls } = server();
    const sent = await addAndTest(api, usb, 58, null);
    expect(sent).toEqual({ printerId: 1, jobId: 77 });
    expect(calls).toEqual([
      "GET /api/v1/printers",
      'POST /api/v1/printers {"name":"Receipt printer","transport":"windows","address":"EPSON TM-T82","paper_mm":58}',
      'PUT /api/v1/printers/receipt {"printer_id":1}',
      "POST /api/v1/printers/1/test",
    ]);
  });

  it("uses a printer the venue already has at that address, and adds nothing", async () => {
    const { api, calls, list } = server([printer({ id: 4, name: "Counter", address: "192.168.1.60:9100" })]);
    const sent = await addAndTest(api, typedChoice("192.168.1.60"), 80, null);
    expect(sent.printerId).toBe(4);
    expect(list()).toHaveLength(1);
    expect(calls.some((c) => c.startsWith("POST /api/v1/printers {"))).toBe(false);
  });

  it("brings a retired one back, and gives it the paper chosen", async () => {
    const { api, calls } = server([printer({ id: 4, name: "Counter", active: false, paper_mm: 80 })]);
    await addAndTest(api, typedChoice("192.168.1.60"), 58, null);
    expect(calls).toContain("POST /api/v1/printers/4/restore");
    expect(calls).toContain('PUT /api/v1/printers/4 {"name":"Counter","transport":"tcp","address":"192.168.1.60","paper_mm":58}');
  });

  it("retires the printer an earlier try added, once the new one is the receipt printer", async () => {
    const { api, calls, list } = server([printer({ id: 1, address: "192.168.1.61" })]);
    const sent = await addAndTest(api, usb, 80, 1);
    expect(sent.printerId).toBe(2);
    expect(calls.indexOf("POST /api/v1/printers/1/retire")).toBeGreaterThan(calls.indexOf('PUT /api/v1/printers/receipt {"printer_id":2}'));
    expect(list().map((p) => [p.name, p.active])).toEqual([
      ["Receipt printer", false],
      ["Receipt printer 2", true],
    ]);
  });

  it("does not retire the printer it is testing again", async () => {
    const { api, calls } = server([printer({ id: 1 })]);
    await addAndTest(api, typedChoice("192.168.1.60"), 80, 1);
    expect(calls.some((c) => c.includes("/retire"))).toBe(false);
  });

  it("still prints when the old printer could not be retired", async () => {
    const { api } = server([printer({ id: 1, address: "192.168.1.61" })], {
      "POST /api/v1/printers/1/retire": new ApiError("internal", "internal error", 500),
    });
    await expect(addAndTest(api, usb, 80, 1)).resolves.toEqual({ printerId: 2, jobId: 77 });
  });

  it("fails with the server's refusal, and prints nothing", async () => {
    const refusal = new ApiError("invalid_printer", "A printer's address is its IP on this network.", 422, "address");
    const { api, calls } = server([], { "POST /api/v1/printers": refusal });
    await expect(addAndTest(api, typedChoice("8.8.8.8"), 80, null)).rejects.toBe(refusal);
    expect(calls).toEqual(["GET /api/v1/printers"]);
  });
});

describe("forget", () => {
  it("retires the printer, does nothing with none, and never throws", async () => {
    const { api, calls, list } = server([printer({ id: 3 })]);
    await forget(api, null);
    expect(calls).toEqual([]);
    await forget(api, 3);
    expect(list()[0]?.active).toBe(false);
    const broken = server([], { "POST /api/v1/printers/3/retire": new ApiError("network", "unreachable") });
    await expect(forget(broken.api, 3)).resolves.toBeUndefined();
  });
});

describe("outcomeOf", () => {
  const sent = { printerId: 1, jobId: 77 };
  it.each<[string, PrinterResponse[], string | null]>([
    ["a page that was sent", [withJob(77, "done")], "sent"],
    ["a page the printer did not take", [withJob(77, "failed", "connection refused")], "failed"],
    ["a page given up on", [withJob(77, "dead", "no printer named that")], "failed"],
    ["a page still queued", [withJob(77, "queued")], null],
    ["a page being sent", [withJob(77, "printing")], null],
    ["another job's result", [withJob(76, "done")], null],
    ["a printer with no job", [printer()], null],
    ["a printer that is not there", [], null],
  ])("%s", (_name, printers, kind) => {
    expect(outcomeOf(printers, sent)?.kind ?? null).toBe(kind);
  });

  it("gives the queue's reason for a failure", () => {
    expect(outcomeOf([withJob(77, "failed", "connection refused")], sent)?.message).toContain("connection refused");
  });
});

describe("watchTest", () => {
  const sent = { printerId: 1, jobId: 77 };

  /** A clock that moves only when slept on, and a server that answers with each list in turn. */
  function watching(lists: (PrinterResponse[] | Error)[]) {
    let at = 0;
    let looks = 0;
    const api = {
      get: async () => {
        const next = lists[Math.min(looks++, lists.length - 1)];
        if (next instanceof Error) throw next;
        return { printers: next, stations: [] };
      },
    } as unknown as ApiClient;
    const sleep = async (ms: number) => void (at += ms);
    return { run: () => watchTest(api, sent, undefined, sleep, () => at), looks: () => looks, elapsed: () => at };
  }

  it("answers as soon as the page is sent", async () => {
    const w = watching([[withJob(77, "queued")], [withJob(77, "printing")], [withJob(77, "done")]]);
    expect((await w.run()).kind).toBe("sent");
    expect(w.looks()).toBe(3);
  });

  it("waits out a failed send in case the queue's retry gets through", async () => {
    const w = watching([[withJob(77, "failed", "connection refused")], [withJob(77, "printing")], [withJob(77, "done")]]);
    expect((await w.run()).kind).toBe("sent");
  });

  it("says failed, with the reason, when no retry got through in time", async () => {
    const w = watching([[withJob(77, "failed", "connection refused")]]);
    const out = await w.run();
    expect([out.kind, out.message.includes("connection refused")]).toEqual(["failed", true]);
    expect(w.elapsed()).toBeGreaterThanOrEqual(12_000);
  });

  it("says it does not know when the queue never answered", async () => {
    const w = watching([[withJob(77, "queued")]]);
    expect((await w.run()).kind).toBe("unknown");
  });

  it("looks again after a look that failed", async () => {
    const w = watching([new ApiError("network", "unreachable"), [withJob(77, "done")]]);
    expect((await w.run()).kind).toBe("sent");
  });

  it("stops when it is told to", async () => {
    const stop = new AbortController();
    stop.abort();
    const out = await watchTest(
      {} as ApiClient,
      sent,
      stop.signal,
      async () => {},
      () => 0,
    );
    expect(out.kind).toBe("unknown");
  });
});

describe("printerError", () => {
  const host = "localhost:7777";
  it("puts a refused address on the address", () => {
    expect(printerError(new ApiError("invalid_printer", "A printer's address is its IP on this network.", 422, "address"), host)).toEqual({
      message: "A printer's address is its IP on this network.",
      address: true,
    });
  });
  it.each<[string, unknown, string]>([
    ["a server that cannot be reached", new ApiError("network", "unreachable"), host],
    ["someone who may not add printers", new ApiError("forbidden", "no", 403, "settings.edit"), "not allowed"],
    ["too many searches", new ApiError("rate_limited", "slow down", 429), "Wait a minute"],
    [
      "a server that cannot search",
      new ApiError("no_printer_search", "This server cannot search for printers. Add one by its address.", 501),
      "by its address",
    ],
    ["a refusal about something else", new ApiError("invalid_printer", "Choose 80mm or 58mm.", 422, "paper_mm"), "80mm"],
    ["anything unforeseen", new TypeError("x"), "Try again"],
  ])("%s", (_name, err, says) => {
    const out = printerError(err, host);
    expect(out.message).toContain(says);
    expect(out.address).toBe(false);
  });
});
