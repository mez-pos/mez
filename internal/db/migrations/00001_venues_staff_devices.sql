-- The first migration: who the venue is, who works there, and what they work on.
--
-- Conventions this file establishes, and every later migration follows:
--
--   Money      INTEGER minor units, never REAL and never a decimal string. The
--              currency comes from the venue, so a monetary column needs no
--              sibling currency column unless it can differ from the venue's.
--              SQLite's INTEGER affinity stores 12.5 as a REAL rather than
--              refusing it, so every money column also carries a typeof check.
--              That is the invariant in CLAUDE.md, enforced by the database
--              rather than only by the code above it.
--
--   Time       TEXT, ISO 8601, UTC, ending in Z. The GLOB check refuses a
--              local time or a bare epoch. There are no SQL defaults on these:
--              docs/03-conventions.md injects a clock into the code, and a
--              column defaulting to now() would quietly go around it.
--
--   Booleans   INTEGER 0 or 1, with a check. SQLite has no boolean type and
--              will happily store the text 'false', which is truthy.
--
--   Keys       INTEGER PRIMARY KEY, the rowid alias, without AUTOINCREMENT:
--              nothing here is ever deleted, so rowid reuse cannot happen, and
--              AUTOINCREMENT costs a second table and a write per insert.
--              Anything a person sees is a public_id from internal/domain/id,
--              and every one of those has a unique index. ADR-034 is explicit
--              that the generator cannot promise uniqueness on its own: forty
--              bits is a redraw on conflict, and this is the constraint that
--              makes the conflict happen.

-- +goose Up

CREATE TABLE venues (
    id                         INTEGER PRIMARY KEY,
    public_id                  TEXT    NOT NULL,
    name                       TEXT    NOT NULL,
    legal_name                 TEXT    NOT NULL DEFAULT '',

    -- The venue's currency. Every money column in the schema is in this
    -- currency unless it says otherwise. Three uppercase letters, matching
    -- internal/domain/money.
    currency                   TEXT    NOT NULL,

    -- IANA name, for rendering. Stored times stay UTC.
    timezone                   TEXT    NOT NULL,

    -- The month the financial year starts in: 4 in India, 1 in much of the
    -- world. Invoice series restart on this boundary.
    financial_year_start_month INTEGER NOT NULL DEFAULT 4,

    tax_profile                TEXT    NOT NULL DEFAULT 'none',
    tax_registration_id        TEXT    NOT NULL DEFAULT '',

    -- Cash rounding, in minor units of the venue's currency: 50 settles a bill
    -- to the nearest fifty paise, 1 rounds nothing. money.RoundToNearest
    -- applies it, once, at the end.
    cash_rounding_step_minor   INTEGER NOT NULL DEFAULT 1,

    created_at                 TEXT    NOT NULL,
    updated_at                 TEXT    NOT NULL,

    CHECK (length(name) > 0),
    CHECK (length(currency) = 3 AND currency = upper(currency)),
    CHECK (length(timezone) > 0),
    CHECK (financial_year_start_month BETWEEN 1 AND 12),
    CHECK (tax_profile IN ('none', 'gst_in', 'vat_uk', 'sales_tax_us')),
    CHECK (typeof(cash_rounding_step_minor) = 'integer' AND cash_rounding_step_minor > 0),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*Z'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*Z')
);

CREATE UNIQUE INDEX venues_public_id ON venues (public_id);

-- Settings the schema has no opinion about: receipt footer, tip rule, printer
-- addresses. Anything the database must constrain, join on or compute with is
-- a column on venues instead, which is why currency and the rounding step are
-- up there and not in here.
CREATE TABLE settings (
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    key        TEXT    NOT NULL,
    value      TEXT    NOT NULL,
    updated_at TEXT    NOT NULL,

    PRIMARY KEY (venue_id, key),

    CHECK (length(key) > 0),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*Z')
) WITHOUT ROWID;

-- What a staff member is allowed to do. Per venue, so one venue renaming
-- "supervisor" does not rename it for another on a shared install.
CREATE TABLE roles (
    id          INTEGER PRIMARY KEY,
    venue_id    INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,

    -- A JSON array of permission keys. The set of keys lives in Go, where it
    -- can be exhaustive; the database only insists this is valid JSON.
    permissions TEXT    NOT NULL DEFAULT '[]',

    created_at  TEXT    NOT NULL,

    CHECK (length(name) > 0),
    CHECK (json_valid(permissions)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*Z')
);

CREATE UNIQUE INDEX roles_venue_name ON roles (venue_id, name);

CREATE TABLE staff (
    id         INTEGER PRIMARY KEY,
    public_id  TEXT    NOT NULL,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,

    -- Restricted, not cascading: deleting a role that people still hold would
    -- silently unassign them, and an unassigned cashier is one who can either
    -- do everything or nothing depending on how the code reads it.
    role_id    INTEGER NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,

    name       TEXT    NOT NULL,

    -- The PIN is hashed, never stored. A four digit PIN is weak by
    -- construction, so the hash is what stops a stolen database file from
    -- being a list of everyone's PIN.
    pin_hash   TEXT    NOT NULL,

    active     INTEGER NOT NULL DEFAULT 1,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL,

    CHECK (length(name) > 0),
    CHECK (length(pin_hash) > 0),
    CHECK (active IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*Z'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*Z')
);

CREATE UNIQUE INDEX staff_public_id ON staff (public_id);
CREATE INDEX staff_venue_active ON staff (venue_id, active);

-- A terminal, a kitchen screen or a back office browser that has been paired
-- with this venue.
CREATE TABLE devices (
    id           INTEGER PRIMARY KEY,
    public_id    TEXT    NOT NULL,
    venue_id     INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    name         TEXT    NOT NULL,
    kind         TEXT    NOT NULL,

    -- Hashed, for the same reason as a PIN. The device keeps the secret; this
    -- row only proves one was issued.
    token_hash   TEXT    NOT NULL DEFAULT '',

    paired_at    TEXT,
    last_seen_at TEXT,

    -- Set rather than deleted, so a device that walked out of the building
    -- leaves a record that it was here.
    revoked_at   TEXT,

    created_at   TEXT    NOT NULL,

    CHECK (length(name) > 0),
    CHECK (kind IN ('terminal', 'kitchen', 'back_office', 'guest')),
    CHECK (paired_at    IS NULL OR paired_at    GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*Z'),
    CHECK (last_seen_at IS NULL OR last_seen_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*Z'),
    CHECK (revoked_at   IS NULL OR revoked_at   GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*Z'),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*Z')
);

CREATE UNIQUE INDEX devices_public_id ON devices (public_id);
CREATE INDEX devices_venue_kind ON devices (venue_id, kind);

-- +goose Down

DROP TABLE devices;
DROP TABLE staff;
DROP TABLE roles;
DROP TABLE settings;
DROP TABLE venues;
