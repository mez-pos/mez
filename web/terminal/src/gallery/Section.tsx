import type { ReactNode } from "react";
import { Meta } from "../../../shared/ui";

export function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mt-8">
      <Meta items={["Gallery", title]} />
      <div className="mt-3">{children}</div>
    </section>
  );
}
