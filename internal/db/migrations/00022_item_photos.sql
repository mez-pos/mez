-- +goose Up
-- Item photos (phase 10, Task 2d2, ADR-110). The photo is a file in the
-- data directory's photos folder, named by the SHA-256 of its bytes; this
-- column holds that name, or '' for none. The name is its own cache key:
-- a new photo is a new name, so a terminal never shows a stale one.

ALTER TABLE items ADD COLUMN photo TEXT NOT NULL DEFAULT ''
    CHECK (photo = '' OR (length(photo) = 68 AND photo GLOB '*.jpg' AND substr(photo, 1, 64) NOT GLOB '*[^0-9a-f]*'));

-- +goose Down
ALTER TABLE items DROP COLUMN photo;
