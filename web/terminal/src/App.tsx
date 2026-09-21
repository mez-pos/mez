import { ConnectionIndicator } from "../../shared/ConnectionIndicator";

// Shell only. Screens arrive from Task 6 onward.
export function App() {
  return (
    <div className="flex min-h-screen flex-col bg-canvas text-fg">
      <header className="flex items-center justify-between gap-4 border-b border-line bg-chrome px-6 py-2">
        <p className="text-meta font-semibold uppercase text-fg-faint">Nofiltr · Terminal</p>
        <ConnectionIndicator />
      </header>
      <main className="p-8">
        <h1 className="font-serif text-total">Terminal</h1>
        <p className="mt-4 max-w-prose text-base text-fg-2">The register arrives in phase 06.</p>
      </main>
    </div>
  );
}
