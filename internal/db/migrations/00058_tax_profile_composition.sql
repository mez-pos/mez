-- +goose NO TRANSACTION
-- +goose Up
-- The GST composition scheme is a tax profile of its own (phase 17, Task 28,
-- ADR-401): gst_in_composition, a venue registered for GST that charges no
-- tax and issues a bill of supply. venues.tax_profile's CHECK lists the codes
-- the column may hold, and SQLite cannot change a CHECK, so the table is built
-- again, column for column, with the one code added to the list.
--
-- It follows the procedure SQLite documents for this (lang_altertable.html,
-- "Making Other Kinds Of Table Schema Changes"): foreign keys off, the copy in
-- one transaction, foreign keys on. They are off because nearly every table
-- refers to venues with ON DELETE CASCADE, and dropping venues with them on
-- would delete every row of every venue. PRAGMA foreign_keys does nothing
-- inside a transaction, so this file runs outside goose's; it runs on the
-- writer, a single connection kept for life, so every statement here runs on
-- the same one. The rows keep their ids, so every reference holds after.
PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE venues_new (
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

    -- internal/domain/tax's codes (tax.CodeNone and the rest).
    tax_profile                TEXT    NOT NULL DEFAULT 'none',
    tax_registration_id        TEXT    NOT NULL DEFAULT '',

    -- Cash rounding, in minor units of the venue's currency: 50 settles a bill
    -- to the nearest fifty paise, 1 rounds nothing. money.RoundToNearest
    -- applies it, once, at the end.
    cash_rounding_step_minor   INTEGER NOT NULL DEFAULT 1,

    created_at                 TIMESTAMP NOT NULL,
    updated_at                 TIMESTAMP NOT NULL,

    CHECK (length(name) > 0),
    CHECK (length(currency) = 3 AND currency = upper(currency)),
    CHECK (length(timezone) > 0),
    CHECK (financial_year_start_month BETWEEN 1 AND 12),
    CHECK (tax_profile IN ('none', 'gst_in', 'gst_in_composition', 'vat_uk', 'sales_tax_us')),
    CHECK (typeof(cash_rounding_step_minor) = 'integer' AND cash_rounding_step_minor > 0),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

INSERT INTO venues_new (id, public_id, name, legal_name, currency, timezone, financial_year_start_month,
    tax_profile, tax_registration_id, cash_rounding_step_minor, created_at, updated_at)
SELECT id, public_id, name, legal_name, currency, timezone, financial_year_start_month,
    tax_profile, tax_registration_id, cash_rounding_step_minor, created_at, updated_at
FROM venues;

DROP TABLE venues;
ALTER TABLE venues_new RENAME TO venues;
CREATE UNIQUE INDEX venues_public_id ON venues (public_id);

COMMIT;

PRAGMA foreign_keys = ON;

-- +goose Down
-- Back to the four codes, for a developer: startup only goes up. A venue on
-- the composition scheme cannot be kept under a CHECK that does not list it,
-- so move it to another profile first. While one is left, the INSERT below
-- fails on its row; SQLite ends that statement but leaves the transaction
-- open, with foreign keys off, until the process exits and rolls it back.
PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE venues_old (
    id                         INTEGER PRIMARY KEY,
    public_id                  TEXT    NOT NULL,
    name                       TEXT    NOT NULL,
    legal_name                 TEXT    NOT NULL DEFAULT '',
    currency                   TEXT    NOT NULL,
    timezone                   TEXT    NOT NULL,
    financial_year_start_month INTEGER NOT NULL DEFAULT 4,
    tax_profile                TEXT    NOT NULL DEFAULT 'none',
    tax_registration_id        TEXT    NOT NULL DEFAULT '',
    cash_rounding_step_minor   INTEGER NOT NULL DEFAULT 1,
    created_at                 TIMESTAMP NOT NULL,
    updated_at                 TIMESTAMP NOT NULL,

    CHECK (length(name) > 0),
    CHECK (length(currency) = 3 AND currency = upper(currency)),
    CHECK (length(timezone) > 0),
    CHECK (financial_year_start_month BETWEEN 1 AND 12),
    CHECK (tax_profile IN ('none', 'gst_in', 'vat_uk', 'sales_tax_us')),
    CHECK (typeof(cash_rounding_step_minor) = 'integer' AND cash_rounding_step_minor > 0),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

INSERT INTO venues_old (id, public_id, name, legal_name, currency, timezone, financial_year_start_month,
    tax_profile, tax_registration_id, cash_rounding_step_minor, created_at, updated_at)
SELECT id, public_id, name, legal_name, currency, timezone, financial_year_start_month,
    tax_profile, tax_registration_id, cash_rounding_step_minor, created_at, updated_at
FROM venues;

DROP TABLE venues;
ALTER TABLE venues_old RENAME TO venues;
CREATE UNIQUE INDEX venues_public_id ON venues (public_id);

COMMIT;

PRAGMA foreign_keys = ON;
