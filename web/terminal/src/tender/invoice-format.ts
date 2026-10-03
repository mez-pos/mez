// Words and figures an invoice preview needs that are not money. Money goes
// through <Money>; these never touch an amount.

/** A tax rate in thousandths of a percent (tax.Rate, ADR-049) as text, by string: no float. */
export function rateText(rate: number): string {
  const whole = Math.trunc(rate / 1000);
  const rest = String(Math.abs(rate % 1000))
    .padStart(3, "0")
    .replace(/0+$/, "");
  return rest ? `${whole}.${rest}%` : `${whole}%`;
}

/** What the document is called under the venue's tax profile. */
export function invoiceTitle(taxProfile: string): string {
  if (taxProfile === "gst_in") return "Tax invoice";
  if (taxProfile === "vat_uk") return "VAT receipt";
  return "Invoice";
}

/** When it was issued, in the venue's zone; the device's if the zone is unknown. */
export function issuedAtText(iso: string, timeZone: string, locale?: string): string {
  const options: Intl.DateTimeFormatOptions = { dateStyle: "short", timeStyle: "short", hourCycle: "h23" };
  const at = new Date(iso);
  try {
    return new Intl.DateTimeFormat(locale, { ...options, timeZone }).format(at);
  } catch {
    return new Intl.DateTimeFormat(locale, options).format(at);
  }
}
