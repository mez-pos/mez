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
  saveStaff(value: StoredStaff): void;
  forgetDevice(): void;
  forgetStaff(): void;
}

export function credentialStore(app: AppKind, durable: KeyValue | null, session: KeyValue | null): CredentialStore {
  const deviceKey = `mez.${app}.device`;
  const staffKey = `mez.${app}.staff`;
  return {
    device: () => read(durable, deviceKey, isStoredDevice),
    staff: () => read(session, staffKey, isStoredStaff),
    saveDevice: (value) => write(durable, deviceKey, value),
    saveStaff: (value) => write(session, staffKey, value),
    forgetDevice: () => remove(durable, deviceKey),
    forgetStaff: () => remove(session, staffKey),
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
  return isObject(v) && typeof v.token === "string" && typeof v.expiresAt === "string" && isObject(v.staff) && typeof v.staff.id === "string" && permsOK;
}
