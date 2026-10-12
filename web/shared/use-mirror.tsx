// React binding for MirrorWatch (mirror.ts). An app that can be opened on a
// mirror hands its watch to MirrorProvider; one that cannot hands none, and
// is never told it is on one.

import { createContext, useContext, useSyncExternalStore, type ReactNode } from "react";
import { MirrorWatch, type MirrorMark } from "./mirror";

// A watch nothing is shown to: the terminal, which no mirror serves.
const MirrorContext = createContext<MirrorWatch>(new MirrorWatch());

interface MirrorProviderProps {
  watch: MirrorWatch;
  children: ReactNode;
}

export function MirrorProvider({ watch, children }: MirrorProviderProps) {
  return <MirrorContext.Provider value={watch}>{children}</MirrorContext.Provider>;
}

/**
 * What the mirror last said of its copy, or null when this page is not on a
 * mirror. Null until the first answer has come back, which is before anyone
 * is signed in: the device is checked first.
 */
export function useMirror(): MirrorMark | null {
  const watch = useContext(MirrorContext);
  return useSyncExternalStore(watch.subscribe, watch.current);
}
