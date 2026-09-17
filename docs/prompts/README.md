# Prompts

One file per phase. Open `phase-NN.prompt.md`, copy the fenced block, paste it
as the first message of a new Claude Code chat. That is the whole ritual.

## The loop

1. New chat.
2. Paste the current phase's prompt block.
3. Claude reads `docs/STATE.md`, reads the phase file, does **one task**,
   runs `make check`, updates `STATE.md`, commits, stops.
4. You read the diff and `STATE.md`.
5. New chat. Repeat.

One task per chat. Do not let a session run on into a second task even if it
offers — the context is degraded by then and that is where silent mistakes get
made.

## When to intervene

**Start a fresh chat immediately if** the session starts re-reading files it
already read, contradicts a decision in `DECISIONS.md`, or proposes changing the
stack.

**Stop and do it yourself if** the task involves a migration against data you
care about, or the money and invoice code in Phase 03.

**Use Opus, not Sonnet, for:** Phase 03 entirely, Phase 04 Task 1, and Phase 07
Tasks 2 and 3. These are the places where a subtle error costs real money and
will not surface until a cafe is using it.

## If a session goes wrong

`git reset --hard` to the last good commit, fix the task description in the
phase file to be more specific, start a fresh chat. Do not try to argue a
confused session back on track — it is cheaper to restart.

## Reviewing between sessions

Thirty seconds per session, on `STATE.md` and the diff. Check: does the next
task still make sense, did anything get added that is not in the phase file,
and does the commit message match what actually changed.
