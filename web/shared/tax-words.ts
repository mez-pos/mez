// What a venue's tax registration is called under its tax profile, in every
// app that shows the number: the back office's tax card and the guest's
// receipt (GST-15). The printed receipt names it the same way in Go
// (internal/device/escpos).

/** What the registration number is called under a profile ("gst_in" is "GSTIN"), or null when it has none. */
export function registrationLabel(profile: string): string | null {
  switch (profile) {
    case "gst_in":
      return "GSTIN";
    case "vat_uk":
      return "VAT number";
    default:
      return null;
  }
}
