// The terminal's paths, in one place, so a screen asks for where to go by
// name and the shape of a path is written once.

export const route = {
  /**
   * The order screen: the menu and the order being taken. Not "/order": the
   * server gives /order/ to the guest app, so a reload there would load it.
   */
  order: "/pos",
  tables: "/tables",
  /** The floor with one table picked: where a waiter's card leads for a table with several orders, or none to open. */
  table: (tableId: number) => `/tables?table=${tableId}`,
  /** Order history; with an id, one settled bill. */
  orders: "/orders",
  bill: (orderId: string) => `/orders/${encodeURIComponent(orderId)}`,
  /** Paying an order. */
  pay: (orderId: string) => `/pay/${encodeURIComponent(orderId)}`,
  /** Opening the shift: the till is closed until this is done. */
  shift: "/shift",
  /** Counting and closing the shift. */
  shiftClose: "/shift/close",
  /** A Z-report this till's close issued. */
  zReport: (number: number) => `/shift/z/${number}`,
  /** The waiter's home: the tables as cards (T01). */
  waiter: "/waiter",
  /** One table's own screen, for a waiter (T03). */
  waiterTable: (tableId: number) => `/waiter/table/${tableId}`,
  /** What the person signed in did in the open shift (T08). */
  waiterShift: "/waiter/shift",
  /**
   * The kitchen's board for someone signed in on a terminal (ADR-179). Not
   * "/kitchen": that path is the wall display's, another app of this bundle
   * with a pairing of its own (main.tsx).
   */
  pass: "/pass",
} as const;
