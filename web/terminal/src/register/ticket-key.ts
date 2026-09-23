// Idempotency keys for the register's commands.
//
// Not crypto.randomUUID(): it exists only in a secure context, and a till
// reaching the venue's server at http://192.168.1.20 is not one. The random
// bytes behind it are available everywhere.

const hex = (b: number) => b.toString(16).padStart(2, "0");

/** 128 random bits as 32 hex characters. */
export function newKey(
  random: (bytes: Uint8Array<ArrayBuffer>) => Uint8Array = (b) => crypto.getRandomValues(b),
): string {
  return Array.from(random(new Uint8Array(16)), hex).join("");
}

const ticketKey = "nofiltr.terminal.ticket";

/** Where the current ticket's order id survives a reload. */
export interface TicketMemory {
  read(): string | null;
  write(orderId: string | null): void;
}

/** Session storage, or nothing where it is refused: the ticket then lives until reload. */
export function ticketMemory(storage: Pick<Storage, "getItem" | "setItem" | "removeItem"> | null): TicketMemory {
  return {
    read: () => {
      try {
        const id = storage?.getItem(ticketKey) ?? null;
        return id && /^[A-Za-z0-9_]{1,64}$/.test(id) ? id : null;
      } catch {
        return null;
      }
    },
    write: (orderId) => {
      try {
        if (orderId) storage?.setItem(ticketKey, orderId);
        else storage?.removeItem(ticketKey);
      } catch {
        // storage refused: the ticket is only lost on reload
      }
    },
  };
}
