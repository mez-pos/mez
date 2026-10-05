// The terminal's paths, in one place, so a screen asks for where to go by
// name and the shape of a path is written once.

export const route = {
  /**
   * The order screen: the menu and the order being taken. Not "/order": the
   * server gives /order/ to the guest app, so a reload there would load it.
   */
  order: "/pos",
  tables: "/tables",
  /** The floor with one table picked: where a waiter's card leads until T03 is built. */
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
  /** The kitchen display: another app of the same bundle. */
  kitchen: "/kitchen",
} as const;
