// The numbering half of "Tax and numbering" (docs/design/screens/backoffice-settings.md):
// how invoices and credit notes are numbered, the financial year they restart
// in, and the cash rounding step, as a form. The server checks everything and
// previews what a draft would write next; what is checked here is only what
// costs nothing. No React in this file, so all of it is tested directly.

import type { NumberingRequest, NumberingResponse } from "../../../shared/api-types";
import { majorText, parseMajor } from "./money-text";
import { problemWith, type Problem } from "./problem";

export const MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"] as const;

export interface NumberingForm {
  invoiceSeries: string;
  creditSeries: string;
  invoicePrefix: string;
  creditPrefix: string;
  template: string;
  /** 1 to 12. */
  month: number;
  /** In major units, as typed: "0.50". */
  step: string;
}

export function numberingFormFrom(n: NumberingResponse): NumberingForm {
  return {
    invoiceSeries: n.invoice_series,
    creditSeries: n.credit_series,
    invoicePrefix: n.invoice_prefix,
    creditPrefix: n.credit_prefix,
    template: n.template,
    month: n.financial_year_start_month,
    step: stepText(n.cash_rounding_step, n.max_rounding_step),
  };
}

/** A rounding step in minor units as major units: 50 paise is "0.50". */
export const stepText = majorText;

/** The step the owner typed, in minor units, or null when it is not between the smallest unit and one whole unit. */
export function parseStep(text: string, maxStep: number): number | null {
  const minor = parseMajor(text, maxStep);
  return minor !== null && minor >= 1 && minor <= maxStep ? minor : null;
}

/** The range of steps in words: "from 0.01 to 1.00". */
export const stepRange = (maxStep: number): string => `from ${stepText(1, maxStep)} to ${stepText(maxStep, maxStep)}`;

export type NumberingField =
  "invoice_series" | "credit_series" | "invoice_prefix" | "credit_prefix" | "template" | "financial_year_start_month" | "cash_rounding_step";

const numberingFields: readonly NumberingField[] = [
  "invoice_series",
  "credit_series",
  "invoice_prefix",
  "credit_prefix",
  "template",
  "financial_year_start_month",
  "cash_rounding_step",
];

export const numberingProblemOf = (err: unknown): Problem<NumberingField> => problemWith(err, numberingFields);

/** What can be refused without asking the server. */
export function numberingProblem(f: NumberingForm, maxStep: number): Problem<NumberingField> | null {
  if (parseStep(f.step, maxStep) === null) {
    return { field: "cash_rounding_step", message: `Enter a step ${stepRange(maxStep)} (the smallest rounds nothing).` };
  }
  return null;
}

/** What Save and the preview send. A prefix left empty follows its series. */
export function toNumberingInput(f: NumberingForm, maxStep: number): NumberingRequest {
  return {
    invoice_series: f.invoiceSeries.trim().toUpperCase(),
    credit_series: f.creditSeries.trim().toUpperCase(),
    invoice_prefix: f.invoicePrefix.trim(),
    credit_prefix: f.creditPrefix.trim(),
    template: f.template.trim(),
    financial_year_start_month: f.month,
    cash_rounding_step: parseStep(f.step, maxStep) ?? 0,
  };
}

/** Whether the draft differs from what the server has. A prefix and a code in capitals are the same either way. */
export function numberingChanged(draft: NumberingForm, saved: NumberingForm): boolean {
  const norm = (f: NumberingForm) =>
    JSON.stringify([
      f.invoiceSeries.trim().toUpperCase(),
      f.creditSeries.trim().toUpperCase(),
      f.invoicePrefix.trim(),
      f.creditPrefix.trim(),
      f.template.trim(),
      f.month,
      f.step.trim(),
    ]);
  return norm(draft) !== norm(saved);
}

/**
 * A series code typed. A prefix that was following its code keeps following
 * it, the way the server stores it: the prefix is the code until the owner
 * writes another.
 */
export function withSeries(f: NumberingForm, which: "invoice" | "credit", value: string): NumberingForm {
  const code = value.toUpperCase();
  if (which === "invoice") {
    const follows = f.invoicePrefix === f.invoiceSeries || f.invoicePrefix === "";
    return { ...f, invoiceSeries: code, invoicePrefix: follows ? code : f.invoicePrefix };
  }
  const follows = f.creditPrefix === f.creditSeries || f.creditPrefix === "";
  return { ...f, creditSeries: code, creditPrefix: follows ? code : f.creditPrefix };
}

/** The key the preview is asked for: the same draft is never asked twice running. */
export const previewKey = (f: NumberingForm, maxStep: number): string => JSON.stringify(toNumberingInput(f, maxStep));
