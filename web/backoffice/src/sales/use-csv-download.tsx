// A Download CSV button for a report (backoffice-reports.md, ADR-119): reads
// the file as the signed-in client and hands it to the browser to save.

import { useState, type ReactNode } from "react";
import type { CsvResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Button, useToast } from "../../../shared/ui";
import { saveFile } from "../save-file";

/**
 * Returns the button for a file's path. One download at a time per caller.
 * `stale` turns it off while the figures on screen are not the path's: a
 * file must match its screen.
 */
export function useCsvDownload(): (path: string, stale: boolean) => ReactNode {
  const store = useAuthStore();
  const toast = useToast();
  const [busy, setBusy] = useState<string | null>(null);

  const download = async (path: string) => {
    setBusy(path);
    try {
      const got = await store.api.get<CsvResponse>(path);
      saveFile(got.filename, got.text, got.content_type);
    } catch (err) {
      toast(`Couldn't download the file: ${err instanceof Error ? err.message : String(err)}`, { tone: "warning" });
    } finally {
      setBusy(null);
    }
  };

  return (path, stale) => (
    <Button size="md" disabled={busy !== null || stale} onClick={() => void download(path)}>
      {busy === path ? "Downloading" : "Download CSV"}
    </Button>
  );
}
