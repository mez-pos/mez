package charges

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
)

// A share is a guest paying part of the table's bill (phase 11, Task 4g1,
// ADR-232). It is an amount of what is owed and not a set of lines: the bill
// does not know whose phone ordered what.
//
// A page a guest can still pay on is a claim on that much of the bill. A
// charge is given only for what nobody else is claiming, so every page there
// is can be paid and the bill is not over-paid. A claim lasts as long as its
// page: until the gateway says it was paid, or will not be.

var (
	// ErrOverClaim means a charge was asked for more of the bill than is left
	// once what others are paying is taken off, or for the rest when there is
	// none.
	ErrOverClaim = errors.New("others at the table are paying that part of the bill")

	// ErrBadShare means a share did not say who is paying, or how much.
	ErrBadShare = errors.New("a share needs who is paying and an amount of the bill")

	// ErrBillMoved means the bill kept changing while a charge was being
	// started for it. Asked again, it will likely start.
	ErrBillMoved = errors.New("the bill changed while a charge was being started")
)

// maxPayer is the longest name a phone may give itself.
const maxPayer = 64

// maxMoves is how often a charge is started again because the bill moved
// under it. Each try opens a page at the gateway that nobody is given.
const maxMoves = 3

// part is what of a bill a charge is for: a payer's share of it, or with no
// payer the whole of what nobody is paying.
type part struct {
	payer  string
	amount money.Amount
}

func (p part) whole() bool { return p.payer == "" }

// Claims is how a table's bill stands for a phone that would pay some of it.
type Claims struct {
	// Due is what the bill owes.
	Due money.Amount
	// Claimed is what others are paying of it now: every page still open but
	// the phone's own shares.
	Claimed money.Amount
	// Unclaimed is the most the phone can ask for. It is never less than
	// nothing.
	Unclaimed money.Amount
}

// StartShare asks the venue's gateway for part of what the table owes, for
// one phone, and answers where that guest pays. payer is a name the phone
// made up and keeps: asked again by the same phone for the same amount while
// the charge is open, it is the same charge. Two phones are two charges,
// whatever they ask for.
//
// A share must fit in what others are not paying, or it is ErrOverClaim and
// the gateway is not asked. A phone's own earlier shares do not count against
// it: a guest who backed out of a page and chose another figure cannot close
// the first.
func (s *Service) StartShare(ctx context.Context, venueID, tableID int64, payer string, amount money.Amount, description, returnURL string) (Charge, error) {
	if payer == "" || len(payer) > maxPayer {
		return Charge{}, fmt.Errorf("%w: who is paying is %d bytes long", ErrBadShare, len(payer))
	}
	if amount.Minor <= 0 {
		return Charge{}, fmt.Errorf("%w: %d is not an amount to pay", ErrBadShare, amount.Minor)
	}
	return s.onBill(ctx, venueID, tableID, part{payer: payer, amount: amount}, description, returnURL)
}

// Claims reads how a table's bill stands for a phone. With no payer it is how
// it stands for a charge for the whole of what is left, which only shares
// stand in the way of. A table with no ticket has no claims and owes nothing.
func (s *Service) Claims(ctx context.Context, venueID, tableID int64, payer string) (Claims, error) {
	ticket, open, err := s.orders.TableTicket(ctx, venueID, tableID)
	if err != nil || !open {
		return Claims{}, err
	}
	due := ticket.State.Totals.BalanceDue
	others, err := claimed(ctx, s.db.Queries(), ticket.OrderID, payer)
	if err != nil {
		return Claims{}, err
	}
	return Claims{
		Due:       due,
		Claimed:   money.Amount{Minor: others, Currency: due.Currency},
		Unclaimed: money.Amount{Minor: max(due.Minor-others, 0), Currency: due.Currency},
	}, nil
}

// EqualPart is what one guest is asked for when a bill of total is split
// evenly among people, with unclaimed of it left for them to take.
//
// The parts are money.SplitEvenly's, so they add up to the total to the last
// minor unit, and a guest is offered the first part that what is already
// paid or claimed does not cover: guests paying in turn pay 333.34, 333.33
// and 333.33 of a thousand. It is never more than is left, so if two took the
// larger part at once the last pays a paisa less and the bill is still paid
// exactly.
func EqualPart(total, unclaimed money.Amount, people int) (money.Amount, error) {
	parts, err := money.SplitEvenly(total, people)
	if err != nil {
		return money.Amount{}, err
	}
	covered, err := total.Sub(unclaimed)
	if err != nil {
		return money.Amount{}, err
	}
	if total.Minor <= 0 || unclaimed.Minor <= 0 {
		return money.Amount{Currency: total.Currency}, nil
	}
	offered, reached := unclaimed, money.Amount{Currency: total.Currency}
	for _, p := range parts {
		if reached, err = reached.Add(p); err != nil {
			return money.Amount{}, err
		}
		if reached.Minor > covered.Minor {
			offered = p
			break
		}
	}
	if offered.Minor > unclaimed.Minor {
		offered = unclaimed
	}
	return offered, nil
}

// onBill starts a charge on a table's bill, again if the bill moved while it
// was being started.
func (s *Service) onBill(ctx context.Context, venueID, tableID int64, p part, description, returnURL string) (Charge, error) {
	for range maxMoves {
		c, err := s.tryOnBill(ctx, venueID, tableID, p, description, returnURL)
		if !errors.Is(err, ErrBillMoved) {
			return c, err
		}
	}
	return Charge{}, fmt.Errorf("%w: table %d, %d times", ErrBillMoved, tableID, maxMoves)
}

func (s *Service) tryOnBill(ctx context.Context, venueID, tableID int64, p part, description, returnURL string) (Charge, error) {
	ticket, open, err := s.orders.TableTicket(ctx, venueID, tableID)
	if err != nil {
		return Charge{}, err
	}
	if !open {
		return Charge{}, fmt.Errorf("%w: table %d has no ticket", ErrNothingToPay, tableID)
	}
	due := ticket.State.Totals.BalanceDue
	if due.Minor <= 0 {
		return Charge{}, fmt.Errorf("%w: nothing is owed at table %d", ErrNothingToPay, tableID)
	}
	if !p.whole() && p.amount.Currency != due.Currency {
		return Charge{}, fmt.Errorf("%w: the bill is in %s and the share in %s", ErrBadShare, due.Currency, p.amount.Currency)
	}
	gw, err := s.gateways(ctx, venueID)
	if err != nil {
		return Charge{}, err
	}

	orderID := sql.NullInt64{Int64: ticket.OrderID, Valid: true}
	others, err := claimed(ctx, s.db.Queries(), ticket.OrderID, p.payer)
	if err != nil {
		return Charge{}, err
	}
	ask := p.amount
	if p.whole() {
		ask = money.Amount{Minor: due.Minor - others, Currency: due.Currency}
	}
	// Asked here first, on a read that may be a moment old, so a share that
	// plainly does not fit opens no page at the gateway.
	if !fits(ask, others, due) {
		return Charge{}, fmt.Errorf("%w: %s asked of %s at table %d, with %d claimed", ErrOverClaim, ask, due, tableID, others)
	}

	a := asking{
		venueID: venueID, tableID: tableID, orderID: ticket.OrderID, payer: p.payer, due: ask, description: description, returnURL: returnURL,
		// Asked again where it counts: in the one write transaction that
		// saves the charge, so two phones cannot both be given the same part
		// of the bill, and against the bill as it is now.
		fits: func(q *gen.Queries) error {
			row, err := q.OrderByID(ctx, ticket.OrderID)
			if err != nil {
				return fmt.Errorf("reading the ticket at table %d again: %w", tableID, err)
			}
			if row.LastSeq != ticket.State.LastSeq {
				return ErrBillMoved
			}
			now, err := claimed(ctx, q, ticket.OrderID, p.payer)
			if err != nil {
				return err
			}
			if !fits(ask, now, due) {
				return fmt.Errorf("%w: %s asked of %s at table %d, with %d claimed", ErrOverClaim, ask, due, tableID, now)
			}
			return nil
		},
	}
	if p.whole() {
		a.prior = func() (gen.GatewayCharge, error) {
			return s.db.Queries().OpenGatewayChargeForOrder(ctx, gen.OpenGatewayChargeForOrderParams{
				OrderID: orderID, Gateway: gw.Name(), AmountMinor: ask.Minor, Currency: ask.Currency,
			})
		}
	} else {
		a.prior = func() (gen.GatewayCharge, error) {
			return s.db.Queries().OpenGatewayChargeForShare(ctx, gen.OpenGatewayChargeForShareParams{
				OrderID: orderID, Payer: sql.NullString{String: p.payer, Valid: true}, Gateway: gw.Name(), AmountMinor: ask.Minor, Currency: ask.Currency,
			})
		}
	}
	return s.open(ctx, gw, a)
}

// fits is whether a charge of ask can be given on a bill owing due, of which
// others are paying claimed.
func fits(ask money.Amount, claimed int64, due money.Amount) bool {
	return ask.Minor > 0 && ask.Minor <= due.Minor-claimed
}

// claimed is what stands in the way of a charge on a ticket, in minor units:
// for a payer's share, every open charge on the bill but that payer's own
// shares; for a charge for the whole of what is left, every open share.
//
// Charges for the whole bill do not count against each other. Two of them
// open for different amounts is a bill that changed, and that case is
// ADR-220's: the second to be paid is kept for a person.
func claimed(ctx context.Context, q *gen.Queries, orderID int64, payer string) (int64, error) {
	order := sql.NullInt64{Int64: orderID, Valid: true}
	var n int64
	var err error
	if payer == "" {
		n, err = q.OpenSharesOnOrder(ctx, order)
	} else {
		n, err = q.OpenClaimsOfOthers(ctx, gen.OpenClaimsOfOthersParams{OrderID: order, Payer: sql.NullString{String: payer, Valid: true}})
	}
	if err != nil {
		return 0, fmt.Errorf("reading what is being paid of order %d: %w", orderID, err)
	}
	return n, nil
}
