// Joins class names, skipping the falsy ones. Twelve bytes of clsx, without
// the dependency: guest counts every import.
export function cx(...classes: (string | false | null | undefined)[]): string {
  return classes.filter(Boolean).join(" ");
}
