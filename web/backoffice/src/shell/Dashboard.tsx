// The back office home. A skeleton until the reports task gives it today's
// numbers: no placeholder figures, because a dash that reads as a zero is a
// lie on a money screen.

import { Link } from "wouter";
import { useAuth } from "../../../shared/auth/use-auth";
import { Card } from "../../../shared/ui";
import { LowStockCard } from "../stock/LowStockCard";
import { usePermissions } from "./BackOfficeShell";
import { visibleNav } from "./nav-logic";

export function Dashboard() {
  const status = useAuth();
  const held = usePermissions();
  const groups = visibleNav(held)
    .map((g) => ({ ...g, sections: g.sections.filter((s) => s.id !== "dashboard") }))
    .filter((g) => g.sections.length > 0);

  return (
    <div className="max-w-5xl">
      <h1 className="font-serif text-total">Back office</h1>
      {status.kind === "signed_in" && <p className="mt-1 text-sm text-fg-3">Signed in as {status.staff.name}</p>}

      <section aria-labelledby="today" className="mt-8">
        <h2 id="today" className="text-meta font-semibold uppercase text-fg-faint">
          Today
        </h2>
        <Card className="mt-2">
          <p className="text-sm text-fg-2">Sales, covers and open tickets for today arrive with reports.</p>
        </Card>
      </section>

      <LowStockCard held={held} />

      <section aria-labelledby="yours" className="mt-8">
        <h2 id="yours" className="text-meta font-semibold uppercase text-fg-faint">
          Your sections
        </h2>
        {groups.length === 0 ? (
          <Card className="mt-2">
            <p className="text-sm text-fg-2">Your role opens no back office sections. Ask the owner if you need one.</p>
          </Card>
        ) : (
          <div className="mt-2 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {groups.map((g) => (
              <Card key={g.label ?? "top"} padding="none">
                <h3 className="border-b border-line-soft px-4 py-2.5 text-base font-semibold">{g.label}</h3>
                <ul>
                  {g.sections.map((s) => (
                    <li key={s.id} className="border-b border-line-soft last:border-b-0">
                      {s.built ? (
                        <Link href={s.path} className="flex min-h-tap flex-col justify-center px-4 py-2 hover:bg-sunken focus-visible:bg-sunken">
                          <span className="text-sm font-semibold">{s.label}</span>
                          <span className="text-sm text-fg-3">{s.description}</span>
                        </Link>
                      ) : (
                        <div className="flex min-h-tap flex-col justify-center px-4 py-2">
                          <span className="flex items-center gap-2 text-sm font-semibold text-fg-muted">
                            {s.label}
                            <span className="text-meta uppercase text-fg-faint">Soon</span>
                          </span>
                          <span className="text-sm text-fg-3">{s.description}</span>
                        </div>
                      )}
                    </li>
                  ))}
                </ul>
              </Card>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
