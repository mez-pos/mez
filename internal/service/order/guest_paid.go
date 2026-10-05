package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/id"
	"github.com/byjoellobo/mez/internal/domain/money"
)

// "I have paid" (phase 11, Task 4e4, ADR-218). A guest who paid the venue
// by UPI says so on their phone, and the terminal tells staff to look.
//
// A notice is a message and nothing else. It appends no event, settles no
// bill and is in no report: UPI tells the venue's server nothing, so nothing
// here can know money arrived (ADR-215). A member of staff finds the payment
// on the venue's own phone and records it on the terminal, and that record,
// payment.tendered, is the only payment there is.

var (
	// ErrNothingToPay means there is nothing a guest could have paid by
	// UPI: the table has no ticket, nothing is owed on it, or the venue
	// takes no UPI.
	ErrNothingToPay = errors.New("there is nothing to pay by UPI on this bill")

	// ErrNoticeNotFound means no notice with that id at the caller's venue.
	ErrNoticeNotFound = errors.New("no such paid notice")
)

// noticePrefix is the public ID prefix of a paid notice: pdn_7K3M9QX2.
const noticePrefix = "pdn"

// PaidNotice is a guest's "I have paid", as staff see it. Amount is what
// was left on the ticket when they said it; Due is what is left now.
// Replayed is true when the guest had already said so and nothing was
// written this time.
type PaidNotice struct {
	PublicID      string
	TableID       int64
	TableName     string
	OrderID       int64
	OrderPublicID string
	Amount        money.Amount
	Due           money.Amount
	ToldAt        time.Time
	Replayed      bool
}

// TellPaid records that a guest at the table says they have paid, and tells
// the terminals. Said again while staff have still to deal with the first,
// it is the first: a ticket has one open notice.
func (s *Service) TellPaid(ctx context.Context, venueID, tableID int64) (PaidNotice, error) {
	ticket, open, err := s.TableTicket(ctx, venueID, tableID)
	if err != nil {
		return PaidNotice{}, err
	}
	if !open {
		return PaidNotice{}, fmt.Errorf("%w: table %d has no ticket", ErrNothingToPay, tableID)
	}
	due := ticket.State.Totals.BalanceDue
	// The venue must take UPI, and this bill must be one UPI can pay: the
	// same test the bill's own link passes.
	account, err := s.UPIAccount(ctx, venueID)
	if err != nil {
		return PaidNotice{}, err
	}
	if _, err := account.Link(due, ""); err != nil {
		return PaidNotice{}, fmt.Errorf("%w: %v", ErrNothingToPay, err)
	}

	at := s.clock.Now().UTC()
	for range maxPublicIDDraws {
		publicID, err := id.NewPublic(noticePrefix)
		if err != nil {
			return PaidNotice{}, fmt.Errorf("drawing a notice id: %w", err)
		}
		notice := PaidNotice{
			PublicID: publicID, TableID: tableID, OrderID: ticket.OrderID, OrderPublicID: ticket.PublicID,
			Amount: due, Due: due, ToldAt: at,
		}
		err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
			prior, err := q.OpenPaidNoticeForOrder(ctx, ticket.OrderID)
			if err == nil {
				notice.PublicID, notice.ToldAt, notice.Replayed = prior.PublicID, prior.CreatedAt.UTC(), true
				notice.Amount = money.Amount{Minor: prior.AmountMinor, Currency: prior.Currency}
				return nil
			}
			if !isNoRows(err) {
				return fmt.Errorf("reading the paid notice of order %d: %w", ticket.OrderID, err)
			}
			if _, err := q.CreatePaidNotice(ctx, gen.CreatePaidNoticeParams{
				PublicID: publicID, VenueID: venueID, TableID: tableID, OrderID: ticket.OrderID,
				AmountMinor: due.Minor, Currency: due.Currency, CreatedAt: at,
			}); err != nil {
				return fmt.Errorf("saving a paid notice: %w", err)
			}
			// A table saying it has paid is the floor changed, with no order
			// event to say so (ADR-211).
			return q.BumpFloorVersion(ctx, venueID)
		})
		if db.IsUniqueViolation(err) {
			// The public id collided, or the same guest said it twice at
			// once: either way the next look finds what is there.
			continue
		}
		if err != nil {
			return PaidNotice{}, err
		}
		if !notice.Replayed {
			s.onChange(venueID)
			s.log.Info("a guest says they have paid", "venue_id", venueID, "table_id", tableID, "order_id", ticket.OrderID, "notice", notice.PublicID)
		}
		return notice, nil
	}
	return PaidNotice{}, fmt.Errorf("saving a paid notice for table %d: it kept colliding", tableID)
}

// PaidTold reports whether a ticket has a notice staff have still to deal
// with: what the guest's bill reads to say "staff have been told".
func (s *Service) PaidTold(ctx context.Context, orderID int64) (bool, error) {
	_, err := s.db.Queries().OpenPaidNoticeForOrder(ctx, orderID)
	if isNoRows(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading the paid notice of order %d: %w", orderID, err)
	}
	return true, nil
}

// PaidNotices is the notices staff have still to deal with, oldest first.
// One whose ticket has since been settled or closed has been dealt with by
// the payment itself and is left out.
func (s *Service) PaidNotices(ctx context.Context, venueID int64) ([]PaidNotice, error) {
	rows, err := s.db.Queries().OpenPaidNotices(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("listing the paid notices of venue %d: %w", venueID, err)
	}
	out := make([]PaidNotice, 0, len(rows))
	for _, r := range rows {
		ticket, err := s.Order(ctx, venueID, r.OrderID)
		if errors.Is(err, ErrOrderNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		due := ticket.State.Totals.BalanceDue
		if due.Minor <= 0 {
			continue
		}
		out = append(out, PaidNotice{
			PublicID: r.PublicID, TableID: r.TableID, TableName: r.TableName, OrderID: r.OrderID, OrderPublicID: r.OrderPublicID,
			Amount: money.Amount{Minor: r.AmountMinor, Currency: r.Currency}, Due: due, ToldAt: r.CreatedAt.UTC(),
		})
	}
	return out, nil
}

// SeePaidNotice puts a notice away: a member of staff has looked, and
// either recorded the payment or found none. It changes nothing on the
// bill. Done twice, it is done once. The guest may say so again after it.
func (s *Service) SeePaidNotice(ctx context.Context, meta Meta, publicID string) error {
	if err := meta.validate(); err != nil {
		return err
	}
	if meta.ActorID < 1 {
		return fmt.Errorf("%w: a notice is put away by a member of staff", ErrInvalidCommand)
	}
	row, err := s.db.Queries().PaidNoticeByPublicID(ctx, publicID)
	if isNoRows(err) || (err == nil && row.VenueID != meta.VenueID) {
		return fmt.Errorf("%w: %q", ErrNoticeNotFound, publicID)
	}
	if err != nil {
		return fmt.Errorf("reading paid notice %q: %w", publicID, err)
	}
	at := s.clock.Now().UTC()
	var changed int64
	err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
		n, err := q.MarkPaidNoticeSeen(ctx, gen.MarkPaidNoticeSeenParams{
			SeenAt: sql.NullTime{Time: at, Valid: true}, SeenBy: sql.NullInt64{Int64: meta.ActorID, Valid: true},
			ID: row.ID, VenueID: meta.VenueID,
		})
		if err != nil {
			return fmt.Errorf("putting paid notice %s away: %w", publicID, err)
		}
		if changed = n; n != 1 {
			return nil
		}
		return q.BumpFloorVersion(ctx, meta.VenueID)
	})
	if err != nil {
		return err
	}
	if changed == 1 {
		s.onChange(meta.VenueID)
	}
	return nil
}
