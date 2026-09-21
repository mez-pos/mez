// Every primitive, on ink and on paper, side by side. Development only: main.tsx
// loads this lazily behind import.meta.env.DEV, so a production build drops it.
// Open http://localhost:5173/gallery with `npm run dev -w terminal`.

import { PartOne } from "./PartOne";
import { PartTwo } from "./PartTwo";

function Ground({ surface }: { surface: "ink" | "paper" }) {
  return (
    <div data-surface={surface} className="p-8">
      <h1 className="font-serif text-total">{surface === "ink" ? "Ink" : "Paper"}</h1>
      <PartOne />
      <PartTwo />
    </div>
  );
}

export default function Gallery() {
  return (
    <div className="grid min-h-screen grid-cols-1 xl:grid-cols-2">
      <Ground surface="ink" />
      <Ground surface="paper" />
    </div>
  );
}
