// Package dbhealth adapts the database's health to the vocabulary
// /api/v1/health publishes.
//
// The two packages use the same three words, and this is the one place allowed
// to know both, so the mapping is written out rather than left to the strings
// happening to match. internal/db must not import internal/api, and
// internal/api must not import internal/db. A venue's server and a mirror's
// both ask it (ADR-353).
package dbhealth

import (
	"github.com/byjoellobo/mez/internal/api"
	"github.com/byjoellobo/mez/internal/db"
)

// Check is the database's health in the API's words, as api.WithDBCheck
// takes it.
func Check(database *db.DB) func() string {
	return func() string { return translate(database.Health().Status) }
}

// translate is the mapping itself, split out so it can be tested against
// every status without a database to produce them.
func translate(status string) string {
	switch status {
	case db.StatusOK:
		return api.CheckOK
	case db.StatusFailing:
		return api.CheckFailing
	default:
		return api.CheckUnknown
	}
}
