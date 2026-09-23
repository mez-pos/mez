-- The menu: what a venue sells, where it is made, and what it is taxed at.
--
-- Nothing an order records ever joins back to these tables. A line copies
-- the name, price, modifiers, station, course and tax facts it was rung up
-- with ("Prices and the snapshot rule", docs/02-domain.md), so every row
-- here can change or be retired without touching a ticket already open.
-- That is also why nothing is deleted: a row stops being sold by setting
-- active to 0, and an id an old event names still means something.
--
-- Every table carries venue_id, even where a parent row already implies it.
-- The read that builds a terminal's menu is then one indexed scan per table
-- rather than a join chain, and a row can never be attached to another
-- venue's parent without the composite foreign keys below refusing it.

-- +goose Up

-- The menu's version, one row per venue. Any write to a menu table bumps
-- it in the same transaction; a terminal compares it with the one it
-- cached, and the realtime stream announces a new one. It is a table of
-- its own rather than a column on venues so that the venue queries, and
-- the Go type they return, do not change shape with it.
CREATE TABLE menu_versions (
    venue_id INTEGER PRIMARY KEY REFERENCES venues (id) ON DELETE CASCADE,
    version  INTEGER NOT NULL,

    CHECK (typeof(version) = 'integer' AND version >= 1)
);

-- A venue with no row is at version 1: its menu has never been written.

-- Where an item is made. The code is what a line snapshots and what the
-- realtime stream routes on ("station:bar"), so it is short, lowercase and
-- never changes once a line has used it.
CREATE TABLE stations (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    code       TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    active     BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    UNIQUE (venue_id, id),
    CHECK (code GLOB '[a-z]*' AND code NOT GLOB '*[^a-z0-9_-]*' AND length(code) <= 32),
    CHECK (length(name) > 0),
    CHECK (active IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE UNIQUE INDEX stations_venue_code ON stations (venue_id, code);

-- Courses, by number. A line snapshots the number, and the kitchen fires
-- course 2 when the table is done with course 1. A cafe that never courses
-- has one, and every item sits in it.
CREATE TABLE courses (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    number     INTEGER NOT NULL,
    name       TEXT    NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    UNIQUE (venue_id, id),
    CHECK (typeof(number) = 'integer' AND number >= 1),
    CHECK (length(name) > 0),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE UNIQUE INDEX courses_venue_number ON courses (venue_id, number);

-- A tax class is a rate items share: "Food 5%", "Alcohol exempt". Items
-- point at a class, so a rate change is one row, not every item. The rate
-- is a tax.Rate, thousandths of a percent (5000 is 5%), ADR-049. It means
-- what the venue's tax profile says it means; US sales tax ignores it and
-- taxes by jurisdiction.
CREATE TABLE tax_classes (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    rate       INTEGER NOT NULL,
    exempt     BOOLEAN NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    UNIQUE (venue_id, id),
    CHECK (length(name) > 0),
    CHECK (typeof(rate) = 'integer' AND rate BETWEEN 0 AND 100000),
    CHECK (exempt IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE TABLE categories (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    active     BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    UNIQUE (venue_id, id),
    CHECK (length(name) > 0),
    CHECK (active IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- An item. price is in the venue's currency, and includes tax or not as
-- the venue prices. hsn is the GST HSN or SAC code, empty where the
-- venue's profile does not use one.
CREATE TABLE items (
    id           INTEGER PRIMARY KEY,
    venue_id     INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    category_id  INTEGER NOT NULL,
    name         TEXT    NOT NULL,
    price        INTEGER NOT NULL,
    hsn          TEXT    NOT NULL DEFAULT '',
    tax_class_id INTEGER NOT NULL,
    station_id   INTEGER NOT NULL,
    course_id    INTEGER NOT NULL,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    active       BOOLEAN NOT NULL DEFAULT 1,
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL,

    UNIQUE (venue_id, id),
    FOREIGN KEY (venue_id, category_id)  REFERENCES categories (venue_id, id),
    FOREIGN KEY (venue_id, tax_class_id) REFERENCES tax_classes (venue_id, id),
    FOREIGN KEY (venue_id, station_id)   REFERENCES stations (venue_id, id),
    FOREIGN KEY (venue_id, course_id)    REFERENCES courses (venue_id, id),
    CHECK (length(name) > 0),
    CHECK (typeof(price) = 'integer' AND price >= 0),
    CHECK (hsn = '' OR (hsn NOT GLOB '*[^0-9]*' AND length(hsn) BETWEEN 4 AND 8)),
    CHECK (active IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE INDEX items_venue_category ON items (venue_id, category_id, sort_order);

-- A group of choices: "Milk", pick exactly one; "Extras", pick up to three.
-- min_select above zero makes the group required. The terminal enforces
-- the bounds; the database only refuses bounds that could never be met.
CREATE TABLE modifier_groups (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    min_select INTEGER NOT NULL DEFAULT 0,
    max_select INTEGER NOT NULL DEFAULT 1,
    active     BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    UNIQUE (venue_id, id),
    CHECK (length(name) > 0),
    CHECK (typeof(min_select) = 'integer' AND min_select >= 0),
    CHECK (typeof(max_select) = 'integer' AND max_select >= 1 AND max_select >= min_select),
    CHECK (active IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- One choice in a group. price is the change to the line's unit price, per
-- unit, and may be negative ("no cheese, less 20"). is_default preselects
-- it when the sheet opens.
CREATE TABLE modifiers (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    group_id   INTEGER NOT NULL,
    name       TEXT    NOT NULL,
    price      INTEGER NOT NULL DEFAULT 0,
    is_default BOOLEAN NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    active     BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    FOREIGN KEY (venue_id, group_id) REFERENCES modifier_groups (venue_id, id),
    CHECK (length(name) > 0),
    CHECK (typeof(price) = 'integer'),
    CHECK (is_default IN (0, 1)),
    CHECK (active IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE INDEX modifiers_venue_group ON modifiers (venue_id, group_id, sort_order);

-- Which groups an item offers, in the order the sheet shows them. A group
-- is shared: "Milk" is one group on every coffee.
CREATE TABLE item_modifier_groups (
    venue_id   INTEGER NOT NULL,
    item_id    INTEGER NOT NULL,
    group_id   INTEGER NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY (item_id, group_id),
    FOREIGN KEY (venue_id, item_id)  REFERENCES items (venue_id, id) ON DELETE CASCADE,
    FOREIGN KEY (venue_id, group_id) REFERENCES modifier_groups (venue_id, id)
) WITHOUT ROWID;

CREATE INDEX item_modifier_groups_venue ON item_modifier_groups (venue_id, item_id, sort_order);

-- +goose Down

DROP TABLE item_modifier_groups;
DROP TABLE modifiers;
DROP TABLE modifier_groups;
DROP TABLE items;
DROP TABLE categories;
DROP TABLE tax_classes;
DROP TABLE courses;
DROP TABLE stations;
DROP TABLE menu_versions;
