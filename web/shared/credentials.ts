// Where the device and staff tokens come from, until Task 6 stores them
// durably after pairing and sign-in.
//
// A development build may take a device token from VITE_DEVICE_TOKEN (in
// web/<app>/.env.local, gitignored), so the stream can be tried against a
// real server before the pairing screen exists. A production build never
// reads it: Vite replaces import.meta.env.DEV with false and drops the branch.

import type { Credentials } from "./api";

export const devCredentials: Credentials = {
  deviceToken: () => (import.meta.env.DEV ? (import.meta.env.VITE_DEVICE_TOKEN ?? null) : null),
  staffToken: () => null,
};
