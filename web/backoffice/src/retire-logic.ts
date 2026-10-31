// What retiring takes with it, said before it is done (ADR-381 item 6, BO-6,
// ADR-387): an area, a table, a group of choices or a category is asked
// first, with a count of what goes with it, and the button that does it is
// the danger one on every screen. An item is not asked: it is undone
// (menu/use-retire-item.ts).

export interface RetireWords {
  readonly title: string;
  readonly body: string;
  /** The confirm button's words, or null when it cannot be retired yet and the body says why. */
  readonly confirm: string | null;
}

const count = (n: number, one: string, many: string) => (n === 1 ? `1 ${one}` : `${n} ${many}`);

/** An area is refused by the server while it has tables in service (backoffice-floor.md), so that is said instead of asked. */
export function retireAreaWords(name: string, tablesInService: number): RetireWords {
  if (tablesInService > 0) {
    return {
      title: `${name} has ${count(tablesInService, "table", "tables")} in service`,
      body: "Retire them, or move them to another area, before you retire the area.",
      confirm: null,
    };
  }
  return {
    title: `Retire ${name}?`,
    body: "It leaves the floor plan and Tables on the till. You can bring it back here.",
    confirm: `Retire ${name}`,
  };
}

/** A table off the floor gives up its cell, and its QR code leads nowhere until it is back. */
export function retireTableWords(name: string): RetireWords {
  return {
    title: `Retire ${name}?`,
    body: "It leaves the floor plan and the till, and its QR code leads nowhere until you bring it back.",
    confirm: `Retire ${name}`,
  };
}

/** A group of choices leaves the choice sheet of every item that offers it; the links stay for bringing it back. */
export function retireGroupWords(name: string, items: number): RetireWords {
  const body =
    items === 0
      ? "No item offers it. You can bring it back."
      : `${count(items, "item stops", "items stop")} offering it. Orders already open keep their choices. Bring it back and they offer it again.`;
  return { title: `Retire ${name}?`, body, confirm: `Retire ${name}` };
}

/** A category takes its items off every order screen with it. */
export function retireCategoryWords(name: string, items: number): RetireWords {
  const body =
    items === 0
      ? "It has no items. You can bring it back."
      : `${items === 1 ? "Its one item comes" : `Its ${items} items come`} off every order screen. You can bring it back.`;
  return { title: `Retire ${name}?`, body, confirm: `Retire ${name}` };
}
