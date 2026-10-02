// Package settings is the back office's venue settings (phase 10, Task 6,
// ADR-129): who the venue is, and one write path for the settings table that
// knows every key and refuses a bad value when it is saved rather than when
// a shift is counted or a bill is paid.
package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
)

// ErrNoVenue means the venue does not exist.
var ErrNoVenue = errors.New("no such venue")

// Invalid is a setting that cannot be saved, and the field it is about.
type Invalid struct {
	Field   string
	Message string
}

func (e *Invalid) Error() string { return e.Field + ": " + e.Message }

// Clock is where the service gets the time.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Service is the venue settings.
type Service struct {
	db    *db.DB
	clock Clock
}

// New returns a Service.
func New(d *db.DB, clock Clock) (*Service, error) {
	if d == nil {
		return nil, errors.New("settings: a database is required")
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{db: d, clock: clock}, nil
}

// Limits on what a venue is called: labels printed on a receipt, not prose.
const (
	maxName      = 60
	maxLegalName = 120
)

// Venue is who the venue is. Currency is here to be shown: it is never
// edited, since changing it would reinterpret every amount recorded.
type Venue struct {
	Name      string `json:"name"`
	LegalName string `json:"legal_name"`
	Currency  string `json:"currency"`
	Timezone  string `json:"timezone"`
}

// Details is the part of a venue the owner edits here.
type Details struct {
	Name      string `json:"name"`
	LegalName string `json:"legal_name"`
	Timezone  string `json:"timezone"`
}

// Entry is one key: what it is set to, and what it is when it is not set.
type Entry struct {
	Key     string `json:"key"`
	Value   string `json:"value"` // empty when not set
	Set     bool   `json:"set"`
	Default string `json:"default"`
}

// Overview is everything the venue is set to.
type Overview struct {
	Venue   Venue
	Entries []Entry
}

// Read is the venue and every key, in the registry's order.
func (s *Service) Read(ctx context.Context, venueID int64) (Overview, error) {
	q := s.db.Queries()
	v, err := q.VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return Overview{}, ErrNoVenue
	}
	if err != nil {
		return Overview{}, fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	rows, err := q.SettingsForVenue(ctx, venueID)
	if err != nil {
		return Overview{}, fmt.Errorf("reading the settings of venue %d: %w", venueID, err)
	}
	stored := make(map[string]string, len(rows))
	for _, r := range rows {
		stored[r.Key] = r.Value
	}
	out := Overview{
		Venue:   Venue{Name: v.Name, LegalName: v.LegalName, Currency: v.Currency, Timezone: v.Timezone},
		Entries: make([]Entry, len(keys)),
	}
	for i, k := range keys {
		val, set := stored[k.Name]
		out.Entries[i] = Entry{Key: k.Name, Value: val, Set: set, Default: k.Default}
	}
	return out, nil
}

// VenueName is the venue's name and nothing else, for a lock screen.
func (s *Service) VenueName(ctx context.Context, venueID int64) (string, error) {
	v, err := s.db.Queries().VenueByID(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoVenue
	}
	if err != nil {
		return "", fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	return v.Name, nil
}

// clean trims the details and refuses what cannot be used.
func (d Details) clean() (Details, error) {
	d.Name, d.LegalName, d.Timezone = strings.TrimSpace(d.Name), strings.TrimSpace(d.LegalName), strings.TrimSpace(d.Timezone)
	switch {
	case d.Name == "":
		return d, &Invalid{"name", "Give the venue a name."}
	case utf8.RuneCountInString(d.Name) > maxName:
		return d, &Invalid{"name", fmt.Sprintf("Keep the name to %d characters.", maxName)}
	case utf8.RuneCountInString(d.LegalName) > maxLegalName:
		return d, &Invalid{"legal_name", fmt.Sprintf("Keep the legal name to %d characters.", maxLegalName)}
	}
	// "Local" loads, as whatever zone this computer is in: a venue whose
	// times change with the machine is not a setting anyone meant.
	if d.Timezone == "" || strings.EqualFold(d.Timezone, "local") {
		return d, &Invalid{"timezone", "Choose a timezone, like Asia/Kolkata."}
	}
	if _, err := time.LoadLocation(d.Timezone); err != nil {
		return d, &Invalid{"timezone", "That is not a timezone name. Use one like Asia/Kolkata or Europe/London."}
	}
	return d, nil
}

// UpdateDetails saves the venue's name, legal name and timezone. Everything
// else about the venue is left as it is.
func (s *Service) UpdateDetails(ctx context.Context, venueID int64, in Details) error {
	in, err := in.clean()
	if err != nil {
		return err
	}
	return s.db.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := q.VenueByID(ctx, venueID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoVenue
		}
		if err != nil {
			return fmt.Errorf("reading venue %d: %w", venueID, err)
		}
		if _, err := q.UpdateVenue(ctx, gen.UpdateVenueParams{
			ID: venueID, Name: in.Name, LegalName: in.LegalName, Timezone: in.Timezone,
			FinancialYearStartMonth: v.FinancialYearStartMonth, TaxProfile: v.TaxProfile,
			TaxRegistrationID: v.TaxRegistrationID, CashRoundingStepMinor: v.CashRoundingStepMinor,
			UpdatedAt: s.clock.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("updating venue %d: %w", venueID, err)
		}
		return nil
	})
}

// Set saves values by key, all or none. A value of "" clears the key, so it
// takes its default again. A key nothing reads is refused, so a typo is not
// stored and then ignored forever.
//
// This is the write path the settings sections build on (6b to 6d); it checks
// each value alone, and the section that edits related keys checks them
// together before calling it.
func (s *Service) Set(ctx context.Context, venueID int64, values map[string]string) error {
	trimmed := make(map[string]string, len(values))
	for name, raw := range values {
		k, ok := find(name)
		if !ok {
			return &Invalid{name, "That is not a setting."}
		}
		v := strings.TrimSpace(raw)
		if v != "" {
			if msg := k.Check(v); msg != "" {
				return &Invalid{name, msg}
			}
		}
		trimmed[name] = v
	}
	now := s.clock.Now().UTC()
	return s.db.WriteTx(ctx, func(q *gen.Queries) error {
		if _, err := q.VenueByID(ctx, venueID); errors.Is(err, sql.ErrNoRows) {
			return ErrNoVenue
		} else if err != nil {
			return fmt.Errorf("reading venue %d: %w", venueID, err)
		}
		for name, v := range trimmed {
			var err error
			if v == "" {
				err = q.DeleteSetting(ctx, gen.DeleteSettingParams{VenueID: venueID, Key: name})
			} else {
				err = q.SetSetting(ctx, gen.SetSettingParams{VenueID: venueID, Key: name, Value: v, UpdatedAt: now})
			}
			if err != nil {
				return fmt.Errorf("saving %s: %w", name, err)
			}
		}
		return nil
	})
}
