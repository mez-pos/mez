// Where credentials live between page loads.
//
// The device token is in localStorage: it survives a restart, which is the
// point of pairing once. The staff session is in sessionStorage: it survives
// a reload but not the browser closing, so a till that crashed asks for a PIN
// again, which is the safer surprise.
//
// Keys carry the app, because the terminal and the back office share an
// origin: a manager's laptop can be paired as both without one erasing the
// other.

import type { DeviceResponse, Permission, StaffResponse } from "../api-types";

export type AppKind = "terminal" | "backoffice" | "kitchen";

export interface StoredDevice {
  readonly token: string;
  readonly device: DeviceResponse;
}

export interface StoredStaff {
  readonly token: string;
  readonly staff: StaffResponse;
  /** Absent in a session stored before sessions carried them (ADR-105). */
  readonly permissions?: readonly Permission[];
  readonly expiresAt: string;
}

/** The part of Storage this needs; tests pass a Map-backed one. */
export interface KeyValue {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export interface CredentialStore {
  device(): StoredDevice | null;
  staff(): StoredStaff | null;
  saveDevice(value: StoredDevice): void;
  /** Writes back what the server now says of the device in use, where it was read from. */
  refreshDevice(value: StoredDevice): void;
  saveStaff(value: StoredStaff): void;
  forgetDevice(): void;
  forgetStaff(): void;
}

/**
 * An app may share another's pairing (ADR-148): the back office shares the
 * terminal's, so a manager signed in on a till reaches Menu and Reports with
 * one pairing for the browser.
 *
 * While the app has no device of its own and the other's is there, the shared
 * device is the one in use, and so is its session slot: the server ties a
 * session to its device, so there is one session for the device, not one per
 * app. Every read, write and forget then acts on that slot, which is why a
 * sign-in or a lock at either app is one at both, and a revoked device is
 * forgotten for both. Once the app is paired on its own, it uses its own keys
 * alone and the other's are left as they were.
 */
export function credentialStore(app: AppKind, durable: KeyValue | null, session: KeyValue | null, shares?: AppKind): CredentialStore {
  const own = { device: `mez.${app}.device`, staff: `mez.${app}.staff` };
  const shared = shares ? { device: `mez.${shares}.device`, staff: `mez.${shares}.staff` } : null;
  const sharing = () => shared !== null && read(durable, own.device, isStoredDevice) === null && read(durable, shared.device, isStoredDevice) !== null;
  /** The keys in use: the shared device's while sharing, else the app's own. */
  const slot = () => (sharing() && shared ? shared : own);
  return {
    device: () => read(durable, own.device, isStoredDevice) ?? (shared ? read(durable, shared.device, isStoredDevice) : null),
    staff: () => read(session, slot().staff, isStoredStaff),
    // Pairing is the app's own, whatever it was sharing before.
    saveDevice: (value) => write(durable, own.device, value),
    refreshDevice: (value) => write(durable, slot().device, value),
    saveStaff: (value) => write(session, slot().staff, value),
    forgetDevice: () => {
      const keys = slot();
      remove(durable, keys.device);
      // The session went with the device it belonged to, whichever is asked for next.
      remove(session, keys.staff);
    },
    forgetStaff: () => remove(session, slot().staff),
  };
}

/** The browser's storage, or null where it is refused (a locked-down kiosk profile). */
export function browserStorage(which: "local" | "session"): KeyValue | null {
  try {
    const storage = which === "local" ? window.localStorage : window.sessionStorage;
    const probe = "mez.probe";
    storage.setItem(probe, "1");
    storage.removeItem(probe);
    return storage;
  } catch {
    return null;
  }
}

// Storage is outside the program: another tab, an old build or a person with
// devtools may have written it. Anything that does not have the right shape
// is treated as absent.
function read<T>(kv: KeyValue | null, key: string, valid: (v: unknown) => v is T): T | null {
  try {
    const raw = kv?.getItem(key);
    if (!raw) return null;
    const value: unknown = JSON.parse(raw);
    return valid(value) ? value : null;
  } catch {
    return null;
  }
}

function write(kv: KeyValue | null, key: string, value: unknown): void {
  try {
    kv?.setItem(key, JSON.stringify(value));
  } catch {
    // Full or refused. The session carries on in memory; it will not survive a reload.
  }
}

function remove(kv: KeyValue | null, key: string): void {
  try {
    kv?.removeItem(key);
  } catch {
    // Refused. Nothing more can be done from here.
  }
}

const isObject = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null;

function isStoredDevice(v: unknown): v is StoredDevice {
  return isObject(v) && typeof v.token === "string" && isObject(v.device) && typeof v.device.id === "string" && typeof v.device.name === "string";
}

function isStoredStaff(v: unknown): v is StoredStaff {
  const perms = isObject(v) ? v.permissions : undefined;
  const permsOK = perms === undefined || (Array.isArray(perms) && perms.every((p) => typeof p === "string"));
  // A session stored before sessions named the role has none: it is dropped,
  // and the person signs in again, rather than carried with a role of undefined.
  return (
    isObject(v) &&
    typeof v.token === "string" &&
    typeof v.expiresAt === "string" &&
    isObject(v.staff) &&
    typeof v.staff.id === "string" &&
    typeof v.staff.role === "string" &&
    permsOK
  );
}
