// The few icons the primitives need, as inline SVG in currentColor. Glyphs
// like ✕ or ▲ are not in the subset fonts (web/shared/fonts), and an icon
// library would cost guest more than these five do.

import type { SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement>;

function Icon({ children, ...rest }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" {...rest}>
      {children}
    </svg>
  );
}

export const CloseIcon = (p: IconProps) => (
  <Icon {...p}>
    <path d="M6 6l12 12M18 6L6 18" />
  </Icon>
);

export const BackspaceIcon = (p: IconProps) => (
  <Icon {...p}>
    <path d="M21 5H9l-6 7 6 7h12z" />
    <path d="M17 9l-5 6M12 9l5 6" />
  </Icon>
);

export const SearchIcon = (p: IconProps) => (
  <Icon {...p}>
    <circle cx="11" cy="11" r="7" />
    <path d="M20 20l-4-4" />
  </Icon>
);

export const SortIcon = ({ state, ...p }: Omit<IconProps, "direction"> & { state: "asc" | "desc" | null }) => (
  <Icon width="14" height="14" {...p}>
    {state !== "desc" && <path d="M7 10l5-5 5 5" opacity={state === "asc" ? 1 : 0.35} />}
    {state !== "asc" && <path d="M7 14l5 5 5-5" opacity={state === "desc" ? 1 : 0.35} />}
  </Icon>
);

export const InboxIcon = (p: IconProps) => (
  <Icon width="32" height="32" strokeWidth="1.5" {...p}>
    <path d="M3 13l3-8h12l3 8v6H3z" />
    <path d="M3 13h5l1 3h6l1-3h5" />
  </Icon>
);
