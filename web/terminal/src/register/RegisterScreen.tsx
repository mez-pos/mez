// The register (docs/design/screens/terminal-register.md): the menu on the
// left, the ticket on the right. One tap on an item rings it up with its
// default modifiers, or opens the modifier sheet when someone has to choose;
// the ticket's total never leaves the screen. Charge swaps it for the tender
// screen (terminal-tender.md) on the same ticket, and Floor swaps it for the
// dining room (terminal-floor.md), where a party is seated or a table's
// ticket found. With no shift open the till is closed (terminal-close.md),
// and a counted shift shows its close until it is closed.

import { useMemo, useState, type ReactNode } from "react";
import type { LineModifier, LineResponse, MenuItemResponse } from "../../../shared/api-types";
import { indexMenu, isSoldOut } from "../../../shared/menu";
import { Button, EmptyState, InboxIcon, SearchField, SearchIcon, useToast } from "../../../shared/ui";
import { useClock } from "../../../shared/use-clock";
import { useMenu } from "../../../shared/use-menu";
import { CategoryRail } from "./CategoryRail";
import { ItemGrid } from "./ItemGrid";
import { ModifierSheet } from "./ModifierSheet";
import { sameModifiers } from "./modifier-selection";
import { addLineFor, defaultModifiers, searchItems } from "./register-logic";
import { TicketPanels, type Panel } from "./TicketPanels";
import { TicketRail } from "./TicketRail";
import { TicketsSheet } from "./TicketsSheet";
import { HistorySheet } from "./HistorySheet";
import { BillScreen } from "../bills/BillScreen";
import { PastBillsSheet } from "../bills/PastBillsSheet";
import { TenderScreen } from "../tender/TenderScreen";
import { chargeBlock, chargeLabel } from "../tender/tender-logic";
import { sendPlan } from "./send-logic";
import { ticketLabel } from "./ticket-time";
import { useTicket } from "./use-ticket";
import { FloorView } from "../floor/FloorView";
import { tableNames } from "../floor/floor-logic";
import { useLiveFloor } from "../floor/use-live-floor";
import { CloseDayScreen } from "../shift/CloseDayScreen";
import { TillClosedScreen } from "../shift/TillClosedScreen";
import { useShift } from "../shift/use-shift";
import { ZReportScreen } from "../../../shared/zreport/ZReportScreen";

export function RegisterScreen() {
  const menu = useMenu();
  const ticket = useTicket();
  const floor = useLiveFloor();
  const shift = useShift();
  // The shift being closed, by id: a new shift is not.
  const [closing, setClosing] = useState<number | null>(null);
  // The Z-report this till's close issued, and whether it is on screen.
  const [lastZ, setLastZ] = useState<number | null>(null);
  const [viewingZ, setViewingZ] = useState(false);
  const names = useMemo(() => tableNames(floor.data), [floor.data]);
  const [floorOpen, setFloorOpen] = useState(false);
  const index = useMemo(() => (menu.data ? indexMenu(menu.data) : null), [menu.data]);
  // Once a minute, so a dish sold out yesterday is back when the trading day turns.
  const now = useClock(60_000);

  const [chosen, setChosen] = useState<number | null>(null);
  const [query, setQuery] = useState("");
  const [sheet, setSheetState] = useState<SheetState | null>(null);
  const [panel, setPanelState] = useState<Panel | null>(null);
  const [ticketsOpen, setTicketsOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  // The ticket being charged. Kept by id, so the tender screen goes away if
  // another ticket comes to the front.
  const [charging, setCharging] = useState<string | null>(null);
  // A settled bill on screen (Task 6b), and the sheet that finds one.
  const [bill, setBill] = useState<string | null>(null);
  const [pastBillsOpen, setPastBillsOpen] = useState(false);
  const toast = useToast();
  const fail = (what: string) => (err: unknown) => toast(`${what}: ${err instanceof Error ? err.message : String(err)}`, { tone: "warning" });
  // One sheet at a time: opening one closes the other.
  const setSheet = (next: SheetState | null) => {
    setSheetState(next);
    if (next) setPanelState(null);
  };
  const setPanel = (next: Panel | null) => {
    setPanelState(next);
    if (next) setSheetState(null);
  };
  // Bumped to empty the search box: SearchField keeps what is typed in it.
  const [searchKey, setSearchKey] = useState(0);
  const clearSearch = () => {
    setQuery("");
    setSearchKey((k) => k + 1);
  };

  const categories = index?.menu.categories ?? [];
  // The first category is chosen until someone picks another, so the most
  // common items are one tap from the moment the screen opens.
  const category = chosen ?? categories[0]?.id ?? null;
  const searching = query !== "";

  const counts = useMemo(() => new Map(categories.map((c) => [c.id, index?.itemsIn(c.id).length ?? 0])), [categories, index]);
  const shown = useMemo(() => {
    if (!index) return [];
    if (searching) return searchItems(index.menu.items, query);
    return category === null ? [] : [...index.itemsIn(category)];
  }, [index, searching, query, category]);

  const stations = useMemo(() => new Map(index?.menu.stations.map((s) => [s.code, s.name])), [index]);

  const ringUp = (item: MenuItemResponse) => {
    if (!index) return;
    const modifiers = defaultModifiers(index.groupsFor(item));
    if (modifiers) ticket.ringUp(addLineFor(item, modifiers));
    else setSheet({ kind: "add", item, error: null, busy: false });
  };

  /** The menu item behind a line the kitchen has not seen, if it has choices. */
  const editable = (line: LineResponse): MenuItemResponse | null => {
    if (!index || line.status !== "active" || line.sent || line.course_fired) return null;
    const item = index.item(line.item_id);
    return item && index.groupsFor(item).length > 0 ? item : null;
  };

  const confirm = async (state: SheetState, modifiers: LineModifier[]) => {
    if (state.kind === "add") {
      ticket.ringUp(addLineFor(state.item, modifiers));
      setSheet(null);
      return;
    }
    if (sameModifiers(state.line.modifiers, modifiers)) {
      setSheet(null);
      return;
    }
    setSheet({ ...state, busy: true, error: null });
    try {
      await ticket.modify(state.line.id, { from: state.line.modifiers ?? [], to: modifiers });
      setSheet(null);
    } catch (err) {
      setSheet({ ...state, busy: false, error: err instanceof Error ? err.message : String(err) });
    }
  };

  const block = chargeBlock(ticket.order, ticket.pending.length);
  const label = ticketLabel(ticket.order ?? ticket.opening, names);

  const currency = index?.menu.currency ?? "INR";
  if (!shift.shift) {
    if (viewingZ && lastZ) {
      return <ZReportScreen number={lastZ} backLabel="Back" onBack={() => setViewingZ(false)} />;
    }
    return <TillClosedScreen currency={currency} state={shift} lastZ={lastZ} onViewZ={() => setViewingZ(true)} />;
  }
  if (shift.shift.counted || closing === shift.shift.id) {
    return (
      <CloseDayScreen
        shift={shift.shift}
        currency={currency}
        commands={shift.commands}
        onBack={() => setClosing(null)}
        onClosed={(z) => {
          setLastZ(z);
          setViewingZ(false);
        }}
      />
    );
  }

  if (floorOpen) {
    return <FloorView floor={floor} ticket={ticket} onRegister={() => setFloorOpen(false)} />;
  }

  if (bill) {
    return (
      <BillScreen
        orderId={bill}
        onBack={() => setBill(null)}
        onTakeBalance={(id) => {
          setBill(null);
          ticket.switchTo(id);
          setCharging(id);
        }}
      />
    );
  }

  if (charging && ticket.order && charging === ticket.orderId) {
    return <TenderScreen order={ticket.order} ticket={ticket} label={label} onBack={() => setCharging(null)} />;
  }

  return (
    <div className="flex min-w-0 flex-1">
      <section aria-label="Menu" className="flex min-w-0 flex-1 flex-col">
        <div className="flex flex-wrap items-start gap-tap-gap px-6 pt-4">
          <Button
            onClick={() => {
              setPanel(null);
              setSheet(null);
              setHistoryOpen(false);
              setFloorOpen(true);
            }}
          >
            Floor
          </Button>
          <CategoryRail
            categories={categories}
            counts={counts}
            chosen={searching ? null : category}
            onChoose={(id) => {
              setChosen(id);
              clearSearch();
            }}
          />
          <SearchField key={searchKey} label="Search the menu" onSearch={setQuery} className="ml-auto w-64" />
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-6 pt-4">
          <MenuBody
            state={menu.isPending ? "loading" : menu.isError && !menu.data ? "error" : "ready"}
            empty={index?.menu.items.length === 0}
            searching={searching}
            query={query}
            onRetry={() => void menu.refetch()}
            onClearSearch={clearSearch}
          >
            {shown.length > 0 && (
              <ItemGrid
                items={shown}
                groupsFor={(item) => index?.groupsFor(item) ?? []}
                stationName={(code) => stations.get(code) ?? code}
                onTap={ringUp}
                soldOut={(item) => isSoldOut(item, new Date(now))}
              />
            )}
            {shown.length === 0 && !searching && categories.length > 0 && (
              <p className="py-12 text-center text-base text-fg-muted">Nothing in this category yet.</p>
            )}
          </MenuBody>
        </div>
      </section>

      <TicketRail
        order={ticket.order}
        orderId={ticket.orderId}
        pending={ticket.pending}
        currency={index?.menu.currency ?? "INR"}
        label={label}
        ticketCount={Math.max(ticket.open?.orders.length ?? 0, ticket.orderId ? 1 : 0)}
        onTickets={() => setTicketsOpen(true)}
        onHistory={() => setHistoryOpen(true)}
        onHold={() => ticket.hold().catch(fail("Couldn't hold"))}
        onResume={() => {
          if (ticket.orderId) ticket.resume(ticket.orderId).catch(fail("Couldn't resume"));
        }}
        onOpenLine={(line) => setPanel({ kind: "line", lineId: line.id })}
        onOrderDiscount={() => setPanel({ kind: "discount", lineId: null })}
        onOrderNote={() => setPanel({ kind: "note" })}
        onRetry={ticket.retry}
        onRemove={ticket.remove}
        sendPlan={sendPlan(ticket.order)}
        onSend={() => ticket.send().catch(fail("Couldn't send"))}
        onFire={(course) => ticket.fire(course).catch(fail(`Couldn't fire course ${course}`))}
        chargeBlock={block}
        chargeLabel={chargeLabel(ticket.order)}
        onCharge={() => {
          setPanel(null);
          setSheet(null);
          setHistoryOpen(false);
          setCharging(ticket.orderId);
        }}
      />

      {historyOpen && ticket.orderId && <HistorySheet key={ticket.orderId} orderId={ticket.orderId} onClose={() => setHistoryOpen(false)} />}

      {ticketsOpen && (
        <TicketsSheet
          open={ticket.open}
          currentId={ticket.orderId}
          names={names}
          onPick={(t) => {
            // A sheet about a line on the ticket being left must not stay open.
            setPanel(null);
            setSheet(null);
            setHistoryOpen(false);
            return t.status === "held" ? ticket.resume(t.id) : Promise.resolve(ticket.switchTo(t.id));
          }}
          onNew={(opening) => {
            setPanel(null);
            ticket.startNew(opening);
          }}
          onPastBills={() => setPastBillsOpen(true)}
          onFloor={() => setFloorOpen(true)}
          onCloseDay={() => {
            setPanel(null);
            setSheet(null);
            setHistoryOpen(false);
            setClosing(shift.shift?.id ?? null);
          }}
          onClose={() => setTicketsOpen(false)}
        />
      )}

      {pastBillsOpen && (
        <PastBillsSheet
          onPick={(b) => {
            setPastBillsOpen(false);
            setBill(b.id);
          }}
          onClose={() => setPastBillsOpen(false)}
        />
      )}

      <TicketPanels
        panel={panel}
        onPanel={setPanel}
        ticket={ticket}
        hasChoices={(line) => editable(line) !== null}
        onModify={(line) => {
          const item = editable(line);
          if (item) setSheet({ kind: "edit", item, line, error: null, busy: false });
        }}
      />

      {sheet && index && (
        <ModifierSheet
          key={sheet.kind === "edit" ? `line-${sheet.line.id}` : `item-${sheet.item.id}`}
          item={sheet.item}
          groups={index.groupsFor(sheet.item)}
          current={sheet.kind === "edit" ? (sheet.line.modifiers ?? []) : undefined}
          error={sheet.error}
          blocked={sheet.kind === "edit" && !sheet.busy ? movedOn(sheet.line, ticket.order?.lines) : null}
          busy={sheet.busy}
          onConfirm={(mods) => void confirm(sheet, mods)}
          onClose={() => setSheet(null)}
        />
      )}
    </div>
  );
}

/**
 * Why a line being edited can no longer be saved as the sheet shows it:
 * the stream has brought a newer version of it. Null while it still can.
 */
function movedOn(opened: LineResponse, lines: readonly LineResponse[] | undefined): string | null {
  const live = lines?.find((l) => l.id === opened.id);
  if (!live || live.status !== "active") return "This line has been voided or comped. Close this.";
  if (live.sent || live.course_fired) return "This has gone to the kitchen. Void it and add it again.";
  if (!sameModifiers(live.modifiers, opened.modifiers)) {
    return "This line was changed on another till. Close this and look again.";
  }
  return null;
}

/** What the modifier sheet is doing: ringing up a new line, or changing one. */
type SheetState =
  | { kind: "add"; item: MenuItemResponse; error: string | null; busy: boolean }
  | { kind: "edit"; item: MenuItemResponse; line: LineResponse; error: string | null; busy: boolean };

interface MenuBodyProps {
  state: "loading" | "error" | "ready";
  empty: boolean;
  searching: boolean;
  query: string;
  onRetry: () => void;
  onClearSearch: () => void;
  children: ReactNode;
}

function MenuBody({ state, empty, searching, query, onRetry, onClearSearch, children }: MenuBodyProps) {
  if (state === "loading") return <p className="py-12 text-center text-base text-fg-muted">Loading the menu</p>;
  if (state === "error") {
    return <EmptyState icon={<InboxIcon />} line="Can't reach the mez server to load the menu." action={{ label: "Try again", onClick: onRetry }} />;
  }
  if (empty) return <EmptyState icon={<InboxIcon />} line="No items on the menu yet. Add them in the back office." />;
  if (searching && !hasChildren(children)) {
    return <EmptyState icon={<SearchIcon />} line={`Nothing matches "${query}".`} action={{ label: "Clear search", onClick: onClearSearch }} />;
  }
  return <>{children}</>;
}

function hasChildren(children: ReactNode): boolean {
  return Array.isArray(children) ? children.some(Boolean) : Boolean(children);
}
