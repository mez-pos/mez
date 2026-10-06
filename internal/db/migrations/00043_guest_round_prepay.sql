-- +goose Up
-- A round at a venue where guests pay first (phase 11, Task 4e5a, ADR-225).
-- It waits as a request does, but for its payment and not for staff: it is
-- not on any terminal, and it becomes lines on a ticket when it is paid.
--
-- due_minor is what the guest is asked to pay for it: what the lines add to
-- the table's bill, tax and all, as the bill itself would work it out, at
-- the moment they asked. It is kept because the answer the guest was given
-- is the amount the charge is for.
ALTER TABLE guest_rounds ADD COLUMN prepay INTEGER NOT NULL DEFAULT 0
    CHECK (prepay IN (0, 1));
ALTER TABLE guest_rounds ADD COLUMN due_minor INTEGER
    CHECK (due_minor IS NULL OR (typeof(due_minor) = 'integer' AND due_minor > 0));

-- +goose Down
ALTER TABLE guest_rounds DROP COLUMN due_minor;
ALTER TABLE guest_rounds DROP COLUMN prepay;
