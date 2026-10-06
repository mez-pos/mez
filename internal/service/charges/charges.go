// Package charges takes a guest's payment through a payment gateway and
// puts it on their table's bill (phase 11, Task 4f1, ADR-220).
//
// A charge is asked of the gateway for what the table owes. The guest pays
// on the gateway's page. When the gateway says it has the money, and only
// then, the charge becomes a payment.tendered on the ticket, by the same
// command a cashier's tender uses, exactly once however often it is asked.
//
// Three things can go wrong with money here, and each has one answer:
//
//   - The gateway is asked twice, or two things ask at once. The tender
//     carries the charge's own idempotency key, so the second is the first.
//   - The gateway took an amount other than the one asked. Nothing is
//     tendered; the charge is kept as unapplied, for a person.
//   - The gateway has the money and the ticket cannot take it: it was paid
//     another way meanwhile, or closed. Nothing is forced onto the bill;
//     the charge is kept as unapplied, for a person to refund or record.
//
// A charge the bill cannot take yet for a passing reason (no shift open,
// the ticket on hold, the database busy) stays open and is tried again.
package charges

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/id"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/gateway"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

var (
	// ErrNoGateway means the venue has no payment gateway set up.
	ErrNoGateway = errors.New("the venue takes no payments through a gateway")

	// ErrNothingToPay means the table has no ticket, or nothing is owed.
	ErrNothingToPay = errors.New("there is nothing to pay on this bill")

	// ErrChargeNotFound means no charge with that id at the caller's venue.
	ErrChargeNotFound = errors.New("no such charge")

	// ErrNoNotices means the venue takes no notices from a gateway, or
	// none from the one named.
	ErrNoNotices = errors.New("the venue takes no notices from that gateway")

	// ErrBadNotice means a notice was not the gateway's own.
	ErrBadNotice = errors.New("the notice is not the gateway's")
)

// chargePrefix is the public ID prefix of a charge: chg_7K3M9QX2.
const chargePrefix = "chg"

// maxDraws bounds the redraw on a public ID collision.
const maxDraws = 5

// maxSweep is the most open charges one sweep asks the gateway about.
const maxSweep = 200

// Status is where a charge stands.
type Status string

// The statuses. The strings are what is stored and sent.
const (
	// Open: asked, and the gateway has not said paid or failed.
	Open Status = "open"
	// Paid: the gateway has the money and it is on the ticket.
	Paid Status = "paid"
	// Unapplied: the gateway has the money and the ticket could not take
	// it. A person must refund it at the gateway or record it by hand.
	Unapplied Status = "unapplied"
	// Failed: the gateway says it will not be paid.
	Failed Status = "failed"
)

// Statuses is every status, for the generated vocabulary.
func Statuses() []Status { return []Status{Open, Paid, Unapplied, Failed} }

// Charge is a charge as it stands.
type Charge struct {
	PublicID    string
	TableID     int64
	OrderID     int64
	Gateway     string
	CheckoutURL string
	Amount      money.Amount
	Status      Status
	// Problem is why the ticket could not take it, when Unapplied.
	Problem string
	// Replayed is true when Start found the charge already open and asked
	// the gateway for nothing.
	Replayed bool
}

// Orders is what charges need of the orders. *order.Service is it.
type Orders interface {
	TableTicket(ctx context.Context, venueID, tableID int64) (svcorder.Result, bool, error)
	TenderPayment(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.TenderPayment) (svcorder.Result, error)
}

// Gateways finds a venue's payment gateway, or ErrNoGateway when it has
// none.
type Gateways func(ctx context.Context, venueID int64) (gateway.Gateway, error)

// Clock is where the service gets the time.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Options configure a Service. DB, Orders and Gateways are required.
type Options struct {
	DB       *db.DB
	Orders   Orders
	Gateways Gateways
	Clock    Clock
	Log      *slog.Logger
	// OnChange is told a venue has an unapplied charge, or has one fewer,
	// once it is committed: the hub's Notify, so terminals read again
	// (ADR-223). Nil tells nobody.
	OnChange func(venueID int64)
	// Notices is the venue's gateway as one that calls back, when it can
	// and is set up to (ADR-224). Nil takes no notices.
	Notices gateway.Notified
}

// Service starts charges and settles them.
type Service struct {
	db       *db.DB
	orders   Orders
	gateways Gateways
	clock    Clock
	log      *slog.Logger
	onChange func(venueID int64)
	notices  gateway.Notified
}

// New returns a Service.
func New(o Options) (*Service, error) {
	if o.DB == nil || o.Orders == nil || o.Gateways == nil {
		return nil, errors.New("charges: the database, the orders and the gateways are all needed")
	}
	if o.Clock == nil {
		o.Clock = systemClock{}
	}
	if o.Log == nil {
		o.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if o.OnChange == nil {
		o.OnChange = func(int64) {}
	}
	return &Service{db: o.DB, orders: o.Orders, gateways: o.Gateways, clock: o.Clock, log: o.Log, onChange: o.OnChange, notices: o.Notices}, nil
}

// Start asks the venue's gateway for what the table owes now, and answers
// where the guest pays. Asked again while that charge is open and the
// table owes the same, it is the same charge and the gateway is not asked.
//
// A table that owes a different amount than an open charge asked for gets
// a new charge. The old one is left open: its page may still be on a
// phone, and if it is paid, Settle finds that out and deals with it.
func (s *Service) Start(ctx context.Context, venueID, tableID int64, description, returnURL string) (Charge, error) {
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
	gw, err := s.gateways(ctx, venueID)
	if err != nil {
		return Charge{}, err
	}

	prior, err := s.db.Queries().OpenGatewayChargeForOrder(ctx, gen.OpenGatewayChargeForOrderParams{
		OrderID: ticket.OrderID, Gateway: gw.Name(), AmountMinor: due.Minor, Currency: due.Currency,
	})
	if err == nil {
		c := toCharge(prior)
		c.Replayed = true
		return c, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Charge{}, fmt.Errorf("reading the open charge of order %d: %w", ticket.OrderID, err)
	}

	at := s.clock.Now().UTC()
	for range maxDraws {
		publicID, err := id.NewPublic(chargePrefix)
		if err != nil {
			return Charge{}, fmt.Errorf("drawing a charge id: %w", err)
		}
		checkout, err := gw.Start(ctx, gateway.Charge{Reference: publicID, Amount: due, Description: description, ReturnURL: returnURL})
		if err != nil {
			return Charge{}, fmt.Errorf("starting a charge at %s: %w", gw.Name(), err)
		}
		err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
			_, err := q.CreateGatewayCharge(ctx, gen.CreateGatewayChargeParams{
				PublicID: publicID, VenueID: venueID, TableID: tableID, OrderID: ticket.OrderID,
				Gateway: gw.Name(), GatewayID: checkout.ID, CheckoutUrl: checkout.URL,
				AmountMinor: due.Minor, Currency: due.Currency, CreatedAt: at,
			})
			return err
		})
		if db.IsUniqueViolation(err) {
			// The public id collided. The charge just opened at the gateway
			// is one nobody was given the page of, so nobody pays it.
			continue
		}
		if err != nil {
			return Charge{}, fmt.Errorf("saving charge %s: %w", publicID, err)
		}
		s.log.Info("a charge was started", "venue_id", venueID, "order_id", ticket.OrderID, "charge", publicID, "gateway", gw.Name(), "amount_minor", due.Minor)
		return Charge{
			PublicID: publicID, TableID: tableID, OrderID: ticket.OrderID, Gateway: gw.Name(),
			CheckoutURL: checkout.URL, Amount: due, Status: Open,
		}, nil
	}
	return Charge{}, fmt.Errorf("saving a charge for table %d: it kept colliding", tableID)
}

// Charge reads a charge as it stands, without asking the gateway.
func (s *Service) Charge(ctx context.Context, venueID int64, publicID string) (Charge, error) {
	row, err := s.row(ctx, venueID, publicID)
	if err != nil {
		return Charge{}, err
	}
	return toCharge(row), nil
}

// Settle asks the gateway how a charge stands and acts on the answer: paid
// puts the payment on the ticket, failed closes the charge, and pending
// leaves it. A charge already closed is answered as it is, and the gateway
// is not asked.
//
// An error means nothing was decided and the charge is still open: the
// gateway could not be reached, or the ticket could not take the payment
// for a reason that may pass. Settle is safe to call again, at any time,
// from anywhere.
func (s *Service) Settle(ctx context.Context, venueID int64, publicID string) (Charge, error) {
	row, err := s.row(ctx, venueID, publicID)
	if err != nil {
		return Charge{}, err
	}
	if Status(row.Status) != Open {
		return toCharge(row), nil
	}
	gw, err := s.gateways(ctx, venueID)
	if err != nil {
		return Charge{}, err
	}
	if gw.Name() != row.Gateway {
		// The venue changed providers with this charge open. Its money, if
		// any, is at the old one, which can no longer be asked.
		return Charge{}, fmt.Errorf("%w: charge %s is at %s, the venue now uses %s", ErrNoGateway, publicID, row.Gateway, gw.Name())
	}
	st, err := gw.Status(ctx, row.GatewayID)
	if err != nil {
		return Charge{}, fmt.Errorf("asking %s about charge %s: %w", gw.Name(), publicID, err)
	}

	switch st.State {
	case gateway.Pending:
		return toCharge(row), nil
	case gateway.Failed:
		return s.close(ctx, row, Failed, "")
	case gateway.Paid:
		return s.apply(ctx, row, st)
	default:
		return Charge{}, fmt.Errorf("charge %s: %s answered a state mez does not know, %q", publicID, gw.Name(), st.State)
	}
}

// apply puts a paid charge on its ticket.
func (s *Service) apply(ctx context.Context, row gen.GatewayCharge, st gateway.Status) (Charge, error) {
	asked := money.Amount{Minor: row.AmountMinor, Currency: row.Currency}
	// How it was paid is written once, before anything is tendered, and the
	// tender is made from what is written: asked twice, with whatever the
	// gateway says the second time, the same thing is tendered.
	if !row.Reference.Valid {
		method, reference := methodOf(st.Method), st.Reference
		if reference == "" {
			reference = row.GatewayID
		}
		err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
			_, err := q.SetGatewayChargePayment(ctx, gen.SetGatewayChargePaymentParams{
				Method: sql.NullString{String: string(method), Valid: true}, Reference: sql.NullString{String: reference, Valid: true}, ID: row.ID,
			})
			return err
		})
		if err != nil {
			return Charge{}, fmt.Errorf("recording how charge %s was paid: %w", row.PublicID, err)
		}
		var rerr error
		if row, rerr = s.row(ctx, row.VenueID, row.PublicID); rerr != nil {
			return Charge{}, rerr
		}
		if Status(row.Status) != Open {
			return toCharge(row), nil // someone else finished it meanwhile
		}
	}

	if st.Amount != asked {
		return s.close(ctx, row, Unapplied, fmt.Sprintf("The gateway took %s, and the charge was for %s.", st.Amount, asked))
	}

	_, err := s.orders.TenderPayment(ctx,
		// The guest is nobody: no actor and no device. The key is the
		// charge's own, so this tender happens once.
		svcorder.Meta{VenueID: row.VenueID, IdempotencyKey: "charge:" + row.PublicID},
		row.OrderID,
		svcorder.TenderPayment{Method: order.PaymentMethod(row.Method.String), Amount: asked, Reference: row.Reference.String},
	)
	switch {
	case err == nil:
		return s.close(ctx, row, Paid, "")
	case errors.Is(err, order.ErrOverTender):
		return s.close(ctx, row, Unapplied, "The bill no longer owes this much: part or all of it was paid another way.")
	case errors.Is(err, order.ErrOrderClosed), errors.Is(err, svcorder.ErrOrderNotFound):
		return s.close(ctx, row, Unapplied, "The bill was closed before this payment arrived.")
	default:
		// No shift open, the ticket on hold, the database busy: it may
		// pass, so the charge stays open and is tried again.
		return Charge{}, fmt.Errorf("putting charge %s on its ticket: %w", row.PublicID, err)
	}
}

// close settles an open charge. If someone else closed it first, theirs
// stands and is what is answered.
func (s *Service) close(ctx context.Context, row gen.GatewayCharge, to Status, problem string) (Charge, error) {
	at := s.clock.Now().UTC()
	var changed int64
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		n, err := q.SettleGatewayCharge(ctx, gen.SettleGatewayChargeParams{
			Status: string(to), Problem: sql.NullString{String: problem, Valid: problem != ""},
			SettledAt: sql.NullTime{Time: at, Valid: true}, ID: row.ID,
		})
		if err != nil {
			return err
		}
		if changed = n; n != 1 || to != Unapplied {
			return nil
		}
		// Money on no bill is something every terminal has to say, and no
		// order event says it: the floor's version does (ADR-211).
		return q.BumpFloorVersion(ctx, row.VenueID)
	})
	if err != nil {
		return Charge{}, fmt.Errorf("closing charge %s as %s: %w", row.PublicID, to, err)
	}
	if changed == 1 && to == Unapplied {
		s.onChange(row.VenueID)
	}
	after, err := s.row(ctx, row.VenueID, row.PublicID)
	if err != nil {
		return Charge{}, err
	}
	switch Status(after.Status) {
	case Paid:
		s.log.Info("a charge was paid and is on its ticket", "venue_id", row.VenueID, "order_id", row.OrderID, "charge", row.PublicID)
	case Unapplied:
		// Money a guest paid that is on no bill. It is the loudest thing
		// this package says.
		s.log.Error("a charge was paid and could not be put on its ticket: refund it or record it by hand",
			"venue_id", row.VenueID, "order_id", row.OrderID, "charge", row.PublicID, "gateway", row.Gateway,
			"gateway_id", row.GatewayID, "amount_minor", row.AmountMinor, "problem", after.Problem.String)
	}
	return toCharge(after), nil
}

// Sweep settles every open charge of a venue: it is how a payment lands
// when the guest paid and closed the page without coming back. It answers
// how many it closed, and the first error, having tried them all.
func (s *Service) Sweep(ctx context.Context, venueID int64) (int, error) {
	rows, err := s.db.Queries().GatewayChargesByStatus(ctx, gen.GatewayChargesByStatusParams{VenueID: venueID, Status: string(Open), Limit: maxSweep})
	if err != nil {
		return 0, fmt.Errorf("listing the open charges of venue %d: %w", venueID, err)
	}
	var first error
	closed := 0
	for _, r := range rows {
		c, err := s.Settle(ctx, venueID, r.PublicID)
		if err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		if c.Status != Open {
			closed++
		}
	}
	return closed, first
}

func (s *Service) row(ctx context.Context, venueID int64, publicID string) (gen.GatewayCharge, error) {
	row, err := s.db.Queries().GatewayChargeByPublicID(ctx, publicID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && row.VenueID != venueID) {
		return gen.GatewayCharge{}, fmt.Errorf("%w: %q", ErrChargeNotFound, publicID)
	}
	if err != nil {
		return gen.GatewayCharge{}, fmt.Errorf("reading charge %q: %w", publicID, err)
	}
	return row, nil
}

func toCharge(r gen.GatewayCharge) Charge {
	return Charge{
		PublicID: r.PublicID, TableID: r.TableID, OrderID: r.OrderID, Gateway: r.Gateway, CheckoutURL: r.CheckoutUrl,
		Amount: money.Amount{Minor: r.AmountMinor, Currency: r.Currency}, Status: Status(r.Status), Problem: r.Problem.String,
	}
}

// methodOf is a gateway's method as the bill records one. What the bill
// has no name for is other.
func methodOf(m gateway.Method) order.PaymentMethod {
	switch m {
	case gateway.UPI:
		return order.PaymentUPI
	case gateway.Card:
		return order.PaymentCard
	default:
		return order.PaymentOther
	}
}

// Notice takes a gateway's call that a charge has moved (ADR-224). It is a
// hint and nothing more: the call is checked to be the gateway's, the
// charge it names is found, and Settle asks the gateway itself. So a notice
// can make a payment land sooner, and can make nothing land that would not
// have.
//
// A notice about a charge mez does not know, or about nothing, is taken and
// does nothing. A settle that decides nothing is not an error here either:
// the gateway has done its part by calling, and the sweep will ask again.
func (s *Service) Notice(ctx context.Context, name string, header http.Header, body []byte) error {
	if s.notices == nil {
		return ErrNoNotices
	}
	gatewayID, err := s.notices.Notice(header, body)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadNotice, err)
	}
	if gatewayID == "" {
		return nil
	}
	row, err := s.db.Queries().GatewayChargeByGatewayID(ctx, gen.GatewayChargeByGatewayIDParams{Gateway: name, GatewayID: gatewayID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading the charge a notice is about: %w", err)
	}
	if _, err := s.Settle(ctx, row.VenueID, row.PublicID); err != nil {
		s.log.Warn("a gateway's notice could not be acted on just now; the sweep will ask again", "charge", row.PublicID, "error", err)
	}
	return nil
}
