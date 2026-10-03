package menus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/byjoellobo/mez/internal/db/gen"
)

var (
	// ErrCourseNotFound means the course is not this venue's.
	ErrCourseNotFound = errors.New("no such course at this venue")
	// ErrTicketsOpen refuses renumbering the courses while a ticket is open:
	// its lines hold the numbers they were rung up under, and the kitchen
	// fires by number.
	ErrTicketsOpen = errors.New("tickets are still open")
)

// CourseInput is a course as the editor sends it. Its number is its place
// in the order, and is not chosen.
type CourseInput struct {
	Name string `json:"name"`
}

// CourseRetired is what retiring a course moved.
type CourseRetired struct {
	ItemsMoved int64 `json:"items_moved"`
}

// CreateCourse adds a course after the last one, numbered one past it.
func (s *Service) CreateCourse(ctx context.Context, venueID int64, in CourseInput) (int64, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return 0, err
	}
	now := s.now()
	var id int64
	err = s.edit(ctx, venueID, func(q *gen.Queries) error {
		next, err := q.NextCourseNumber(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: next course number: %w", err)
		}
		id, err = q.CreateCourse(ctx, gen.CreateCourseParams{
			VenueID: venueID, Number: next, Name: name, CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
	return id, err
}

// RenameCourse changes a course's name. Its number stays.
func (s *Service) RenameCourse(ctx context.Context, venueID, id int64, in CourseInput) error {
	name, err := cleanName(in.Name)
	if err != nil {
		return err
	}
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		n, err := q.RenameCourse(ctx, gen.RenameCourseParams{Name: name, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrCourseNotFound)
	})
}

// OrderCourses numbers the courses 1, 2, 3 in the order given. ids must name
// every course the venue has, retired ones included, once each. An order
// that changes nothing is accepted whatever is open; one that changes a
// number is refused while any ticket is open (ErrTicketsOpen).
func (s *Service) OrderCourses(ctx context.Context, venueID int64, ids []int64) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		have, err := q.CourseIDsByNumber(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: reading courses: %w", err)
		}
		if !samePermutation(have, ids) {
			return ErrStaleOrder
		}
		current, err := courseNumbers(ctx, q, venueID, have)
		if err != nil {
			return err
		}
		if slices.Equal(have, ids) && inSequence(current) {
			return nil
		}
		open, err := q.UnsettledOrderCount(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: counting the open tickets of venue %d: %w", venueID, err)
		}
		if open > 0 {
			return fmt.Errorf("%w: %d", ErrTicketsOpen, open)
		}
		// Numbers are unique, so each is first parked past every number in
		// use, then set for good: no two courses ever share one mid-way.
		now := s.now()
		park := current[len(current)-1] + int64(len(ids))
		for i, id := range ids {
			n, err := q.SetCourseNumber(ctx, gen.SetCourseNumberParams{Number: park + int64(i) + 1, Now: now, VenueID: venueID, ID: id})
			if err := oneRow(n, err, ErrCourseNotFound); err != nil {
				return err
			}
		}
		for i, id := range ids {
			n, err := q.SetCourseNumber(ctx, gen.SetCourseNumberParams{Number: int64(i) + 1, Now: now, VenueID: venueID, ID: id})
			if err := oneRow(n, err, ErrCourseNotFound); err != nil {
				return err
			}
		}
		return nil
	})
}

// courseNumbers is the venue's course numbers, in order.
func courseNumbers(ctx context.Context, q *gen.Queries, venueID int64, ids []int64) ([]int64, error) {
	rows, err := q.EditorCourses(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("menus: reading course numbers: %w", err)
	}
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = r.Number
	}
	if len(out) != len(ids) {
		return nil, fmt.Errorf("menus: %d courses read, %d numbered", len(ids), len(out))
	}
	return out, nil
}

// inSequence reports whether numbers are exactly 1, 2, 3, ... in order.
func inSequence(numbers []int64) bool {
	for i, n := range numbers {
		if n != int64(i)+1 {
			return false
		}
	}
	return true
}

// RetireCourse takes a course out of service. Its items must go somewhere: in.MoveTo
// names another course in service, which takes every item (retired ones too).
// Tickets already rung up keep the course number they copied. A venue keeps at
// least one course in service. Retiring one already retired changes nothing.
func (s *Service) RetireCourse(ctx context.Context, venueID, id int64, in RetireInput) (CourseRetired, error) {
	var out CourseRetired
	err := s.edit(ctx, venueID, func(q *gen.Queries) error {
		c, err := q.CourseForEdit(ctx, gen.CourseForEditParams{VenueID: venueID, ID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %d", ErrCourseNotFound, id)
		}
		if err != nil {
			return fmt.Errorf("menus: reading course %d: %w", id, err)
		}
		if !c.Active {
			return nil
		}
		live, err := q.ActiveCourseCount(ctx, venueID)
		if err != nil {
			return fmt.Errorf("menus: counting courses: %w", err)
		}
		if live < 2 {
			return &Invalid{"id", "This is the only course in service. Add another before retiring it."}
		}
		items, err := q.ItemCountForCourse(ctx, gen.ItemCountForCourseParams{VenueID: venueID, CourseID: id})
		if err != nil {
			return fmt.Errorf("menus: counting course %d's items: %w", id, err)
		}
		if items > 0 {
			target, err := q.CourseForEdit(ctx, gen.CourseForEditParams{VenueID: venueID, ID: in.MoveTo})
			switch {
			case in.MoveTo == 0:
				return &Invalid{"move_to", "Choose the course its items go to."}
			case in.MoveTo == id:
				return &Invalid{"move_to", "Choose a different course."}
			case errors.Is(err, sql.ErrNoRows):
				return &Invalid{"move_to", "That is not a course here."}
			case err != nil:
				return fmt.Errorf("menus: reading course %d: %w", in.MoveTo, err)
			case !target.Active:
				return &Invalid{"move_to", "That course is retired. Choose one in service."}
			}
			if out.ItemsMoved, err = q.MoveCourseItems(ctx, gen.MoveCourseItemsParams{
				ToCourse: in.MoveTo, Now: s.now(), VenueID: venueID, FromCourse: id,
			}); err != nil {
				return fmt.Errorf("menus: moving course %d's items: %w", id, err)
			}
		}
		n, err := q.SetCourseActive(ctx, gen.SetCourseActiveParams{Active: false, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrCourseNotFound)
	})
	return out, err
}

// RestoreCourse brings a course back into service. What was moved off it
// stays where it went.
func (s *Service) RestoreCourse(ctx context.Context, venueID, id int64) error {
	return s.edit(ctx, venueID, func(q *gen.Queries) error {
		n, err := q.SetCourseActive(ctx, gen.SetCourseActiveParams{Active: true, Now: s.now(), VenueID: venueID, ID: id})
		return oneRow(n, err, ErrCourseNotFound)
	})
}
