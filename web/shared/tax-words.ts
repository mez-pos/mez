// What a venue's tax registration is called under its tax profile, in every
// app that shows the number: the back office's tax card and the guest's
// receipt (GST-15). The printed receipt names it the same way in Go
// (internal/device/escpos).

/** The GST composition scheme's profile code (ADR-401): a GSTIN, and no tax. */
export const compositionProfile = "gst_in_composition";

/**
 * What CGST Rule 5(1)(g) has a composition taxable person print at the top of
 * every bill of supply, in its own words (ADR-401). Go has the same words in
 * tax.CompositionDeclaration.
 */
export const compositionDeclaration = "Composition taxable person, not eligible to collect tax on supplies";

/** What the registration number is called under a profile ("gst_in" is "GSTIN"), or null when it has none. */
export function registrationLabel(profile: string): string | null {
  switch (profile) {
    case "gst_in":
    case compositionProfile:
      return "GSTIN";
    case "vat_uk":
      return "VAT number";
    default:
      return null;
  }
}

/** The line a document must print under its title: the composition scheme's declaration, or null for none. */
export function declarationOf(profile: string): string | null {
  return profile === compositionProfile ? compositionDeclaration : null;
}
