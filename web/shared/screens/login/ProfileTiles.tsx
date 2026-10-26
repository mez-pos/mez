// The profiles to tap on the sign-in screen (D01): a round avatar with the
// person's initials, their name and their role under it, four to a page and a
// More for the rest. The design draws photographs; nobody has one yet, so the
// initials are the avatar.

import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react";
import type { StaffResponse } from "../../api-types";
import { initials, roleText } from "../../auth/format";
import { cx } from "../../ui/cx";
import { tilePage } from "./login-logic";

interface ProfileTilesProps {
  people: readonly StaffResponse[];
  selectedId: string | null;
  page: number;
  onChoose: (member: StaffResponse) => void;
  onPage: (page: number) => void;
}

export function ProfileTiles({ people, selectedId, page, onChoose, onPage }: ProfileTilesProps) {
  const view = tilePage(people, page);
  return (
    <ul className="mt-10 flex flex-wrap items-start gap-x-4 gap-y-6 2xl:gap-x-6 [@media(max-height:900px)]:mt-5">
      {view.shown.map((member) => {
        const chosen = member.id === selectedId;
        return (
          <li key={member.id}>
            <button
              type="button"
              aria-pressed={chosen}
              onClick={() => onChoose(member)}
              className="group flex w-26 flex-col items-center 2xl:w-31 rounded text-center outline-offset-4 outline-line-focus focus-visible:outline-2"
            >
              <span
                aria-hidden="true"
                className={cx(
                  "flex size-20 items-center justify-center rounded-round border-3 bg-sunken font-serif text-total text-fg-2 transition-colors 2xl:size-27.5",
                  chosen ? "border-attention bg-attention-tint text-attention-text" : "border-line-strong group-hover:bg-raised-high",
                )}
              >
                {initials(member.name)}
              </span>
              <span className="mt-3 block max-w-full truncate text-base font-bold">{member.name}</span>
              <span className="mt-0.5 block max-w-full truncate text-sm text-fg-muted">{roleText(member.role)}</span>
            </button>
          </li>
        );
      })}
      {view.prev !== null && (
        // Back a page, for a team of more than two pages: More alone went
        // round the whole way to reach the page before.
        <li>
          <button
            type="button"
            onClick={() => onPage(view.prev ?? 0)}
            aria-label={`Previous profiles, page ${view.page} of ${view.pages}`}
            className="flex w-20 flex-col items-center pt-3 text-center outline-offset-4 outline-line-focus focus-visible:outline-2 2xl:w-25 2xl:pt-6"
          >
            <span className="flex size-14 items-center justify-center rounded-round border border-line-strong bg-raised hover:bg-raised-high">
              <ChevronLeftIcon aria-hidden="true" size={24} strokeWidth={2} />
            </span>
            <span className="mt-2.5 text-sm text-fg-muted">Back</span>
          </button>
        </li>
      )}
      {view.hasMore && (
        <li>
          <button
            type="button"
            onClick={() => onPage(view.next)}
            aria-label={`More profiles, page ${view.page + 1} of ${view.pages}`}
            className="flex w-20 flex-col items-center pt-3 text-center outline-offset-4 outline-line-focus focus-visible:outline-2 2xl:w-25 2xl:pt-6"
          >
            <span className="flex size-14 items-center justify-center rounded-round border border-line-strong bg-raised hover:bg-raised-high">
              <ChevronRightIcon aria-hidden="true" size={24} strokeWidth={2} />
            </span>
            <span className="mt-2.5 text-sm text-fg-muted">More</span>
          </button>
        </li>
      )}
    </ul>
  );
}
