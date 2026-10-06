-- +goose Up
-- A guest's round knows which lines it became on its ticket (phase 11, Task
-- 5a, ADR-236): the seq of its first line.added. Its lines are appended
-- together, so they are that seq and the ones after it, one for each line.
-- That is what lets a guest be told how their own order stands in the
-- kitchen, on a ticket a waiter and other guests are adding to as well.
--
-- A round sent before this has none, and is told only that it was sent.
ALTER TABLE guest_rounds ADD COLUMN first_seq INTEGER
    CHECK (first_seq IS NULL OR (typeof(first_seq) = 'integer' AND first_seq >= 2 AND status = 'sent'));

-- +goose Down
ALTER TABLE guest_rounds DROP COLUMN first_seq;
