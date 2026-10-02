package menus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/money"
)

// ErrGroupNotFound is a modifier group that is not this venue's.
var ErrGroupNotFound = errors.New("no such modifier group at this venue")

// GroupInput is a modifier group as the editor saves it: whole, with its
// choices in the order the choice sheet shows them. A choice with an id is
// one the group already has; one without is new; one the group has that is
// not listed is retired.
type GroupInput struct {
	Name      string          `json:"name"`
	MinSelect int64           `json:"min_select"`
	MaxSelect int64           `json:"max_select"`
	Modifiers []ModifierInput `json:"modifiers"`
}

// ModifierInput is one choice. Price is per unit of the line and may be
// negative.
type ModifierInput struct {
	ID        int64        `json:"id,omitempty"`
	Name      string       `json:"name"`
	Price     money.Amount `json:"price"`
	IsDefault bool         `json:"is_default"`
}

// CreateGroup adds a modifier group and its choices.
func (s *Service) CreateGroup(ctx context.Context, venueID int64, in GroupInput) (int64, error) {
	var id int64
	err := s.edit(ctx, venueID, func(q *gen.Queries) error {
		draft, err := s.checkGroup(ctx, q, venueID, in)
		if err != nil {
			return err
		}
		for i, m := range in.Modifiers {
			if m.ID != 0 {
				return &Invalid{fmt.Sprintf("modifiers.%d.id", i), "A new group has no choices yet. Reload and try again."}
			}
		}
		now := s.now()
		id, err = q.CreateModifierGroup(ctx, gen.CreateModifierGroupParams{
			VenueID: venueID, Name: draft.Name, MinSelect: draft.MinSelect, MaxSelect: draft.MaxSelect, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("menus: creating a modifier group: %w", err)
		}
		return s.writeChoices(ctx, q, venueID, id, draft, in.Modifiers, nil)
	})
	return id, err
}

// UpdateGroup saves a group whole: its name, its bounds, and its choices
// as listed. A line already rung up keeps the choices and prices it copied.
func (s *Service) UpdateGroup(ctx context.Context, venueID, id int64, in GroupInput) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		if _, err := s.group(ctx, q, venueID, id); err != nil {
			return err
		}
		draft, err := s.checkGroup(ctx, q, venueID, in)
		if err != nil {
			return err
		}
		have, err := q.ModifierIDsOfGroup(ctx, gen.ModifierIDsOfGroupParams{VenueID: venueID, GroupID: id})
		if err != nil {
			return fmt.Errorf("menus: reading the choices of group %d: %w", id, err)
		}
		n, err := q.UpdateModifierGroup(ctx, gen.UpdateModifierGroupParams{
			Name: draft.Name, MinSelect: draft.MinSelect, MaxSelect: draft.MaxSelect, Now: s.now(), VenueID: venueID, ID: id,
		})
		if err := oneRow(n, err, ErrGroupNotFound); err != nil {
			return err
		}
		return s.writeChoices(ctx, q, venueID, id, draft, in.Modifiers, have)
	})
}

// SetGroupActive retires a group or brings it back. Retired, it leaves the
// choice sheet of every item that offers it; brought back, it returns to
// them, because the links stay.
func (s *Service) SetGroupActive(ctx context.Context, venueID, id int64, active bool) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		n, err := q.SetModifierGroupActive(ctx, gen.SetModifierGroupActiveParams{Active: active, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrGroupNotFound)
	})
}

// SetItemGroups sets which groups an item offers, in the order its choice
// sheet shows them. An empty list offers none. A retired group may be
// listed: it shows again when it is brought back.
func (s *Service) SetItemGroups(ctx context.Context, venueID, itemID int64, ids []int64) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		if _, err := q.ItemForEdit(ctx, gen.ItemForEditParams{VenueID: venueID, ID: itemID}); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %d", ErrItemNotFound, itemID)
		} else if err != nil {
			return fmt.Errorf("menus: reading item %d: %w", itemID, err)
		}
		seen := make(map[int64]bool, len(ids))
		for _, id := range ids {
			if seen[id] {
				return &Invalid{"ids", "A group is listed twice."}
			}
			seen[id] = true
			if _, err := s.group(ctx, q, venueID, id); errors.Is(err, ErrGroupNotFound) {
				return &Invalid{"ids", "One of those groups is not on this menu. Reload and try again."}
			} else if err != nil {
				return err
			}
		}
		if err := q.DetachItemGroups(ctx, gen.DetachItemGroupsParams{VenueID: venueID, ItemID: itemID}); err != nil {
			return fmt.Errorf("menus: clearing the groups of item %d: %w", itemID, err)
		}
		for i, id := range ids {
			if err := q.AttachModifierGroup(ctx, gen.AttachModifierGroupParams{VenueID: venueID, ItemID: itemID, GroupID: id, SortOrder: int64(i)}); err != nil {
				return fmt.Errorf("menus: offering group %d on item %d: %w", id, itemID, err)
			}
		}
		return nil
	})
}

// checkGroup checks a group's rules and its choices' currency.
func (s *Service) checkGroup(ctx context.Context, q *gen.Queries, venueID int64, in GroupInput) (menu.GroupDraft, error) {
	venue, err := q.VenueByID(ctx, venueID)
	if err != nil {
		return menu.GroupDraft{}, fmt.Errorf("menus: reading venue %d: %w", venueID, err)
	}
	draft := menu.GroupDraft{Name: in.Name, MinSelect: in.MinSelect, MaxSelect: in.MaxSelect, Choices: make([]menu.ChoiceDraft, len(in.Modifiers))}
	for i, m := range in.Modifiers {
		if m.Price.Currency != venue.Currency {
			return menu.GroupDraft{}, &Invalid{fmt.Sprintf("modifiers.%d.price", i), fmt.Sprintf("Prices at this venue are in %s.", venue.Currency)}
		}
		if _, err := money.New(m.Price.Minor, m.Price.Currency); err != nil {
			return menu.GroupDraft{}, &Invalid{fmt.Sprintf("modifiers.%d.price", i), "That price is too large."}
		}
		draft.Choices[i] = menu.ChoiceDraft{Name: m.Name, Price: m.Price.Minor, IsDefault: m.IsDefault}
	}
	clean, p := menu.CleanGroup(draft)
	if p != nil {
		return menu.GroupDraft{}, &Invalid{p.Field, p.Message}
	}
	return clean, nil
}

// writeChoices saves a group's choices as listed: each listed id updated in
// place and put back on offer, each new one created, and each the group
// had but no longer lists retired. have is the ids the group had.
func (s *Service) writeChoices(ctx context.Context, q *gen.Queries, venueID, groupID int64, draft menu.GroupDraft, in []ModifierInput, have []int64) error {
	owned := make(map[int64]bool, len(have))
	for _, id := range have {
		owned[id] = true
	}
	now := s.now()
	kept := make(map[int64]bool, len(in))
	for i, c := range draft.Choices {
		id := in[i].ID
		if id == 0 {
			if _, err := q.CreateModifier(ctx, gen.CreateModifierParams{
				VenueID: venueID, GroupID: groupID, Name: c.Name, Price: c.Price, IsDefault: c.IsDefault, SortOrder: int64(i),
				CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return fmt.Errorf("menus: adding choice %q: %w", c.Name, err)
			}
			continue
		}
		if !owned[id] || kept[id] {
			return &Invalid{fmt.Sprintf("modifiers.%d.id", i), "The group changed since you opened it. Reload and try again."}
		}
		kept[id] = true
		n, err := q.UpdateModifier(ctx, gen.UpdateModifierParams{
			Name: c.Name, Price: c.Price, IsDefault: c.IsDefault, SortOrder: int64(i), Now: now,
			VenueID: venueID, GroupID: groupID, ID: id,
		})
		if err := oneRow(n, err, ErrGroupNotFound); err != nil {
			return err
		}
	}
	for _, id := range have {
		if kept[id] {
			continue
		}
		if _, err := q.RetireModifier(ctx, gen.RetireModifierParams{Now: now, VenueID: venueID, ID: id}); err != nil {
			return fmt.Errorf("menus: retiring choice %d: %w", id, err)
		}
	}
	return nil
}

func (s *Service) group(ctx context.Context, q *gen.Queries, venueID, id int64) (gen.GroupForEditRow, error) {
	g, err := q.GroupForEdit(ctx, gen.GroupForEditParams{VenueID: venueID, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return g, fmt.Errorf("%w: %d", ErrGroupNotFound, id)
	}
	if err != nil {
		return g, fmt.Errorf("menus: reading modifier group %d: %w", id, err)
	}
	return g, nil
}
