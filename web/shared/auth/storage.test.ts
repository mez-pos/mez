import { describe, expect, it } from "vitest";
import { credentialStore, type KeyValue, type StoredDevice, type StoredStaff } from "./storage";

const memory = (): KeyValue & { keys(): string[] } => {
  const map = new Map<string, string>();
  return {
    getItem: (k) => map.get(k) ?? null,
    setItem: (k, v) => void map.set(k, v),
    removeItem: (k) => void map.delete(k),
    keys: () => [...map.keys()],
  };
};

const device = (token: string, kind: "terminal" | "back_office" = "terminal"): StoredDevice => ({
  token,
  device: { id: `dev_${token}`, name: "Counter 1", kind },
});
const staff = (token: string): StoredStaff => ({
  token,
  staff: { id: "stf_1", name: "Arjun", role: "manager" },
  permissions: ["reports.view"],
  expiresAt: "2026-10-04T20:00:00Z",
});

describe("credentialStore", () => {
  it("keeps each app's credentials apart", () => {
    const durable = memory();
    const session = memory();
    credentialStore("terminal", durable, session).saveDevice(device("nfd_terminal"));
    expect(credentialStore("backoffice", durable, session).device()).toBeNull();
    expect(credentialStore("terminal", durable, session).device()?.token).toBe("nfd_terminal");
  });

  describe("an app that shares another's pairing", () => {
    it("uses the other's device when it has none of its own, so one pairing opens both", () => {
      const durable = memory();
      const session = memory();
      credentialStore("terminal", durable, session).saveDevice(device("nfd_terminal"));
      const backOffice = credentialStore("backoffice", durable, session, "terminal");
      expect(backOffice.device()?.token).toBe("nfd_terminal");
    });

    it("prefers its own pairing to the other's", () => {
      const durable = memory();
      const session = memory();
      credentialStore("terminal", durable, session).saveDevice(device("nfd_terminal"));
      const backOffice = credentialStore("backoffice", durable, session, "terminal");
      backOffice.saveDevice(device("nfd_office", "back_office"));
      expect(backOffice.device()?.token).toBe("nfd_office");
    });

    it("carries the other's staff session only while it is on the other's device", () => {
      const durable = memory();
      const session = memory();
      const terminal = credentialStore("terminal", durable, session);
      terminal.saveDevice(device("nfd_terminal"));
      terminal.saveStaff(staff("nfs_terminal"));

      const backOffice = credentialStore("backoffice", durable, session, "terminal");
      expect(backOffice.staff()?.token).toBe("nfs_terminal");

      // Once it has a pairing of its own, a session from the terminal's device
      // would be refused by the server, so it is not offered.
      backOffice.saveDevice(device("nfd_office", "back_office"));
      expect(backOffice.staff()).toBeNull();
    });

    it("has one session with the other while it is on the shared device: signing in at either is signing in at both", () => {
      const durable = memory();
      const session = memory();
      const terminal = credentialStore("terminal", durable, session);
      terminal.saveDevice(device("nfd_terminal"));
      terminal.saveStaff(staff("nfs_first"));
      const backOffice = credentialStore("backoffice", durable, session, "terminal");

      // A later person signs in at the terminal: the back office sees them, not
      // a copy of the person before.
      terminal.saveStaff(staff("nfs_second"));
      expect(backOffice.staff()?.token).toBe("nfs_second");

      // And one who signs in at the back office is the terminal's session too.
      backOffice.saveStaff(staff("nfs_third"));
      expect(terminal.staff()?.token).toBe("nfs_third");
      expect(
        durable
          .keys()
          .concat(session.keys())
          .filter((k) => k.includes("backoffice")),
      ).toEqual([]);
    });

    it("locking at the back office locks the terminal too, with nothing left behind to read", () => {
      const durable = memory();
      const session = memory();
      const terminal = credentialStore("terminal", durable, session);
      terminal.saveDevice(device("nfd_terminal"));
      terminal.saveStaff(staff("nfs_terminal"));
      const backOffice = credentialStore("backoffice", durable, session, "terminal");
      backOffice.forgetStaff();
      expect(backOffice.staff()).toBeNull();
      expect(terminal.staff()).toBeNull();
    });

    it("writes its own session under its own key once it has its own device", () => {
      const durable = memory();
      const session = memory();
      const terminal = credentialStore("terminal", durable, session);
      terminal.saveDevice(device("nfd_terminal"));
      terminal.saveStaff(staff("nfs_terminal"));
      const backOffice = credentialStore("backoffice", durable, session, "terminal");
      backOffice.saveDevice(device("nfd_office", "back_office"));
      backOffice.saveStaff(staff("nfs_office"));
      expect(backOffice.staff()?.token).toBe("nfs_office");
      expect(terminal.staff()?.token).toBe("nfs_terminal");
    });

    it("refreshing the device it uses writes back where it came from, so it never gains a copy of its own", () => {
      const durable = memory();
      const session = memory();
      const terminal = credentialStore("terminal", durable, session);
      terminal.saveDevice(device("nfd_terminal"));
      terminal.saveStaff(staff("nfs_terminal"));
      const backOffice = credentialStore("backoffice", durable, session, "terminal");

      const renamed = { ...device("nfd_terminal"), device: { id: "dev_nfd_terminal", name: "Bar till", kind: "terminal" as const } };
      backOffice.refreshDevice(renamed);

      expect(terminal.device()?.device.name).toBe("Bar till");
      expect(durable.keys().filter((k) => k.includes("backoffice"))).toEqual([]);
      // Still on the shared device, so the shared session is still offered.
      expect(backOffice.staff()?.token).toBe("nfs_terminal");
    });

    it("refreshes its own device when it has one", () => {
      const durable = memory();
      const session = memory();
      credentialStore("terminal", durable, session).saveDevice(device("nfd_terminal"));
      const backOffice = credentialStore("backoffice", durable, session, "terminal");
      backOffice.saveDevice(device("nfd_office", "back_office"));
      backOffice.refreshDevice({ ...device("nfd_office", "back_office"), device: { id: "dev_nfd_office", name: "Office", kind: "back_office" } });
      expect(backOffice.device()?.device.name).toBe("Office");
      expect(credentialStore("terminal", durable, session).device()?.device.name).toBe("Counter 1");
    });

    it("a device revoked while shared is forgotten for both: it is one device", () => {
      const durable = memory();
      const session = memory();
      credentialStore("terminal", durable, session).saveDevice(device("nfd_terminal"));
      const backOffice = credentialStore("backoffice", durable, session, "terminal");
      backOffice.forgetDevice();
      expect(backOffice.device()).toBeNull();
      expect(credentialStore("terminal", durable, session).device()).toBeNull();
    });

    it("forgetting its own device leaves the terminal paired", () => {
      const durable = memory();
      const session = memory();
      credentialStore("terminal", durable, session).saveDevice(device("nfd_terminal"));
      const backOffice = credentialStore("backoffice", durable, session, "terminal");
      backOffice.saveDevice(device("nfd_office", "back_office"));
      backOffice.forgetDevice();
      expect(credentialStore("terminal", durable, session).device()?.token).toBe("nfd_terminal");
    });
  });

  it("shares nothing unless asked to", () => {
    const durable = memory();
    const session = memory();
    credentialStore("terminal", durable, session).saveDevice(device("nfd_terminal"));
    expect(credentialStore("kitchen", durable, session).device()).toBeNull();
  });
});
