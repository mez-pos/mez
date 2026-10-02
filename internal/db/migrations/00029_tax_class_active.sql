-- +goose Up
-- A tax class can be retired (phase 10, Task 6b, ADR-131): kept for the
-- items and invoices that name it, offered to no new item. Every class
-- there is stays active.

ALTER TABLE tax_classes ADD COLUMN active BOOLEAN NOT NULL DEFAULT 1
    CHECK (active IN (0, 1));

-- +goose Down
ALTER TABLE tax_classes DROP COLUMN active;
