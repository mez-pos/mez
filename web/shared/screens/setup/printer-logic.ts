// The wizard's printer step, apart from how it is drawn (setup-wizard.md,
// ADR-268): what a search found, as choices; adding the one chosen as the
// receipt printer; and what became of its test page.
//
// It is the printers API the back office uses, called by the owner the wizard
// has just signed in. Nothing here is the wizard's alone.

import { ApiError, type ApiClient } from "../../api";
import type { AssignRequest, DiscoverResponse, PrinterResponse, PrinterSettings, PrintersResponse, PrinterTransport, TestPageResponse } from "../../api-types";
import { unreachableMessage } from "../../auth/messages";

export type PaperMM = 80 | 58;

/** A printer the owner could pick: where it is, and what to call that on screen. */
export interface PrinterChoice {
  readonly transport: PrinterTransport;
  readonly address: string;
  readonly label: string;
  readonly detail: string;
  /** The venue has it already: picking it adds nothing, and tests the one that is there. */
  readonly known: boolean;
}

/** A choice's identity: what is searched for again must still be the thing that was picked. */
export function choiceKey(c: Pick<PrinterChoice, "transport" | "address">): string {
  return `${c.transport}|${sameAddress(c.transport, c.address)}`;
}

/** What a search found, as choices: this computer's own printers first, then the network's. */
export function choicesOf(found: DiscoverResponse): PrinterChoice[] {
  return [
    ...found.usb.map((c): PrinterChoice => ({
      transport: "windows",
      address: c.address,
      label: c.address,
      detail: "Plugged into this computer",
      known: c.known,
    })),
    ...found.network.map((c): PrinterChoice => ({
      transport: "tcp",
      address: c.address,
      label: `Network printer at ${c.address}`,
      detail: "Answers on the printer port",
      known: c.known,
    })),
  ];
}

/** A printer at an address the owner typed, for one the search did not find. */
export function typedChoice(address: string): PrinterChoice {
  const at = address.trim();
  return { transport: "tcp", address: at, label: `Network printer at ${at}`, detail: "Entered by hand", known: false };
}

/**
 * What is wrong with a typed address, or null. Only its shape: whether it is
 * on this network is the server's to say (ADR-259), and it says it under the
 * same field.
 */
export function addressProblem(address: string): string | null {
  const at = address.trim();
  if (at === "") return "Enter the printer's address.";
  if (!/^\d{1,3}(\.\d{1,3}){3}(:\d{1,5})?$/.test(at)) return "Enter the printer's IP address, such as 192.168.1.50.";
  return null;
}

/** One line on what was searched, for under the list. */
export function searchedWords(found: DiscoverResponse): string {
  if (found.searched.length === 0) return "This computer is not on a network that could be searched.";
  // An owner knows "this network", not "192.168.29.0/24" (SET-8).
  if (found.searched.length === 1) return "Searched this network and this computer's own printers.";
  return `Searched this computer's ${found.searched.length} networks and its own printers.`;
}

// As the server compares them: the default port is no difference, and Windows
// does not tell printer names apart by case.
function sameAddress(transport: PrinterTransport, address: string): string {
  const at = address.trim();
  return transport === "tcp" ? at.replace(/:9100$/, "") : at.toLowerCase();
}

function find(printers: readonly PrinterResponse[], choice: PrinterChoice): PrinterResponse | undefined {
  return printers.find((p) => p.transport === choice.transport && sameAddress(p.transport, p.address) === sameAddress(choice.transport, choice.address));
}

const baseName = "Receipt printer";

/** A name no printer of the venue has: retired ones keep theirs. */
export function freeName(printers: readonly PrinterResponse[]): string {
  const taken = new Set(printers.map((p) => p.name.toLowerCase()));
  if (!taken.has(baseName.toLowerCase())) return baseName;
  for (let n = 2; ; n++) {
    const name = `${baseName} ${n}`;
    if (!taken.has(name.toLowerCase())) return name;
  }
}

/** A test page on its way: which printer, and which job. */
export interface Sent {
  readonly printerId: number;
  readonly jobId: number;
}

/**
 * Makes `choice` the venue's receipt printer and queues a test page on it.
 *
 * A printer the venue already has at that address is used as it is (brought
 * back if it was retired, and given the paper chosen), so trying the same
 * printer twice adds it once. `replaces` is the printer an earlier try added
 * and the owner has now moved on from: it is retired, so a wrong guess does
 * not stay in the list as a printer.
 */
export async function addAndTest(api: ApiClient, choice: PrinterChoice, paper: PaperMM, replaces: number | null): Promise<Sent> {
  const before = await api.get<PrintersResponse>("/api/v1/printers");
  let printer = find(before.printers, choice);
  if (!printer) {
    const body: PrinterSettings = { name: freeName(before.printers), transport: choice.transport, address: choice.address.trim(), paper_mm: paper };
    const after = await api.post<PrintersResponse>("/api/v1/printers", body);
    printer = find(after.printers, choice);
    if (!printer) throw new ApiError("bad_response", "the server added a printer and did not list it");
  } else {
    if (!printer.active) await api.post<PrintersResponse>(`/api/v1/printers/${printer.id}/restore`);
    if (printer.paper_mm !== paper) {
      const body: PrinterSettings = { name: printer.name, transport: printer.transport, address: printer.address, paper_mm: paper };
      await api.put<PrintersResponse>(`/api/v1/printers/${printer.id}`, body);
    }
  }
  const receipt: AssignRequest = { printer_id: printer.id };
  await api.put<PrintersResponse>("/api/v1/printers/receipt", receipt);
  if (replaces !== null && replaces !== printer.id) {
    try {
      await api.post<PrintersResponse>(`/api/v1/printers/${replaces}/retire`);
    } catch {
      // Tidying up. The new printer is the receipt printer either way, and
      // the old one can be retired on the Printers screen.
    }
  }
  const test = await api.post<TestPageResponse>(`/api/v1/printers/${printer.id}/test`);
  return { printerId: printer.id, jobId: test.job_id };
}

/**
 * Retires a printer the wizard added and the owner is leaving without: one
 * that never printed must not stay as the venue's receipt printer, failing
 * every receipt of the first day. It never throws: the owner is on their way
 * out, and the Printers screen can do the same.
 */
export async function forget(api: ApiClient, printerId: number | null): Promise<void> {
  if (printerId === null) return;
  try {
    await api.post<PrintersResponse>(`/api/v1/printers/${printerId}/retire`);
  } catch {
    // Left as it is.
  }
}

/**
 * What became of a test page.
 *   sent     the bytes reached the printer. Paper is the owner's to confirm:
 *            the raw protocol has no acknowledgement.
 *   failed   the printer could not be reached, with the queue's reason
 *   unknown  the queue has not said in the time allowed
 */
export interface TestOutcome {
  readonly kind: "sent" | "failed" | "unknown";
  readonly message: string;
}

const pollEveryMs = 600;
const pollForMs = 12_000;

/** Reads one answer about the job from the printers list, or null while it is still on its way. */
export function outcomeOf(printers: readonly PrinterResponse[], sent: Sent): TestOutcome | null {
  const job = printers.find((p) => p.id === sent.printerId)?.last_job;
  if (!job || job.id !== sent.jobId) return null; // not picked up yet, or a later job has taken its place
  switch (job.state) {
    case "done":
      return { kind: "sent", message: "The test page was sent to the printer." };
    case "failed":
    case "dead":
      return { kind: "failed", message: job.error ? `The printer did not take the page: ${job.error}` : "The printer did not take the page." };
    default:
      return null;
  }
}

/**
 * Waits for the test page's outcome. `sleep` and `now` are the clock, so a
 * test does not have to wait twelve seconds to see a printer that never answers.
 */
export async function watchTest(
  api: ApiClient,
  sent: Sent,
  signal?: AbortSignal,
  sleep: (ms: number) => Promise<void> = (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
  now: () => number = Date.now,
): Promise<TestOutcome> {
  const until = now() + pollForMs;
  let failed: TestOutcome | null = null;
  while (now() < until && !signal?.aborted) {
    await sleep(pollEveryMs);
    let list: PrintersResponse;
    try {
      list = await api.get<PrintersResponse>("/api/v1/printers", signal ? { signal } : {});
    } catch {
      continue; // one missed look is not an answer
    }
    const outcome = outcomeOf(list.printers, sent);
    if (outcome?.kind === "sent") return outcome;
    // A failed send is retried by the queue: it is the answer only if nothing better comes.
    if (outcome?.kind === "failed") failed = outcome;
  }
  return failed ?? { kind: "unknown", message: "The printer has not answered yet. Check that it is switched on, and look for the page." };
}

/** A failure to add or test, in words for the owner, and the field it is about if it is the address. */
export function printerError(err: unknown, host: string): { message: string; address: boolean } {
  if (!(err instanceof ApiError)) return { message: "Something went wrong. Try again.", address: false };
  if (err.unreachable) return { message: unreachableMessage(host), address: false };
  switch (err.code) {
    case "invalid_printer":
      return { message: err.message, address: err.field === "address" };
    case "forbidden":
      return { message: "You are not allowed to add printers. Skip this step; a manager can add one in the back office.", address: false };
    case "no_printer_search":
    case "rate_limited":
      return { message: err.code === "rate_limited" ? "Too many searches. Wait a minute, or enter the printer's address." : err.message, address: false };
    default:
      return { message: `The server refused: ${err.message}`, address: false };
  }
}
