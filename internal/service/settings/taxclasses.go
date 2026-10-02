package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// The refusals for tax classes. The API maps them to codes.
var (
	ErrTaxClassNotFound = errors.New("no such tax class at this venue")
	ErrTaxClassName     = errors.New("a tax class with that name already exists")
)

// TaxClassInUse is a class that active items still charge, so it cannot be
// retired.
type TaxClassInUse struct{ Items []string }

func (e *TaxClassInUse) Error() string {
	return fmt.Sprintf("%d items still use this tax class", len(e.Items))
}

// maxClassName bounds a tax class's name: a label in a drop-down.
const maxClassName = 40

// TaxClass is a rate items share. A price change on an item, or a rate
// change here, is for lines rung up from then on: an open ticket keeps the
// rate it was rung up at (docs/02-domain.md, the snapshot rule).
type TaxClass struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Rate   tax.Rate `json:"rate"`
	Exempt bool     `json:"exempt"`
	Active bool     `json:"active"`
}

// TaxClassInput is a tax class as the owner sets it.
type TaxClassInput struct {
	Name   string   `json:"name"`
	Rate   tax.Rate `json:"rate"`
	Exempt bool     `json:"exempt"`
}

func (in TaxClassInput) clean() (TaxClassInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	switch {
	case in.Name == "":
		return in, &Invalid{"name", "Give the tax class a name, like Food or Alcohol."}
	case utf8.RuneCountInString(in.Name) > maxClassName:
		return in, &Invalid{"name", fmt.Sprintf("Keep the name to %d characters.", maxClassName)}
	case in.Rate.Validate() != nil:
		return in, &Invalid{"rate", "Enter a rate from 0% to 100%."}
	case in.Exempt && in.Rate != 0:
		return in, &Invalid{"rate", "An exempt class has no rate: set it to 0%, or untick exempt."}
	}
	return in, nil
}

func (s *Service) taxClasses(ctx context.Context, q *gen.Queries, venueID int64) ([]TaxClass, error) {
	rows, err := q.TaxClassesForVenue(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("reading the tax classes of venue %d: %w", venueID, err)
	}
	out := make([]TaxClass, len(rows))
	for i, r := range rows {
		out[i] = TaxClass{ID: r.ID, Name: r.Name, Rate: tax.Rate(r.Rate), Exempt: r.Exempt, Active: r.Active}
	}
	return out, nil
}

// nameTaken reports whether another class has this name, retired ones
// included: bringing one back should never make two the same.
func (s *Service) nameTaken(ctx context.Context, q *gen.Queries, venueID, except int64, name string) error {
	classes, err := s.taxClasses(ctx, q, venueID)
	if err != nil {
		return err
	}
	for _, c := range classes {
		if c.ID != except && strings.EqualFold(c.Name, name) {
			return fmt.Errorf("%w: %w", ErrTaxClassName, &Invalid{"name", "A tax class with that name already exists."})
		}
	}
	return nil
}

// edit runs one change to the tax classes through the menu's write path,
// which bumps the menu version in the same transaction, and then tells
// the tills.
func (s *Service) edit(ctx context.Context, venueID int64, fn func(q *gen.Queries) error) error {
	if _, err := s.db.EditMenu(ctx, venueID, fn); err != nil {
		return err
	}
	s.onChange(venueID)
	return nil
}

// CreateTaxClass adds a class and returns its id.
func (s *Service) CreateTaxClass(ctx context.Context, venueID int64, in TaxClassInput) (int64, error) {
	in, err := in.clean()
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.edit(ctx, venueID, func(q *gen.Queries) error {
		if err := s.nameTaken(ctx, q, venueID, 0, in.Name); err != nil {
			return err
		}
		now := s.clock.Now().UTC()
		id, err = q.CreateTaxClass(ctx, gen.CreateTaxClassParams{
			VenueID: venueID, Name: in.Name, Rate: int64(in.Rate), Exempt: in.Exempt, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("creating a tax class: %w", err)
		}
		return nil
	})
	return id, err
}

// UpdateTaxClass renames a class or changes its rate. Open tickets keep
// the rate they were rung up at.
func (s *Service) UpdateTaxClass(ctx context.Context, venueID, id int64, in TaxClassInput) error {
	in, err := in.clean()
	if err != nil {
		return err
	}
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		if err := s.nameTaken(ctx, q, venueID, id, in.Name); err != nil {
			return err
		}
		n, err := q.UpdateTaxClass(ctx, gen.UpdateTaxClassParams{
			Name: in.Name, Rate: int64(in.Rate), Exempt: in.Exempt, Now: s.clock.Now().UTC(), VenueID: venueID, ID: id,
		})
		if err != nil {
			return fmt.Errorf("updating tax class %d: %w", id, err)
		}
		if n != 1 {
			return ErrTaxClassNotFound
		}
		return nil
	})
}

// SetTaxClassActive retires a class or brings it back. A class that items on
// the menu still charge cannot be retired: they would sell at a rate nobody
// can now choose.
func (s *Service) SetTaxClassActive(ctx context.Context, venueID, id int64, active bool) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		if !active {
			names, err := q.ActiveItemsOnTaxClass(ctx, gen.ActiveItemsOnTaxClassParams{VenueID: venueID, TaxClassID: id})
			if err != nil {
				return fmt.Errorf("reading the items on tax class %d: %w", id, err)
			}
			if len(names) > 0 {
				return &TaxClassInUse{Items: names}
			}
		}
		n, err := q.SetTaxClassActive(ctx, gen.SetTaxClassActiveParams{Active: active, Now: s.clock.Now().UTC(), VenueID: venueID, ID: id})
		if err != nil {
			return fmt.Errorf("setting tax class %d active: %w", id, err)
		}
		if n != 1 {
			return ErrTaxClassNotFound
		}
		return nil
	})
}
