-- +goose Up
-- The floor plan (phase 08, Task 1, ADR-085): the venue's areas and where
-- its tables stand in them. Layout only: whether a table is free, seated or
-- dirty is a projection of its orders, from Task 2, and is not stored here.

CREATE TABLE areas (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT 1,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    UNIQUE (venue_id, name),
    CHECK (length(name) > 0),
    CHECK (active IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- A table is retired, never deleted: an order that sat at it keeps its id.
-- Its name is unique across the venue, not the area, because staff say
-- "table 4" without saying where.
CREATE TABLE tables (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    area_id    INTEGER NOT NULL REFERENCES areas (id),
    name       TEXT    NOT NULL,
    seats      INTEGER NOT NULL,
    shape      TEXT    NOT NULL,

    -- The cell it stands in on its area's grid, from the top left. A table
    -- takes one cell whatever its shape; the grid's size is the service's.
    x          INTEGER NOT NULL,
    y          INTEGER NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT 1,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    UNIQUE (venue_id, name),
    CHECK (length(name) > 0),
    CHECK (typeof(seats) = 'integer' AND seats > 0),
    CHECK (shape IN ('square', 'round', 'long')),
    CHECK (typeof(x) = 'integer' AND x >= 0),
    CHECK (typeof(y) = 'integer' AND y >= 0),
    CHECK (active IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- One table in service per cell. A retired table gives its cell up.
CREATE UNIQUE INDEX tables_cell ON tables (area_id, x, y) WHERE active = 1;
CREATE INDEX tables_venue ON tables (venue_id, area_id);

-- +goose Down
DROP INDEX tables_venue;
DROP INDEX tables_cell;
DROP TABLE tables;
DROP TABLE areas;
