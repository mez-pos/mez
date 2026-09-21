// Every primitive, on ink and on paper, side by side. Development only: main.tsx
// loads this lazily behind import.meta.env.DEV, so a production build drops it.
// Open http://localhost:5173/gallery with `npm run dev -w terminal`.

import { useState, type ReactNode } from "react";
import type { Money as Amount } from "../../shared/api-types";
import { Button, Card, Meta, Money, Qty, StatusChip, type ButtonSize, type ButtonVariant, type ChipStatus } from "../../shared/ui";

const inr = (minor: number): Amount => ({ minor, currency: "INR" });

const variants: ButtonVariant[] = ["primary", "secondary", "ghost", "danger"];
const sizes: ButtonSize[] = ["sm", "md", "tap", "charge"];

const bill: [string, Amount][] = [
  ["Cappuccino", inr(36_000)],
  ["Avocado toast", inr(1_12_000)],
  ["Service 10%", inr(14_800)],
  ["GST 5%", inr(8_140)],
  ["Refund, cold coffee", inr(-22_000)],
];

const chips: ChipStatus[] = [
  { kind: "kitchen", status: "queued" },
  { kind: "kitchen", status: "firing" },
  { kind: "kitchen", status: "ready" },
  { kind: "kitchen", status: "late" },
  { kind: "kitchen", status: "void" },
  { kind: "order", status: "open" },
  { kind: "order", status: "held" },
  { kind: "order", status: "settling" },
  { kind: "order", status: "closed" },
  { kind: "order", status: "reopened" },
  { kind: "payment", status: "pending" },
  { kind: "payment", status: "captured" },
  { kind: "payment", status: "voided" },
  { kind: "payment", status: "refunded" },
  { kind: "line", status: "active" },
  { kind: "line", status: "voided" },
  { kind: "line", status: "comped" },
];

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mt-8">
      <Meta items={["Gallery", title]} />
      <div className="mt-3">{children}</div>
    </section>
  );
}

function Buttons() {
  return (
    <div className="flex flex-col gap-3">
      {variants.map((variant) => (
        <div key={variant} className="flex flex-wrap items-center gap-tap-gap">
          {sizes.map((size) => (
            <Button key={size} variant={variant} size={size}>
              {size === "charge" ? (
                <>
                  Charge <Money amount={inr(1_70_940)} />
                </>
              ) : (
                `${variant} ${size}`
              )}
            </Button>
          ))}
          <Button variant={variant} disabled>
            Disabled
          </Button>
        </div>
      ))}
    </div>
  );
}

function Amounts() {
  return (
    <div className="flex flex-col gap-4">
      <table className="w-full max-w-sm border-collapse">
        <tbody>
          {bill.map(([name, amount]) => (
            <tr key={name} className="border-b border-line-soft">
              <td className="py-1.5 text-fg-2">{name}</td>
              <td className="py-1.5 text-right">
                <Money amount={amount} tone={amount.minor < 0 ? "danger" : "default"} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="flex items-baseline justify-between gap-4">
        <span className="text-fg-2">Total</span>
        <Money amount={inr(1_48_940)} variant="total" />
      </div>
      <div className="flex flex-wrap gap-6 text-sm">
        <Money amount={{ minor: 1234, currency: "JPY" }} />
        <Money amount={{ minor: 1234, currency: "KWD" }} />
        <Money amount={{ minor: 99, currency: "GBP" }} />
        <Money amount={inr(5_000)} tone="accent" />
        <Money amount={inr(5_000)} tone="muted" />
        <Money amount={{ minor: 12.5, currency: "INR" }} />
      </div>
    </div>
  );
}

function Quantities() {
  const [qty, setQty] = useState(2);
  return (
    <div className="flex flex-wrap items-center gap-6">
      <Qty value={qty} onChange={setQty} label="Flat white" />
      <Qty value={1} onChange={() => undefined} label="At the minimum" />
      <Qty value={3} />
    </div>
  );
}

function Primitives() {
  return (
    <>
      <Section title="Button">
        <Buttons />
      </Section>
      <Section title="Money">
        <Amounts />
      </Section>
      <Section title="Qty">
        <Quantities />
      </Section>
      <Section title="StatusChip">
        <div className="flex flex-wrap gap-2">
          {chips.map((chip) => (
            <StatusChip key={`${chip.kind}-${chip.status}`} status={chip} />
          ))}
        </div>
      </Section>
      <Section title="Card and Meta">
        <Card className="max-w-sm">
          <Meta items={["Table 4", "Dine in", "Covers 3"]} />
          <p className="mt-2 font-serif text-xl">Good evening, table four</p>
          <div className="mt-3 flex items-center justify-between">
            <StatusChip status={{ kind: "order", status: "open" }} />
            <Money amount={inr(1_48_940)} />
          </div>
        </Card>
      </Section>
    </>
  );
}

export default function Gallery() {
  return (
    <div className="grid min-h-screen grid-cols-1 xl:grid-cols-2">
      <div data-surface="ink" className="p-8">
        <h1 className="font-serif text-total">Ink</h1>
        <Primitives />
      </div>
      <div data-surface="paper" className="p-8">
        <h1 className="font-serif text-total">Paper</h1>
        <Primitives />
      </div>
    </div>
  );
}
