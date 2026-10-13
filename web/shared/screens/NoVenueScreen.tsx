// What a paired device shows while the computer it opens has no venue yet
// (ADR-339): a new install at the address the venue's computer had, before
// the venue is set up or brought back. The device keeps its pairing and asks
// again by itself, so a till left on the counter carries on when the venue
// arrives, with nobody touching it.

import { useEffect, useState } from "react";
import { useAuthStore } from "../auth/use-auth";
import { Button, Logo, Meta } from "../ui";

/** How often it asks. The server's own answer says to ask again in five seconds. */
const askEveryMs = 5_000;

export function NoVenueScreen() {
  const store = useAuthStore();
  const [asking, setAsking] = useState(false);

  useEffect(() => {
    // A recheck that throws is asked again in five seconds like any other.
    const timer = setInterval(() => void store.recheck().catch(() => undefined), askEveryMs);
    return () => clearInterval(timer);
  }, [store]);

  const ask = async () => {
    setAsking(true);
    try {
      await store.recheck();
    } catch {
      // The button comes back, and the next asking is five seconds off.
    } finally {
      setAsking(false);
    }
  };

  return (
    <main className="flex min-h-screen items-center justify-center bg-canvas p-6 text-fg">
      <div className="flex w-full max-w-md flex-col gap-6">
        <div>
          <Logo className="mb-6 h-12" />
          <Meta items={["Waiting for the venue"]} />
          <h1 className="mt-3 font-serif text-total">This computer has no venue yet</h1>
          <p className="mt-3 text-base text-fg-2" role="status">
            mez is running there, and the venue is still to be set up or brought back. This device keeps its pairing. It carries on by itself when the venue is
            there.
          </p>
        </div>
        <Button onClick={() => void ask()} disabled={asking} className="w-full">
          Check again
        </Button>
        <Meta items={["Server", window.location.host]} />
      </div>
    </main>
  );
}
