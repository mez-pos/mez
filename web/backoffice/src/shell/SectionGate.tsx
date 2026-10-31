// Stands in front of a section's screen. A staff member who may not open it
// is told so, and the screen underneath never mounts, so none of its
// requests are made only to be refused.

import type { ReactNode } from "react";
import { useLocation } from "wouter";
import { Button, Card } from "../../../shared/ui";
import { usePermissions } from "./BackOfficeShell";
import { canOpen, section, type SectionId } from "./nav-logic";

export function SectionGate({ id, children }: { id: SectionId; children: ReactNode }) {
  const held = usePermissions();
  const [, navigate] = useLocation();
  const s = section(id);
  if (canOpen(s, held)) return <>{children}</>;

  return (
    <Card padding="lg" className="max-w-prose">
      <h1 className="font-serif text-total">That is not yours to open</h1>
      <p className="mt-2 text-base text-fg-2">
        {s.label} needs the permission <span className="font-mono">{s.needs}</span>. Ask the owner to add it to your role.
      </p>
      <Button className="mt-4" onClick={() => navigate("/")}>
        Back to the dashboard
      </Button>
    </Card>
  );
}
