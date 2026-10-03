-- +goose Up
-- A course can be retired (phase 10, Task 6e, ADR-134): kept for the items
-- that named it, offered to no new item and left off the till. Every course
-- there is stays active.

ALTER TABLE courses ADD COLUMN active BOOLEAN NOT NULL DEFAULT 1
    CHECK (active IN (0, 1));

-- +goose Down
ALTER TABLE courses DROP COLUMN active;
