// The primitives. Every screen after phase 05 is assembled from these
// (docs/design/components.md). Nothing here may depend on React internals:
// guest renders them through preact/compat (ADR-064).
//
// DataTable is back office only and is imported from "ui/DataTable", not
// from here, so nothing guest loads can pull it in.
export { Button, type ButtonProps, type ButtonSize, type ButtonVariant } from "./Button";
export { Card, type CardProps } from "./Card";
export { EmptyState, type EmptyStateProps } from "./EmptyState";
export { ErrorBoundary } from "./ErrorBoundary";
export { Logo, type LogoProps } from "./Logo";
export { Meta, type MetaProps } from "./Meta";
export { Confirm, Modal, type ConfirmProps, type ModalProps } from "./Modal";
export { Money, type MoneyProps } from "./Money";
export { NumPad, type NumPadProps } from "./NumPad";
export { applyAmountKey, applyPinKey, type NumKey } from "./numpad-keys";
export { Qty, type QtyProps } from "./Qty";
export { SearchField, type SearchFieldProps } from "./SearchField";
export { Sheet, type SheetProps } from "./Sheet";
export { StatusChip, type StatusChipProps } from "./StatusChip";
export { Timeline, type TimelineProps } from "./Timeline";
export { ToastProvider, useToast, type ShowToast } from "./Toast";
export { CloseIcon, InboxIcon, SearchIcon } from "./icons";
export { focusButton, focusField, selectedRing } from "./focus";
export type { ChipStatus, ChipTone, KitchenStatus, RemoteState } from "./status";
export type { ToastTone } from "./toast-queue";
