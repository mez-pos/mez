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
// A charge can also be for a round held until it is paid (ADR-226): then
// the paid round is put on a ticket and sent to the kitchen first, and the
// payment tendered on that ticket after.
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
	PublicID string
	TableID  int64
	// OrderID is the ticket the payment goes on. It is 0 for a round's
	// charge until the paid round is on a ticket.
	OrderID int64
	// Round is the round this pays for, by public id, when it is a round's
	// charge and not a bill's.
	Round string
	// Payer is the phone a share of the bill is for, by the name it gave
	// itself (ADR-232). A charge for the whole of what is left has none.
	Payer       string
	Gateway     string
	CheckoutURL string
	Amount      money.Amount
	Status      Status
	// Problem is why the ticket could not take it, when Unapplied.
	Problem string
	// LetGo is true when the table gave the charge's page up (ADR-234): it
	// is no claim on the bill, though it can still be paid.
	LetGo bool
	// Replayed is true when Start found the charge already open and asked
	// the gateway for nothing.
	Replayed bool
}

// Orders is what charges need of the orders. *order.Service is it.
type Orders interface {
	TableTicket(ctx context.Context, venueID, tableID int64) (svcorder.Result, bool, error)
	TenderPayment(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.TenderPayment) (svcorder.Result, error)
	PayableRound(ctx context.Context, venueID, tableID int64, publicID string) (svcorder.Round, error)
	SendPaidRound(ctx context.Context, venueID int64, publicID string) (svcorder.Round, error)
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
// What guests are paying in shares is not asked for again (ADR-232): with
// shares open it is a charge for the rest, and ErrOverClaim when there is
// no rest.
//
// A table that owes a different amount than an open charge asked for gets
// a new charge. The old one is left open: its page may still be on a
// phone, and if it is paid, Settle finds that out and deals with it.
func (s *Service) Start(ctx context.Context, venueID, tableID int64, description, returnURL string) (Charge, error) {
	return s.onBill(ctx, venueID, tableID, part{}, description, returnURL)
}

// asking is a charge about to be asked of the gateway: for a ticket's
// balance or a share of it, or for a round. prior finds the charge already
// open for the same thing, or sql.ErrNoRows. fits, when there is one, has
// the last word on whether the charge may be saved, in the transaction that
// saves it.
type asking struct {
	venueID, tableID int64
	orderID          int64
	round            string
	payer            string
	due              money.Amount
	description      string
	returnURL        string
	prior            func() (gen.GatewayCharge, error)
	fits             func(q *gen.Queries) error
}

// open answers the charge already open for what is asked, or asks the
// gateway for a new one and saves it.
func (s *Service) open(ctx context.Context, gw gateway.Gateway, a asking) (Charge, error) {
	replay := func() (Charge, bool, error) {
		prior, err := a.prior()
		if errors.Is(err, sql.ErrNoRows) {
			return Charge{}, false, nil
		}
		if err != nil {
			return Charge{}, false, fmt.Errorf("reading the open charge at table %d: %w", a.tableID, err)
		}
		c := toCharge(prior)
		c.Replayed = true
		return c, true, nil
	}
	if c, ok, err := replay(); err != nil || ok {
		return c, err
	}

	at := s.clock.Now().UTC()
	for range maxDraws {
		publicID, err := id.NewPublic(chargePrefix)
		if err != nil {
			return Charge{}, fmt.Errorf("drawing a charge id: %w", err)
		}
		checkout, err := gw.Start(ctx, gateway.Charge{Reference: publicID, Amount: a.due, Description: a.description, ReturnURL: a.returnURL})
		if err != nil {
			return Charge{}, fmt.Errorf("starting a charge at %s: %w", gw.Name(), err)
		}
		err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
			if a.fits != nil {
				// Refused here, the charge just opened at the gateway is one
				// nobody is given the page of.
				if err := a.fits(q); err != nil {
					return err
				}
			}
			_, err := q.CreateGatewayCharge(ctx, gen.CreateGatewayChargeParams{
				PublicID: publicID, VenueID: a.venueID, TableID: a.tableID,
				OrderID:       sql.NullInt64{Int64: a.orderID, Valid: a.orderID != 0},
				RoundPublicID: sql.NullString{String: a.round, Valid: a.round != ""},
				Payer:         sql.NullString{String: a.payer, Valid: a.payer != ""},
				Gateway:       gw.Name(), GatewayID: checkout.ID, CheckoutUrl: checkout.URL,
				AmountMinor: a.due.Minor, Currency: a.due.Currency, CreatedAt: at,
			})
			return err
		})
		if db.IsUniqueViolation(err) {
			// Another phone asked for the same round at the same moment, or
			// this one asked twice for the same share, and that charge is
			// the one. Otherwise the public id collided. Either
			// way the charge just opened at the gateway is one nobody was
			// given the page of, so nobody pays it.
			if c, ok, rerr := replay(); rerr != nil || ok {
				return c, rerr
			}
			continue
		}
		if err != nil {
			return Charge{}, fmt.Errorf("saving charge %s: %w", publicID, err)
		}
		s.log.Info("a charge was started", "venue_id", a.venueID, "order_id", a.orderID, "round", a.round, "share", a.payer != "", "charge", publicID, "gateway", gw.Name(), "amount_minor", a.due.Minor)
		return Charge{
			PublicID: publicID, TableID: a.tableID, OrderID: a.orderID, Round: a.round, Payer: a.payer, Gateway: gw.Name(),
			CheckoutURL: checkout.URL, Amount: a.due, Status: Open,
		}, nil
	}
	return Charge{}, fmt.Errorf("saving a charge for table %d: it kept colliding", a.tableID)
}

// StartRound asks the venue's gateway for what a round held until it is
// paid asks of the guest (ADR-226), and answers where they pay. Asked again
// while that charge is open, it is the same charge.
//
// What is asked is the round's own figure, fixed when it was placed: what
// its lines add to the bill. The table's balance has no part in it.
func (s *Service) StartRound(ctx context.Context, venueID, tableID int64, round, description, returnURL string) (Charge, error) {
	held, err := s.orders.PayableRound(ctx, venueID, tableID, round)
	if errors.Is(err, svcorder.ErrRoundPaid) {
		return Charge{}, fmt.Errorf("%w: %v", ErrNothingToPay, err)
	}
	if err != nil {
		return Charge{}, err
	}
	if held.Due.Minor <= 0 {
		return Charge{}, fmt.Errorf("%w: round %s asks for nothing", ErrNothingToPay, round)
	}
	gw, err := s.gateways(ctx, venueID)
	if err != nil {
		return Charge{}, err
	}
	return s.open(ctx, gw, asking{
		venueID: venueID, tableID: tableID, round: held.PublicID, due: held.Due, description: description, returnURL: returnURL,
		prior: func() (gen.GatewayCharge, error) {
			return s.db.Queries().OpenGatewayChargeForRound(ctx, gen.OpenGatewayChargeForRoundParams{
				RoundPublicID: sql.NullString{String: held.PublicID, Valid: true}, Gateway: gw.Name(),
			})
		},
	})
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

	// forRound is a round's charge; sent is whether its round is on a ticket.
	forRound, sent := row.RoundPublicID.Valid, row.OrderID.Valid
	if st.Amount != asked {
		problem := fmt.Sprintf("The gateway took %s, and the charge was for %s.", st.Amount, asked)
		if forRound && !sent {
			problem += notSent
		}
		return s.close(ctx, row, Unapplied, problem)
	}

	if forRound && !sent {
		var done bool
		var c Charge
		var err error
		if row, c, done, err = s.sendRound(ctx, row); err != nil || done {
			return c, err
		}
	}

	_, err := s.orders.TenderPayment(ctx,
		// The guest is nobody: no actor and no device. The key is the
		// charge's own, so this tender happens once.
		svcorder.Meta{VenueID: row.VenueID, IdempotencyKey: "charge:" + row.PublicID},
		row.OrderID.Int64,
		svcorder.TenderPayment{Method: order.PaymentMethod(row.Method.String), Amount: asked, Reference: row.Reference.String},
	)
	switch {
	case err == nil:
		return s.close(ctx, row, Paid, "")
	case errors.Is(err, order.ErrOverTender) && forRound:
		return s.close(ctx, row, Unapplied, "The order is with the kitchen, and the bill owes less than this payment: it changed, or was paid another way.")
	case errors.Is(err, order.ErrOverTender):
		return s.close(ctx, row, Unapplied, "The bill no longer owes this much: part or all of it was paid another way.")
	case (errors.Is(err, order.ErrOrderClosed) || errors.Is(err, svcorder.ErrOrderNotFound)) && forRound:
		return s.close(ctx, row, Unapplied, "The order went to the kitchen, and its bill was closed before this payment was put on it.")
	case errors.Is(err, order.ErrOrderClosed), errors.Is(err, svcorder.ErrOrderNotFound):
		return s.close(ctx, row, Unapplied, "The bill was closed before this payment arrived.")
	default:
		// No shift open, the ticket on hold, the database busy: it may
		// pass, so the charge stays open and is tried again.
		return Charge{}, fmt.Errorf("putting charge %s on its ticket: %w", row.PublicID, err)
	}
}

// notSent ends the problem of a round's charge whose round never went to
// the kitchen: staff reading it have a guest who paid and is waiting.
const notSent = " The order was not sent to the kitchen."

// sendRound puts the round a paid charge is for on a ticket and sends it,
// and writes that ticket on the charge. done is true when the charge was
// closed here instead, with nothing sent.
//
// It is a step of its own before the tender, and each is safe to do again:
// a round already sent answers with its ticket, so a charge that stopped
// between the two is finished by the next settle.
func (s *Service) sendRound(ctx context.Context, row gen.GatewayCharge) (gen.GatewayCharge, Charge, bool, error) {
	round, err := s.orders.SendPaidRound(ctx, row.VenueID, row.RoundPublicID.String)
	var c Charge
	switch {
	case errors.Is(err, svcorder.ErrRoundLapsed):
		c, err = s.close(ctx, row, Unapplied, "It paid for an order placed in a shift that has since closed."+notSent)
		return row, c, true, err
	case errors.Is(err, svcorder.ErrRoundNotFound):
		c, err = s.close(ctx, row, Unapplied, "The order it paid for is no longer there."+notSent)
		return row, c, true, err
	case err != nil:
		// No shift open, the table's ticket on hold or printed, the database
		// busy: it may pass, so the charge stays open and is tried again.
		return row, Charge{}, false, fmt.Errorf("sending the round charge %s paid for: %w", row.PublicID, err)
	}
	err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.SetGatewayChargeOrder(ctx, gen.SetGatewayChargeOrderParams{OrderID: sql.NullInt64{Int64: round.OrderID, Valid: true}, ID: row.ID})
		return err
	})
	if err != nil {
		return row, Charge{}, false, fmt.Errorf("writing the ticket of charge %s: %w", row.PublicID, err)
	}
	if row, err = s.row(ctx, row.VenueID, row.PublicID); err != nil {
		return row, Charge{}, false, err
	}
	if Status(row.Status) != Open {
		return row, toCharge(row), true, nil // someone else finished it meanwhile
	}
	return row, Charge{}, false, nil
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
		s.log.Info("a charge was paid and is on its ticket", "venue_id", row.VenueID, "order_id", row.OrderID.Int64, "charge", row.PublicID)
	case Unapplied:
		// Money a guest paid that is on no bill. It is the loudest thing
		// this package says.
		s.log.Error("a charge was paid and could not be put on its ticket: refund it or record it by hand",
			"venue_id", row.VenueID, "order_id", row.OrderID.Int64, "charge", row.PublicID, "gateway", row.Gateway,
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
		PublicID: r.PublicID, TableID: r.TableID, OrderID: r.OrderID.Int64, Round: r.RoundPublicID.String, Payer: r.Payer.String,
		Gateway: r.Gateway, CheckoutURL: r.CheckoutUrl,
		Amount: money.Amount{Minor: r.AmountMinor, Currency: r.Currency}, Status: Status(r.Status), Problem: r.Problem.String, LetGo: r.LetGoAt.Valid,
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
