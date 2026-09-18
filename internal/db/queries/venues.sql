-- Venues. Explicit column lists throughout, per docs/03-conventions.md: a
-- SELECT * here would silently change shape the next time a migration adds a
-- column, and RETURNING * has exactly the same problem.

-- name: CreateVenue :one
INSERT INTO venues (
    public_id,
    name,
    legal_name,
    currency,
    timezone,
    financial_year_start_month,
    tax_profile,
    tax_registration_id,
    cash_rounding_step_minor,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING
    id, public_id, name, legal_name, currency, timezone,
    financial_year_start_month, tax_profile, tax_registration_id,
    cash_rounding_step_minor, created_at, updated_at;

-- name: VenueByID :one
SELECT
    id, public_id, name, legal_name, currency, timezone,
    financial_year_start_month, tax_profile, tax_registration_id,
    cash_rounding_step_minor, created_at, updated_at
FROM venues
WHERE id = ?;

-- name: VenueByPublicID :one
SELECT
    id, public_id, name, legal_name, currency, timezone,
    financial_year_start_month, tax_profile, tax_registration_id,
    cash_rounding_step_minor, created_at, updated_at
FROM venues
WHERE public_id = ?;

-- name: ListVenues :many
SELECT
    id, public_id, name, legal_name, currency, timezone,
    financial_year_start_month, tax_profile, tax_registration_id,
    cash_rounding_step_minor, created_at, updated_at
FROM venues
ORDER BY name;

-- name: UpdateVenue :one
--
-- Currency is deliberately absent. Changing it would reinterpret every minor
-- unit already recorded against this venue, which is a migration, not a
-- setting.
UPDATE venues
SET name                       = ?,
    legal_name                 = ?,
    timezone                   = ?,
    financial_year_start_month = ?,
    tax_profile                = ?,
    tax_registration_id        = ?,
    cash_rounding_step_minor   = ?,
    updated_at                 = ?
WHERE id = ?
RETURNING
    id, public_id, name, legal_name, currency, timezone,
    financial_year_start_month, tax_profile, tax_registration_id,
    cash_rounding_step_minor, created_at, updated_at;
