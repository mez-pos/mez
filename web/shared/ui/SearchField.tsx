// Search: debounced, clearable, keyboard-dismissible (docs/design/components.md).
//
// - Typing reports after `debounceMs` of quiet, so a menu of 400 items is not
//   filtered on every keystroke of "cappuccino".
// - Clear is a full tap target, and reports the empty query at once.
// - Escape clears; Escape on an empty field leaves it. Enter reports at once
//   and leaves the field, which drops a tablet's on-screen keyboard.
// - The input's text is 16px at least (tokens.css), so iOS does not zoom.

import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { cx } from "./cx";
import { debounce } from "./debounce";
import { CloseIcon, SearchIcon } from "./icons";

export interface SearchFieldProps {
  onSearch: (query: string) => void;
  /** Names the field for screen readers, and is its placeholder. */
  label: string;
  debounceMs?: number;
  initialQuery?: string;
  className?: string;
}

export function SearchField({ onSearch, label, debounceMs = 200, initialQuery = "", className }: SearchFieldProps) {
  const [query, setQuery] = useState(initialQuery);
  const input = useRef<HTMLInputElement>(null);
  const latest = useRef(onSearch);
  latest.current = onSearch;

  const report = useMemo(() => debounce((q: string) => latest.current(q.trim()), debounceMs), [debounceMs]);
  useEffect(() => () => report.cancel(), [report]);

  const change = (next: string) => {
    setQuery(next);
    report.call(next);
  };

  const clear = () => {
    setQuery("");
    report.cancel();
    latest.current("");
    input.current?.focus();
  };

  const keyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Escape" && query !== "") {
      // Handled here, so an enclosing modal does not also close.
      event.preventDefault();
      event.stopPropagation();
      clear();
    } else if (event.key === "Escape") {
      input.current?.blur();
    } else if (event.key === "Enter") {
      report.flush();
      input.current?.blur();
    }
  };

  return (
    <div className={cx("relative flex items-center", className)}>
      <SearchIcon className="pointer-events-none absolute left-4 text-fg-muted" />
      <input
        ref={input}
        type="search"
        value={query}
        onChange={(event) => change(event.target.value)}
        onKeyDown={keyDown}
        aria-label={label}
        placeholder={label}
        enterKeyHint="search"
        autoComplete="off"
        spellCheck={false}
        className={cx(
          "h-tap w-full rounded-pill border border-line-strong bg-raised pl-11 pr-12 text-fg placeholder:text-fg-muted",
          "outline-accent-tint focus-visible:border-accent focus-visible:outline-3",
          // The browser's own clear button would be a second, smaller one.
          "[&::-webkit-search-cancel-button]:appearance-none",
        )}
      />
      {query !== "" && (
        <button
          type="button"
          aria-label="Clear search"
          onClick={clear}
          className="absolute right-0 flex size-tap items-center justify-center text-fg-muted hover:text-fg"
        >
          <CloseIcon />
        </button>
      )}
    </div>
  );
}
