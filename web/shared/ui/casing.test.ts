import { describe, expect, it } from "vitest";

// Every component of the three apps and the shared kit, as text.
const sources = import.meta.glob(
  ["../../terminal/src/**/*.tsx", "../../backoffice/src/**/*.tsx", "../../shared/**/*.tsx", "../../guest/src/**/*.tsx", "!**/*.test.*"],
  {
    query: "?raw",
    import: "default",
    eager: true,
  },
) as Record<string, string>;

// The brand's lines, drawn in the script face as the reference designs draw
// them: display art, not labels (ADR-407).
const brandLines = new Set([
  "Good Food,",
  "Good Food",
  "Brighter Days ♡",
  "Great Food,",
  "Great Food",
  "Brighter Days",
  "Happier People ♡",
  "Good Teams,",
  "Great Service ♡",
  "Good Teams, Great Service ♡",
  "Good Food, Happier People ♡",
  "Same Tables.",
  "New Stories.",
  "New Stories. ♡",
]);

// Words that keep their capital wherever they stand.
const proper = new Set([
  "mez",
  "Cloud",
  "GST",
  "GSTIN",
  "UPI",
  "VAT",
  "HSN",
  "SAC",
  "PIN",
  "QR",
  "USB",
  "IP",
  "OK",
  "Z-report",
  "Z-reports",
  "India",
  "UK",
  "US",
]);
const small = new Set(["a", "an", "and", "or", "of", "to", "the", "in", "on", "at", "for", "by", "with", "as", "is", "it"]);

/** A label in Title Case: two or more words that matter, each capitalised, past the first not a name. */
function titleCase(text: string): boolean {
  const words = text.match(/[A-Za-z][A-Za-z'-]*/g) ?? [];
  const big = words.filter((w) => !small.has(w.toLowerCase()));
  if (big.length < 2 || big.some((w) => w[0] !== w[0]?.toUpperCase())) return false;
  return big.slice(1).some((w) => !proper.has(w) && w !== w.toUpperCase());
}

describe("one casing (TECH-24, ADR-407)", () => {
  it("every label and heading the apps draw is in sentence case", () => {
    const found = Object.entries(sources).flatMap(([file, text]) =>
      [...text.matchAll(/>\s*([A-Z][^<>{}\n]{2,80}?)\s*</g)]
        .map((m) => (m[1] ?? "").trim())
        .filter((t) => !brandLines.has(t) && titleCase(t))
        .map((t) => `${file.replace(/^\.\.\/\.\.\//, "")}: ${t}`),
    );
    expect(found).toEqual([]);
  });
});
