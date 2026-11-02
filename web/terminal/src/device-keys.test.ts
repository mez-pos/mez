import { describe, expect, it } from "vitest";
import { credentialStore, type AppKind, type KeyValue } from "../../shared/auth/storage";
import { prefsKey } from "./kitchen/board-prefs";
import { stationKey } from "./kitchen/kitchen-logic";

// The mez app for Android tablets follows the PC to a new address by
// carrying every localStorage key that starts with "mez." from the old
// address to the new one, and only when a "mez.<app>.device" is among them
// (ADR-416). A key the till or the kitchen display keeps across a restart
// outside that prefix would be left behind at the old address.

const recording = (): KeyValue & { keys(): string[] } => {
  const map = new Map<string, string>();
  return {
    getItem: (k) => map.get(k) ?? null,
    setItem: (k, v) => void map.set(k, v),
    removeItem: (k) => void map.delete(k),
    keys: () => [...map.keys()],
  };
};

describe("what a tablet carries to the PC's new address", () => {
  it.each<AppKind>(["terminal", "kitchen", "backoffice"])("keeps the %s's pairing as mez.<app>.device", (app) => {
    const durable = recording();
    credentialStore(app, durable, recording()).saveDevice({ token: "nfd_x", device: { id: "dev_x", name: "Counter 1", kind: "terminal" } });
    expect(durable.keys()).toEqual([`mez.${app}.device`]);
  });

  it("keeps the kitchen display's station and board under the same prefix", () => {
    for (const key of [stationKey, prefsKey]) expect(key).toMatch(/^mez\.kitchen\./);
  });
});
