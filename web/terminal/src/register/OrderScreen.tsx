// The order screen (D03): categories down the left, the menu in the middle, the
// order on the right. A card's "+" rings the item up with its default choices;
// the card itself opens the item modal (D04) when there is anything to choose
// (ADR-154); the total never leaves the screen. Pay goes to /pay on the same order, the floor is /tables, a
// settled bill is /orders/:id, and with no shift open the whole terminal waits
// at /shift (ADR-148): each is a route of its own, and the order in front lives
// above them all (terminal-context). ADR-151 records what this screen draws.
//
// The search stands over the categories (TILL-3), and on a short screen the
// menu's chrome closes up, so a 1024x768 tablet shows six dishes.
//
// A waiter's is T02, the same screen rearranged (ADR-165): Popular first, three
// cards across, and a rail that ends in Save and Send to Kitchen. The table,
// the guests and how it is served are in the top bar (WaiterOrderBar).

import { useMemo, useState, type ReactNode } from "react";
import { useLocation } from "wouter";
import type { LineResponse, MenuItemResponse } from "../../../shared/api-types";
import { indexMenu, isSoldOut } from "../../../shared/menu";
import { useAuth } from "../../../shared/auth/use-auth";
import { backOffice } from "../shell/nav-logic";
import { EmptyState, InboxIcon, SearchField, SearchIcon, useToast } from "../../../shared/ui";
import { useClock } from "../../../shared/use-clock";
import { useMenu } from "../../../shared/use-menu";
import { route } from "../shell/routes";
import { useTerminal } from "../shell/terminal-context";
import { useWaiterFrame } from "../shell/use-waiter-frame";
import { CategorySidebar } from "./CategorySidebar";
import { HistorySheet } from "./HistorySheet";
import { ItemGrid } from "./ItemGrid";
import { ItemModal, type ItemChoice } from "./ItemModal";
import { MenuHeader } from "./MenuHeader";
import { sameModifiers } from "../../../shared/modifier-selection";
import { OrderRail } from "./OrderRail";
import { courseNamer } from "./rail-logic";
import {
  addLineFor,
  categoryIconKey,
  dietFilterActive,
  filterItems,
  itemsForView,
  noDietFilter,
  ringsUpWith,
  searchItems,
  sidebarEntries,
  type CardTap,
  type DietFilter,
  type MenuView,
} from "./register-logic";
import { TicketPanels, type Panel } from "./TicketPanels";
import { TicketsSheet } from "./TicketsSheet";

export function OrderScreen() {
  const auth = useAuth();
  const menu = useMenu();
  const { ticket, names } = useTerminal();
  const [, navigate] = useLocation();
  const index = useMemo(() => (menu.data ? indexMenu(menu.data) : null), [menu.data]);
  // Once a minute, so a dish sold out yesterday is back when the trading day turns.
  const now = useClock(60_000);

  const waiter = useWaiterFrame();
  const [picked, setView] = useState<MenuView>(waiter ? "popular" : "all");
  const [query, setQuery] = useState("");
  const [diet, setDiet] = useState<DietFilter>(noDietFilter);
  const [sheet, setSheetState] = useState<SheetState | null>(null);
  const [panel, setPanelState] = useState<Panel | null>(null);
  const [ordersOpen, setOrdersOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
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

  const categories = useMemo(() => index?.menu.categories ?? [], [index]);
  const items = useMemo(() => index?.menu.items ?? [], [index]);
  const entries = useMemo(() => sidebarEntries(categories, items), [categories, items]);
  // Popular is where a waiter starts (T02), on a menu that marks anything popular.
  const view = picked === "popular" && !entries.some((e) => e.view === "popular") ? "all" : picked;
  const courseName = useMemo(() => courseNamer(index?.menu.courses ?? []), [index]);
  const searching = query !== "";
  const shown = useMemo(() => {
    const base = searching ? searchItems(items, query) : itemsForView(view, items);
    return filterItems(base, diet);
  }, [items, searching, query, view, diet]);
  const filtering = dietFilterActive(diet);
  const title = searching ? "Search results" : (entries.find((e) => e.view === view)?.label ?? "All Items");

  const iconFor = useMemo(() => {
    const byCategory = new Map(categories.map((c) => [c.id, categoryIconKey(c.name)]));
    return (item: MenuItemResponse) => byCategory.get(item.category_id) ?? "utensils";
  }, [categories]);

  const ringUp = (item: MenuItemResponse, tap: CardTap) => {
    if (!index) return;
    const modifiers = ringsUpWith(index.groupsFor(item), tap);
    if (modifiers) ticket.ringUp(addLineFor(item, modifiers));
    else setSheet({ kind: "add", item, error: null, busy: false });
  };

  /** The menu item behind a line the kitchen has not seen, if it has choices. */
  const editable = (line: LineResponse): MenuItemResponse | null => {
    if (!index || line.status !== "active" || line.sent || line.course_fired) return null;
    const item = index.item(line.item_id);
    return item && index.groupsFor(item).length > 0 ? item : null;
  };

  const confirm = async (state: SheetState, choice: ItemChoice) => {
    if (state.kind === "add") {
      ticket.ringUp(addLineFor(state.item, choice.modifiers, { qty: choice.qty, course: choice.course }), choice.note);
      setSheet(null);
      return;
    }
    // A line being changed: each thing that differs is its own event, in turn.
    const { line } = state;
    const steps = [
      ...(sameModifiers(line.modifiers, choice.modifiers) ? [] : [() => ticket.modify(line.id, { from: line.modifiers ?? [], to: choice.modifiers })]),
      ...(choice.qty === line.qty ? [] : [() => ticket.changeQty(line.id, line.qty, choice.qty)]),
      ...(choice.note === "" ? [] : [() => ticket.addNote({ scope: "line", line_id: line.id, text: choice.note })]),
    ];
    if (steps.length === 0) {
      setSheet(null);
      return;
    }
    setSheet({ ...state, busy: true, error: null });
    let saved = 0;
    try {
      for (const step of steps) {
        await step();
        saved += 1;
      }
      setSheet(null);
    } catch (err) {
      // Once part of it is saved the modal shows an old copy of the line: close
      // it and say what is missing, so the rest is done from the line as it is.
      if (saved > 0) {
        setSheet(null);
        fail("Only part of the change was saved")(err);
      } else {
        setSheet({ ...state, busy: false, error: err instanceof Error ? err.message : String(err) });
      }
    }
  };

  return (
    <div className="flex min-w-0 flex-1">
      <CategorySidebar
        entries={entries}
        chosen={searching ? null : view}
        onChoose={(next) => {
          setView(next);
          clearSearch();
        }}
        // "Search": a longer placeholder is cut off in a 184px sidebar.
        search={<SearchField key={searchKey} label="Search" onSearch={setQuery} />}
        waiter={waiter}
      />
      <section aria-label="Menu" className="flex min-w-0 flex-1 flex-col px-4 pt-4 short:pt-3 2xl:px-6 2xl:pt-5">
        <MenuHeader title={title} filter={diet} onFilter={setDiet} />
        <div className="min-h-0 flex-1 overflow-y-auto pb-4 pt-4 short:pb-3 short:pt-3 2xl:pb-6 2xl:pt-5">
          <MenuBody
            addHref={auth.kind === "signed_in" && auth.permissions.includes("menu.edit") ? backOffice.menu : null}
            state={menu.isPending ? "loading" : menu.isError && !menu.data ? "error" : "ready"}
            empty={items.length === 0}
            shown={shown.length}
            searching={searching}
            filtering={filtering}
            query={query}
            onRetry={() => void menu.refetch()}
            onClearSearch={clearSearch}
            onClearFilter={() => setDiet(noDietFilter)}
          >
            <ItemGrid
              items={shown}
              groupsFor={(item) => index?.groupsFor(item) ?? []}
              iconFor={iconFor}
              onTap={ringUp}
              soldOut={(item) => isSoldOut(item, new Date(now))}
              dense={waiter}
            />
          </MenuBody>
        </div>
      </section>

      <OrderRail
        ticket={ticket}
        currency={index?.menu.currency ?? "INR"}
        names={names}
        dietFor={(line) => index?.item(line.item_id)?.diet}
        onOpenLine={(line) => setPanel({ kind: "line", lineId: line.id })}
        onVoidLine={(line) => setPanel({ kind: "void", lineId: line.id })}
        onNote={() => setPanel({ kind: "note" })}
        onDiscount={() => setPanel({ kind: "discount", lineId: null })}
        onRemoveDiscount={(d) => setPanel({ kind: "remove_discount", seq: d.seq })}
        onOrders={() => setOrdersOpen(true)}
        onHistory={() => setHistoryOpen(true)}
        onPastBills={() => navigate(route.orders)}
        onTables={() => navigate(route.tables)}
        onPay={() => {
          if (ticket.orderId) navigate(route.pay(ticket.orderId));
        }}
        onFail={fail}
        waiter={waiter}
        courseName={courseName}
        {...(waiter ? { onSaved: () => navigate(route.waiter) } : {})}
      />

      {historyOpen && ticket.orderId && <HistorySheet key={ticket.orderId} orderId={ticket.orderId} onClose={() => setHistoryOpen(false)} />}

      {ordersOpen && (
        <TicketsSheet
          open={ticket.open}
          openError={ticket.openError}
          onRetryOpen={ticket.refetchOpen}
          currentId={ticket.orderId}
          names={names}
          onPick={(t) => {
            // A sheet about a line on the order being left must not stay open.
            setPanel(null);
            setSheet(null);
            setHistoryOpen(false);
            return t.status === "held" ? ticket.resume(t.id) : Promise.resolve(ticket.switchTo(t.id));
          }}
          onNew={(opening) => {
            setPanel(null);
            ticket.startNew(opening);
          }}
          onPastBills={() => navigate(route.orders)}
          onFloor={() => navigate(route.tables)}
          onClose={() => setOrdersOpen(false)}
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
        <ItemModal
          key={sheet.kind === "edit" ? `line-${sheet.line.id}` : `item-${sheet.item.id}`}
          item={sheet.item}
          groups={index.groupsFor(sheet.item)}
          courses={index.menu.courses}
          line={sheet.kind === "edit" ? sheet.line : undefined}
          error={sheet.error}
          blocked={sheet.kind === "edit" && !sheet.busy ? movedOn(sheet.line, ticket.order?.lines) : null}
          busy={sheet.busy}
          onConfirm={(choice) => void confirm(sheet, choice)}
          onClose={() => setSheet(null)}
        />
      )}
    </div>
  );
}

/**
 * Why a line being edited can no longer be saved as the modal shows it:
 * the stream has brought a newer version of it. Null while it still can.
 */
function movedOn(opened: LineResponse, lines: readonly LineResponse[] | undefined): string | null {
  const live = lines?.find((l) => l.id === opened.id);
  if (!live || live.status !== "active") return "This line has been voided or comped. Close this.";
  if (live.sent || live.course_fired) return "This has gone to the kitchen. Void it and add it again.";
  if (!sameModifiers(live.modifiers, opened.modifiers) || live.qty !== opened.qty) {
    return "This line was changed on another screen. Close this and look again.";
  }
  return null;
}

/** What the item modal is doing: ringing up a new line, or changing one. */
type SheetState =
  | { kind: "add"; item: MenuItemResponse; error: string | null; busy: boolean }
  | { kind: "edit"; item: MenuItemResponse; line: LineResponse; error: string | null; busy: boolean };

interface MenuBodyProps {
  state: "loading" | "error" | "ready";
  empty: boolean;
  /** How many items the view, search and filters leave. */
  shown: number;
  searching: boolean;
  filtering: boolean;
  query: string;
  onRetry: () => void;
  onClearSearch: () => void;
  onClearFilter: () => void;
  /** Where the menu is edited, for someone who may; null for everyone else. */
  addHref: string | null;
  children: ReactNode;
}

function MenuBody({ state, empty, shown, searching, filtering, query, onRetry, onClearSearch, onClearFilter, addHref, children }: MenuBodyProps) {
  if (state === "loading") return <p className="py-12 text-center text-base text-fg-muted">Loading the menu</p>;
  if (state === "error") {
    return <EmptyState icon={<InboxIcon />} line="Can't reach the mez server to load the menu." action={{ label: "Try again", onClick: onRetry }} />;
  }
  if (empty) {
    return (
      <EmptyState
        icon={<InboxIcon />}
        line={addHref ? "No items on the menu yet." : "No items on the menu yet. A manager adds them in the back office, under Menu."}
        {...(addHref ? { action: { label: "Add items to the menu", href: addHref } } : {})}
      />
    );
  }
  if (shown === 0 && filtering) {
    return <EmptyState icon={<SearchIcon />} line="Nothing here matches the filters." action={{ label: "Clear filters", onClick: onClearFilter }} />;
  }
  if (shown === 0 && searching) {
    return <EmptyState icon={<SearchIcon />} line={`Nothing matches "${query}".`} action={{ label: "Clear search", onClick: onClearSearch }} />;
  }
  if (shown === 0) return <p className="py-12 text-center text-base text-fg-muted">Nothing in this category yet.</p>;
  return <>{children}</>;
}
