/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Development only; see shared/credentials.ts. */
  readonly VITE_DEVICE_TOKEN?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
