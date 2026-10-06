// Where the venue is paid when a guest pays from a phone, and when they
// pay, as a form (backoffice-settings.md, "Paid from a phone"; ADR-215,
// ADR-229). The server checks everything again; these checks only spare a
// round trip.

import type { PayMode, PaymentSettingsRequest, PaymentSettingsResponse } from "../../../shared/api-types";
import { problemWith, type Problem } from "./problem";

export interface PaymentsForm {
  address: string;
  payee: string;
  /** When a guest who orders from their phone pays (ADR-225). */
  mode: PayMode;
}

export type PaymentsField = "upi_address" | "upi_payee" | "mode";

const fields: readonly PaymentsField[] = ["upi_address", "upi_payee", "mode"];

/** The two times a guest can pay, in the order the card offers them. */
export const PAY_MODES: readonly { value: PayMode; label: string }[] = [
  { value: "pay_at_table", label: "When they are ready to leave" },
  { value: "prepay", label: "Before the order is sent" },
];

export function paymentsFormFrom(p: PaymentSettingsResponse): PaymentsForm {
  // A server from before the mode says nothing of it, and is paid at the table.
  return { address: p.upi_address, payee: p.upi_payee, mode: p.mode === "prepay" ? "prepay" : "pay_at_table" };
}

export function toPaymentsInput(f: PaymentsForm): PaymentSettingsRequest {
  return { upi_address: f.address.trim(), upi_payee: f.payee.trim(), mode: f.mode };
}

export function paymentsChanged(draft: PaymentsForm, saved: PaymentsForm): boolean {
  return draft.address.trim() !== saved.address.trim() || draft.payee.trim() !== saved.payee.trim() || draft.mode !== saved.mode;
}

export interface ModeWords {
  /** Paying first chosen where nobody can pay online: it does nothing, and is said as a warning. */
  readonly inert: boolean;
  readonly text: string;
}

/**
 * What the chosen time to pay does, for the owner choosing it. canCharge is
 * whether the venue has a payment gateway: without one, paying first is
 * kept and changes nothing (ADR-225).
 */
export function modeWords(mode: PayMode, canCharge: boolean): ModeWords {
  if (mode !== "prepay") {
    return {
      inert: false,
      text: "A guest's first order waits for a member of staff to accept it on the terminal. They pay when they are ready to leave, as they would with a waiter.",
    };
  }
  if (!canCharge) {
    return {
      inert: true,
      text: "This has no effect yet: this venue has no payment gateway, so a guest cannot pay online. Orders still wait for staff, and guests pay when they leave. A gateway is set in the venue's configuration, not on this screen.",
    };
  }
  return {
    inert: false,
    text: "A guest pays online for each order as they place it. Nothing reaches the kitchen until it is paid for, and a paid order goes straight there: staff do not accept it first.",
  };
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
