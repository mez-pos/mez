// The register (docs/design/screens/terminal-register.md): the menu on the
// left, the ticket on the right. One tap on an item rings it up with its
// default modifiers; the ticket's total never leaves the screen.

import { useMemo, useState, type ReactNode } from "react";
import type { MenuItemResponse } from "../../../shared/api-types";
import { indexMenu } from "../../../shared/menu";
import { EmptyState, InboxIcon, SearchField, SearchIcon } from "../../../shared/ui";
import { useMenu } from "../../../shared/use-menu";
import { CategoryRail } from "./CategoryRail";
import { ItemGrid } from "./ItemGrid";
import { addLineFor, defaultModifiers, searchItems } from "./register-logic";
import { TicketRail } from "./TicketRail";
import { useTicket } from "./use-ticket";

export function RegisterScreen() {
  const menu = useMenu();
  const ticket = useTicket();
  const index = useMemo(() => (menu.data ? indexMenu(menu.data) : null), [menu.data]);

  const [chosen, setChosen] = useState<number | null>(null);
  const [query, setQuery] = useState("");
  const [notice, setNotice] = useState<string | null>(null);
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

  const counts = useMemo(
    () => new Map(categories.map((c) => [c.id, index?.itemsIn(c.id).length ?? 0])),
    [categories, index],
  );
  const shown = useMemo(() => {
    if (!index) return [];
    if (searching) return searchItems(index.menu.items, query);
    return category === null ? [] : [...index.itemsIn(category)];
  }, [index, searching, query, category]);

  const stations = useMemo(() => new Map(index?.menu.stations.map((s) => [s.code, s.name])), [index]);

  const ringUp = (item: MenuItemResponse) => {
    if (!index) return;
    const modifiers = defaultModifiers(index.groupsFor(item));
    if (!modifiers) {
      // Task 3 opens the ModifierSheet here.
      setNotice(`Choose options for ${item.name}.`);
      return;
    }
    setNotice(null);
    ticket.ringUp(addLineFor(item, modifiers));
  };

  return (
    <div className="flex min-w-0 flex-1">
      <section aria-label="Menu" className="flex min-w-0 flex-1 flex-col">
        <div className="flex flex-wrap items-start gap-tap-gap px-6 pt-4">
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
        notice={notice}
        onRetry={ticket.retry}
        onRemove={ticket.remove}
      />
    </div>
  );
}

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
    return (
      <EmptyState
        icon={<InboxIcon />}
        line="Can't reach the Nofiltr server to load the menu."
        action={{ label: "Try again", onClick: onRetry }}
      />
    );
  }
  if (empty) return <EmptyState icon={<InboxIcon />} line="No items on the menu yet. Add them in the back office." />;
  if (searching && !hasChildren(children)) {
    return (
      <EmptyState
        icon={<SearchIcon />}
        line={`Nothing matches "${query}".`}
        action={{ label: "Clear search", onClick: onClearSearch }}
      />
    );
  }
  return <>{children}</>;
}

function hasChildren(children: ReactNode): boolean {
  return Array.isArray(children) ? children.some(Boolean) : Boolean(children);
}
