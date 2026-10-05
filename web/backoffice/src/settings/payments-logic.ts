// Where the venue is paid when a guest pays from a phone, as a form
// (backoffice-settings.md, "Paid from a phone"; ADR-215). The server checks
// everything again; these checks only spare a round trip.

import type { PaymentSettingsRequest, PaymentSettingsResponse } from "../../../shared/api-types";
import { problemWith, type Problem } from "./problem";

export interface PaymentsForm {
  address: string;
  payee: string;
}

export type PaymentsField = "upi_address" | "upi_payee";

const fields: readonly PaymentsField[] = ["upi_address", "upi_payee"];

export function paymentsFormFrom(p: PaymentSettingsResponse): PaymentsForm {
  return { address: p.upi_address, payee: p.upi_payee };
}

export function toPaymentsInput(f: PaymentsForm): PaymentSettingsRequest {
  return { upi_address: f.address.trim(), upi_payee: f.payee.trim() };
}

export function paymentsChanged(draft: PaymentsForm, saved: PaymentsForm): boolean {
  return draft.address.trim() !== saved.address.trim() || draft.payee.trim() !== saved.payee.trim();
}

/** A UPI address by its shape, as internal/domain/payment has it: a handle, an @, the bank's name. */
const UPI_ADDRESS = /^[A-Za-z0-9._-]{2,256}@[A-Za-z][A-Za-z0-9]{1,63}$/;

export function paymentsProblem(f: PaymentsForm, maxPayee: number): Problem<PaymentsField> | null {
  const address = f.address.trim();
  if (address !== "" && !UPI_ADDRESS.test(address)) {
    return { field: "upi_address", message: "Enter a UPI address like cafe@okbank: a name, an @, and the bank's handle." };
  }
  if ([...f.payee.trim()].length > maxPayee) {
    return { field: "upi_payee", message: `Keep the name to ${maxPayee} characters.` };
  }
  return null;
}

export function paymentsProblemOf(err: unknown): Problem<PaymentsField> {
  return problemWith(err, fields);
}

/** The name a guest's UPI app will show: the one typed, or the venue's. */
export function payeeShown(f: PaymentsForm, venueName: string): string {
  return f.payee.trim() || venueName;
}

/** What the card says about where things stand, under the fields. */
export function paymentsStanding(saved: PaymentsForm, currency: string): string {
  if (currency !== "INR") {
    return `UPI pays in rupees only, and this venue trades in ${currency}: a guest's bill will not offer it, whatever is entered here.`;
  }
  if (saved.address.trim() === "") return "No address saved: a guest's bill offers no UPI, and says to pay staff or at the counter.";
  return `A guest's bill offers "Pay by UPI" to ${saved.address.trim()}.`;
}
