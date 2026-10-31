// The invoice as it was issued (terminal-tender.md, Task 3): a paper slip on
// the ink screen, the shape of the 80mm receipt the print queue will send.
// Every figure is from the invoice's own snapshots, never today's bill.

import type { IssuedInvoiceResponse, Money as Amount, PaymentMethod } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { declarationOf } from "../../../shared/tax-words";
import { invoiceTitle, issuedAtText, rateText, registrationText } from "./invoice-format";

const methodWords: Record<PaymentMethod, string> = { cash: "Cash", card: "Card", upi: "UPI", other: "Other" };

const negate = (a: Amount): Amount => ({ ...a, minor: -a.minor });

export function InvoicePreview({ invoice }: { invoice: IssuedInvoiceResponse }) {
  const p = invoice.prices;
  const seller = p?.seller;
  const payments = invoice.tenders.filter((t) => t.status !== "voided");
  return (
    <article
      data-surface="paper"
      aria-label={`Invoice ${invoice.invoice_no}`}
      className="w-88 max-w-full rounded border border-line bg-raised px-5 py-5 text-fg"
    >
      <header className="text-center">
        {seller && (
          <>
            <p className="text-lg font-semibold">{seller.legal_name}</p>
            {seller.name !== seller.legal_name && <p className="text-sm text-fg-2">{seller.name}</p>}
            {seller.registration && (
              <p className="mt-1 font-mono text-sm">
                {registrationText(seller.tax_profile)} {seller.registration}
              </p>
            )}
          </>
        )}
        <p className="mt-3 text-meta font-semibold uppercase text-fg-muted">{invoiceTitle(seller?.tax_profile ?? "")}</p>
        {/* Under the title, in the rule's words (CGST Rule 5(1)(g), ADR-401). */}
        {declarationOf(seller?.tax_profile ?? "") && <p className="text-sm text-fg-2">{declarationOf(seller?.tax_profile ?? "")}</p>}
        <p className="font-mono text-base">{invoice.invoice_no}</p>
        <p className="font-mono text-sm text-fg-2">{issuedAtText(invoice.issued_at, invoice.time_zone)}</p>
        {seller?.state_code && <p className="text-sm text-fg-2">Place of supply: {seller.state_code}</p>}
        {invoice.reprints > 0 && (
          <p className="mt-2 text-sm font-semibold text-fg-2">
            Reprinted {invoice.reprints} {invoice.reprints === 1 ? "time" : "times"}. A copy carries the same number.
          </p>
        )}
      </header>

      {p ? (
        <>
          <ol className="mt-4 border-t border-dashed border-line pt-2">
            {p.lines.map((l) => (
              <li key={l.line_id} className="py-1.5">
                <div className="flex items-baseline justify-between gap-3 text-sm">
                  <span className="min-w-0">
                    <span className="font-mono">{l.qty}×</span> {l.name}
                  </span>
                  <Money amount={l.gross} />
                </div>
                {l.modifiers && l.modifiers.length > 0 && <p className="text-sm text-fg-muted">{l.modifiers.map((m) => m.name).join(", ")}</p>}
                {l.hsn && <p className="font-mono text-sm text-fg-muted">HSN/SAC {l.hsn}</p>}
                {l.discount.minor > 0 && (
                  <p className="flex justify-between text-sm text-fg-muted">
                    Discount <Money amount={negate(l.discount)} />
                  </p>
                )}
              </li>
            ))}
          </ol>
          <div className="mt-2 border-t border-dashed border-line pt-2 text-sm">
            {(p.line_discounts.minor > 0 || p.order_discounts.minor > 0) && <Row label="Items" amount={p.gross} />}
            {p.line_discounts.minor > 0 && <Row label="Line discounts" amount={negate(p.line_discounts)} />}
            {p.order_discounts.minor > 0 && <Row label="Order discount" amount={negate(p.order_discounts)} />}
            {/* Taxable value, then each tax, then the total: how a tax invoice
                reads whether the venue prices inclusive or exclusive. */}
            {invoice.tax ? (
              <>
                <Row label="Taxable value" amount={p.taxable} />
                {invoice.tax.components.map((c) => (
                  <Row key={`${c.name}-${c.rate}`} label={`${c.name} ${rateText(c.rate)}`} amount={c.amount} />
                ))}
              </>
            ) : (
              <Row label="Subtotal" amount={p.subtotal} />
            )}
          </div>
          <div className="mt-2 flex items-baseline justify-between border-t border-line-strong pt-2">
            <span className="font-serif text-xl">Total</span>
            <Money amount={p.total} variant="total" />
          </div>
        </>
      ) : (
        <p className="mt-4 text-sm text-fg-2">Issued before invoices kept their prices; the lines are on the order&apos;s history.</p>
      )}

      {payments.length > 0 && (
        <div className="mt-3 border-t border-dashed border-line pt-2 text-sm">
          {payments.map((t) => (
            <div key={t.id}>
              <Row label={`Paid · ${methodWords[t.method]}${t.reference ? ` · ${t.reference}` : ""}`} amount={t.amount} />
              {t.tip.minor > 0 && <Row label="Tip, apart from the bill" amount={t.tip} />}
            </div>
          ))}
        </div>
      )}
    </article>
  );
}

function Row({ label, amount }: { label: string; amount: Amount }) {
  return (
    <div className="flex items-baseline justify-between gap-3 py-0.5">
      <span className="min-w-0">{label}</span>
      <Money amount={amount} />
    </div>
  );
}
