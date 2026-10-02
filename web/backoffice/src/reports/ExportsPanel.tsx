// A closed shift's export files under its Z-report (zreport.md, ADR-104):
// the tax register, the Tally vouchers and the day's sales, each saved
// from the server as a CSV file.

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import type { ExportFileResponse, ExportKind, ExportResponse, ExportsResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Button, Card, useToast } from "../../../shared/ui";
import { saveFile } from "../save-file";

const kinds: Readonly<Record<ExportKind, { title: string; what: string }>> = {
  tax_register: { title: "Tax register", what: "Every invoice and credit note by the columns your tax regime reports." },
  tally: { title: "Tally vouchers", what: "A sales voucher per invoice and a credit note voucher per credit note, for Tally's import." },
  sales: { title: "Day's sales", what: "One row per invoice or credit note, with what it was paid by." },
};

/** A size a person reads: "12.4 KB". */
export function sizeText(bytes: number): string {
  if (bytes < 1024) return `${bytes} bytes`;
  return `${(Math.round((bytes / 1024) * 10) / 10).toString()} KB`;
}

export function ExportsPanel({ shiftId }: { shiftId: number }) {
  const store = useAuthStore();
  const toast = useToast();
  const [busy, setBusy] = useState<number | null>(null);
  const files = useQuery({
    queryKey: ["exports", shiftId],
    queryFn: ({ signal }) => store.api.get<ExportsResponse>(`/api/v1/shift/${shiftId}/exports`, { signal }),
    staleTime: Infinity,
  });

  const download = async (f: ExportFileResponse) => {
    setBusy(f.id);
    try {
      const got = await store.api.get<ExportResponse>(`/api/v1/exports/${f.id}`);
      saveFile(got.filename, got.text, got.content_type);
    } catch (err) {
      toast(`Couldn't download ${f.filename}: ${err instanceof Error ? err.message : String(err)}`, { tone: "warning" });
    } finally {
      setBusy(null);
    }
  };

  return (
    <section aria-label="Exports" className="mx-auto mt-4 flex w-full max-w-3xl flex-col gap-3">
      <h2 className="font-serif text-xl">Exports</h2>
      {files.isPending && <p className="text-base text-fg-muted">Loading the files</p>}
      {files.isError && <p className="text-base text-danger">Can't reach the Nofiltr server to list the files.</p>}
      {files.data?.files.length === 0 && (
        <p className="text-base text-fg-muted">This shift has no export files. Shifts closed before exports existed have none.</p>
      )}
      {files.data?.files.map((f) => (
        <Card key={f.id} className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0">
            <p className="text-base font-semibold">{kinds[f.kind].title}</p>
            <p className="text-sm text-fg-muted">{kinds[f.kind].what}</p>
            <p className="mt-1 truncate font-mono text-sm text-fg-2">
              {f.filename} · {sizeText(f.size)}
            </p>
          </div>
          <Button disabled={busy === f.id} onClick={() => void download(f)}>
            Download
          </Button>
        </Card>
      ))}
    </section>
  );
}
