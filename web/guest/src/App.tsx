// Shell only. Screens arrive from Task 6 onward. Until then it wears the
// design's landing: the logo, a serif greeting and one script line, which is
// garnish and never a label (tokens.md).

import { Logo } from "../../shared/ui/Logo";

export function App() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 bg-canvas p-6 text-center text-fg">
      <Logo className="h-12" />
      <div className="w-full max-w-sm rounded-card border border-line bg-raised p-6 shadow-card">
        <p className="text-meta font-bold uppercase text-fg-muted">Order</p>
        <h1 className="mt-2 font-serif text-total">Good food, together</h1>
        <p className="mt-4 text-base text-fg-2">Ordering from the table arrives in phase 11.</p>
      </div>
      <p aria-hidden="true" className="font-script text-xl text-attention-text">
        Scan. Order. Relax.
      </p>
    </main>
  );
}
