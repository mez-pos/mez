package main

import (
	"context"
	"log/slog"

	"github.com/mez-pos/mez/internal/db"
)

// venueAtStart is the public id of the venue this server serves, for health
// to name (ADR-416), read once here so that health never asks the database.
// Empty with no venue yet (the wizard names the one it makes) and with more
// than one, which a tablet then cannot tell apart: it pairs again by hand.
func venueAtStart(ctx context.Context, database *db.DB, log *slog.Logger) string {
	venues, err := database.Queries().ListVenues(ctx)
	if err != nil {
		log.Warn("the venue could not be read for health to name; a tablet whose PC changes address pairs again", "error", err)
		return ""
	}
	if len(venues) > 1 {
		log.Warn("this machine has more than one venue, so health names none; a tablet whose PC changes address pairs again", "venues", len(venues))
	}
	if len(venues) != 1 {
		return ""
	}
	return venues[0].PublicID
}
