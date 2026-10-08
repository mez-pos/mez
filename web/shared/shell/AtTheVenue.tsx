// What a screen says in the place of something that is done at the venue and
// nowhere else, when the back office is open through the relay (ADR-308,
// ADR-309). One sentence, the same everywhere, so it is learned once. The
// server refuses the thing either way; this is so nobody fills in a form to
// be told afterwards.

import { MapPinIcon } from "lucide-react";
import { cx } from "../ui/cx";

/** The server's own words for a refused remote request, and the default here. */
export const atTheVenueText = "This can only be done at the venue.";

interface AtTheVenueProps {
  /** What it is that waits for the venue, when the default sentence would not say. */
  children?: string;
  className?: string;
}

export function AtTheVenue({ children = atTheVenueText, className }: AtTheVenueProps) {
  return (
    <p className={cx("flex items-center gap-2 text-base text-fg-2", className)}>
      <MapPinIcon aria-hidden="true" size={17} className="shrink-0 text-fg-muted" />
      {children}
    </p>
  );
}
