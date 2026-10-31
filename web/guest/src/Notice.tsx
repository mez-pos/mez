// A line the guest has to read once: a dish in their cart can no longer be
// ordered. It stays until dismissed, since a toast that fades is how someone
// finds out at the counter instead.

interface NoticeProps {
  text: string;
  onDismiss: () => void;
}

export function Notice({ text, onDismiss }: NoticeProps) {
  if (text === "") return null;
  return (
    <div role="status" className="mt-3 flex items-start gap-3 rounded-card border border-chip-warning-border bg-chip-warning-bg p-4 text-chip-warning-fg">
      <p className="min-w-0 flex-1 text-base">{text}</p>
      <button
        type="button"
        onClick={onDismiss}
        className="-my-2 -mr-2 min-h-tap shrink-0 rounded-pill px-3 text-base font-bold underline outline-focus focus-visible:outline-3"
      >
        OK
      </button>
    </div>
  );
}
