// Who this device is and who is signed in on it, as one state machine.
//
//   checking   -> reading storage and asking the server what it still accepts
//   unpaired   -> no device token, or the server no longer accepts it
//   locked     -> a paired device with nobody signed in
//   signed_in  -> a staff member's session on this device
//
// Screens do not decide this; route.ts maps each state to a screen. Every
// request made through this store's client reports a revoked device or an
// expired session back here, so the state follows the server's word.

import { ApiError, createApiClient, type ApiClient, type CredentialFailure, type Credentials } from "../api";
import type { DeviceKind, DeviceResponse, PairRequest, PairResponse, Permission, SessionResponse, SignInRequest, StaffResponse } from "../api-types";
import type { AppKind, CredentialStore, StoredDevice } from "./storage";

export type AuthStatus =
  | { readonly kind: "checking" }
  | { readonly kind: "unpaired"; readonly notice: "revoked" | "unrecognised" | null }
  | { readonly kind: "locked"; readonly device: DeviceResponse; readonly notice: "expired" | "signed_out" | null }
  | {
      readonly kind: "signed_in";
      readonly device: DeviceResponse;
      readonly staff: StaffResponse;
      /** What the role grants, to hide what would be refused. The server still checks. */
      readonly permissions: readonly Permission[];
      readonly expiresAt: string;
    };

/** Which device kind each app pairs as. A terminal code opens a terminal. */
export const deviceKindFor: Readonly<Record<AppKind, DeviceKind>> = {
  terminal: "terminal",
  backoffice: "back_office",
  kitchen: "kitchen",
};

/** A code redeemed for the wrong app: the device exists, but not for this screen. */
export class WrongDeviceKind extends Error {
  override readonly name = "WrongDeviceKind";
  constructor(
    readonly got: DeviceKind,
    readonly want: DeviceKind,
  ) {
    super(`this code pairs a ${got} device, and this is the ${want} app`);
  }
}

type Listener = () => void;

export class AuthStore {
  private status: AuthStatus = { kind: "checking" };
  private booting: Promise<void> | null = null;
  private readonly listeners = new Set<Listener>();
  readonly credentials: Credentials;
  readonly api: ApiClient;

  constructor(
    private readonly app: AppKind,
    private readonly store: CredentialStore,
    fetchFn?: typeof fetch,
  ) {
    this.credentials = {
      deviceToken: () => this.store.device()?.token ?? null,
      staffToken: () => this.store.staff()?.token ?? null,
    };
    this.api = createApiClient({
      credentials: this.credentials,
      onCredentialFailure: (code) => this.credentialFailed(code),
      ...(fetchFn ? { fetch: fetchFn } : {}),
    });
  }

  current = (): AuthStatus => this.status;

  subscribe = (listener: Listener): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  /**
   * Once, at start. Asks the server whether the stored device and session
   * still stand. If the server cannot be reached, what is stored is trusted:
   * a till that boots before its server should not have to pair again, and
   * the connection indicator already says the server is missing.
   */
  boot(): Promise<void> {
    // Once per store. React runs effects twice in development, and a second
    // boot after the first forgot a refused device would find nothing stored
    // and overwrite the reason the screen must show.
    this.booting ??= this.checkStored();
    return this.booting;
  }

  private async checkStored(): Promise<void> {
    const stored = this.store.device();
    if (!stored) return this.set({ kind: "unpaired", notice: null });

    const device = await this.check(() => this.api.get<DeviceResponse>("/api/v1/devices/me"), stored.device);
    if (device === "refused") return; // credentialFailed has already moved us to unpaired
    this.store.refreshDevice({ ...stored, device });

    const staff = this.store.staff();
    if (!staff) return this.set({ kind: "locked", device, notice: null });
    const session = await this.check(() => this.api.get<SessionResponse>("/api/v1/sessions/current"), null);
    if (session === "refused") {
      // A revoked device has already moved to unpaired. An expired session
      // has only been forgotten: nobody was signed in yet to be locked out.
      if (this.status.kind === "checking") this.set({ kind: "locked", device, notice: "expired" });
      return;
    }
    if (session) this.store.saveStaff({ ...staff, staff: session.staff, permissions: session.permissions, expiresAt: session.expires_at });
    this.set({
      kind: "signed_in",
      device,
      staff: session?.staff ?? staff.staff,
      permissions: session?.permissions ?? staff.permissions ?? [],
      expiresAt: session?.expires_at ?? staff.expiresAt,
    });
  }

  /** Redeem a pairing code. Throws ApiError or WrongDeviceKind for the screen to explain. */
  async pair(code: string): Promise<void> {
    const body: PairRequest = { code };
    const paired = await this.api.post<PairResponse>("/api/v1/devices/pair", body);
    const want = deviceKindFor[this.app];
    if (paired.device.kind !== want) throw new WrongDeviceKind(paired.device.kind, want);

    const stored: StoredDevice = { token: paired.token, device: paired.device };
    this.store.saveDevice(stored);
    this.store.forgetStaff();
    this.set({ kind: "locked", device: paired.device, notice: null });
  }

  /** Open a staff session with a PIN. Throws ApiError: invalid_pin, locked_out, network. */
  async signIn(staffId: string, pin: string): Promise<void> {
    const device = this.pairedDevice();
    const body: SignInRequest = { staff_id: staffId, pin };
    const session = await this.api.post<SessionResponse>("/api/v1/sessions", body);
    if (!session.token) throw new ApiError("bad_response", "the server opened a session without a token");

    this.store.saveStaff({ token: session.token, staff: session.staff, permissions: session.permissions, expiresAt: session.expires_at });
    this.set({ kind: "signed_in", device, staff: session.staff, permissions: session.permissions, expiresAt: session.expires_at });
  }

  /** Lock the screen. Ends the session on the server if it can; locks either way. */
  async signOut(): Promise<void> {
    const device = this.pairedDevice();
    try {
      await this.api.del("/api/v1/sessions/current");
    } catch {
      // Unreachable or already gone. Forgetting the token here is what locks
      // this screen, and the server lets the session expire on its own.
    }
    this.store.forgetStaff();
    this.set({ kind: "locked", device, notice: "signed_out" });
  }

  /** From any request, and from the realtime stream: the server's word on our credentials. */
  credentialFailed(code: CredentialFailure): void {
    if (code === "sign_in_required") {
      this.store.forgetStaff();
      if (this.status.kind === "signed_in") this.set({ kind: "locked", device: this.status.device, notice: "expired" });
      return;
    }
    this.store.forgetStaff();
    this.store.forgetDevice();
    this.set({ kind: "unpaired", notice: code === "device_revoked" ? "revoked" : "unrecognised" });
  }

  private pairedDevice(): DeviceResponse {
    const s = this.status;
    if (s.kind === "locked" || s.kind === "signed_in") return s.device;
    const stored = this.store.device();
    if (stored) return stored.device;
    throw new Error("no paired device");
  }

  // A check the server may refuse (then the store has already moved on), fail
  // to answer (then `fallback`), or pass.
  private async check<T, F>(request: () => Promise<T>, fallback: F): Promise<T | F | "refused"> {
    try {
      return await request();
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) return "refused";
      return fallback;
    }
  }

  private set(next: AuthStatus): void {
    this.status = next;
    for (const listener of this.listeners) listener();
  }
}
