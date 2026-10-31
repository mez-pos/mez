// Arrow keys for a group of radios drawn as buttons (TECH-20, ADR-403), the
// way the WAI-ARIA radio group moves. The group is one tab stop, on the
// chosen radio (the first when none is chosen); Right and Down choose the
// next, Left and Up the one before, wrapping; Home and End the ends. A
// disabled radio is passed over. Choosing is the radio's own click, so an
// arrow does exactly what a tap would.
//
// Spread on the element with role="radiogroup":
//   <div role="radiogroup" aria-label="Payment method" {...radioGroup}>

import type { KeyboardEvent } from "react";

/**
 * Where a key moves among `count` radios from the one at `at` (-1 for none),
 * passing over those `skip` refuses; null for any other key, or when there is
 * nowhere else to go.
 */
export function radioStep(key: string, at: number, count: number, skip: (i: number) => boolean = () => false): number | null {
  let dir: 1 | -1;
  let from: number;
  switch (key) {
    case "ArrowRight":
    case "ArrowDown":
      [dir, from] = [1, at];
      break;
    case "ArrowLeft":
    case "ArrowUp":
      [dir, from] = [-1, at < 0 ? count : at];
      break;
    case "Home":
      [dir, from] = [1, -1];
      break;
    case "End":
      [dir, from] = [-1, count];
      break;
    default:
      return null;
  }
  for (let n = 1; n <= count; n++) {
    const i = (((from + dir * n) % count) + count) % count;
    if (skip(i)) continue;
    return i === at ? null : i;
  }
  return null;
}

const isOff = (el: HTMLElement): boolean => el.hasAttribute("disabled") || el.getAttribute("aria-disabled") === "true";

const radiosIn = (group: HTMLElement): HTMLElement[] => Array.from(group.querySelectorAll<HTMLElement>('[role="radio"]'));

/** One tab stop for the group: the chosen radio, else the first that can be chosen. */
function rove(group: HTMLElement): void {
  const radios = radiosIn(group);
  const stop = radios.find((r) => r.getAttribute("aria-checked") === "true" && !isOff(r)) ?? radios.find((r) => !isOff(r));
  for (const r of radios) {
    const index = r === stop ? "0" : "-1";
    if (r.getAttribute("tabindex") !== index) r.setAttribute("tabindex", index);
  }
}

const watched = new WeakSet<HTMLElement>();

/**
 * Keeps the tab stop on the chosen radio as the choice changes, whoever
 * changes it. It watches the group rather than asking each radio for a
 * tabIndex, so a group takes it in one spread.
 */
function watch(group: HTMLElement | null): void {
  if (!group || watched.has(group)) return;
  watched.add(group);
  rove(group);
  new MutationObserver(() => rove(group)).observe(group, {
    subtree: true,
    childList: true,
    attributes: true,
    attributeFilter: ["aria-checked", "disabled", "aria-disabled"],
  });
}

function onKeyDown(event: KeyboardEvent<HTMLElement>): void {
  // With a modifier the key is the browser's or the system's: Alt+Left is Back.
  if (event.altKey || event.ctrlKey || event.metaKey) return;
  const radios = radiosIn(event.currentTarget);
  const focused = radios.findIndex((r) => r === document.activeElement);
  const at = focused >= 0 ? focused : radios.findIndex((r) => r.getAttribute("aria-checked") === "true");
  const next = radioStep(event.key, at, radios.length, (i) => isOff(radios[i] as HTMLElement));
  if (next === null) return;
  event.preventDefault();
  const radio = radios[next] as HTMLElement;
  radio.focus();
  radio.click();
}

/** Spread on a role="radiogroup" element of radio buttons: arrow keys, and one tab stop. */
export const radioGroup = { ref: watch, onKeyDown } as const;
