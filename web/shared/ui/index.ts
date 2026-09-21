// The primitives. Every screen after phase 05 is assembled from these
// (docs/design/components.md). Nothing here may depend on React internals:
// guest renders them through preact/compat (ADR-064).
export { Button, type ButtonProps, type ButtonSize, type ButtonVariant } from "./Button";
export { Card, type CardProps } from "./Card";
export { Meta, type MetaProps } from "./Meta";
export { Money, type MoneyProps } from "./Money";
export { Qty, type QtyProps } from "./Qty";
export { StatusChip, type StatusChipProps } from "./StatusChip";
export type { ChipStatus, ChipTone, KitchenStatus } from "./status";
