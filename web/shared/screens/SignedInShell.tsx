// The frame around every signed-in screen: where you are, who you are,
// whether the server is there, and the way out. Screens fill `children`.

import type { ReactNode } from "react";
import { useAuth, useAuthStore } from "../auth/use-auth";
import { ConnectionIndicator } from "../ConnectionIndicator";
import { Button, Meta } from "../ui";

interface SignedInShellProps {
  area: string;
  /** "Lock" on a shared till; "Sign out" on a manager's own computer. */
  exitLabel: "Lock" | "Sign out";
  /**
   * The screen takes the whole height below the header and scrolls inside
   * itself, as the register does so its total never leaves the screen.
   */
  fill?: boolean;
  /** The app's own status in the header, before the connection: the till's print queue. */
  status?: ReactNode;
  children: ReactNode;
}

export function SignedInShell({ area, exitLabel, fill = false, status: extra, children }: SignedInShellProps) {
  const status = useAuth();
  const store = useAuthStore();
  if (status.kind !== "signed_in") return null; // the Gate never lets this happen

  return (
    <div className={fill ? "flex h-screen flex-col overflow-hidden bg-canvas text-fg" : "flex min-h-screen flex-col bg-canvas text-fg"}>
      <header className="flex flex-wrap items-center justify-between gap-4 border-b border-line bg-chrome px-6 py-2">
        <Meta items={["mez", area, status.device.name]} />
        <div className="flex items-center gap-tap-gap">
          {extra}
          <ConnectionIndicator />
          <span className="px-2 text-base font-semibold">{status.staff.name}</span>
          <Button onClick={() => void store.signOut()}>{exitLabel}</Button>
        </div>
      </header>
      <main className={fill ? "flex min-h-0 flex-1" : "flex-1 p-8"}>{children}</main>
    </div>
  );
}
