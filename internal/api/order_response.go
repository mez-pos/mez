package api

import (
	"cmp"
	"slices"
	"time"

	"github.com/mez-pos/mez/internal/domain/kitchen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tax"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// orderResponse is an order as every screen reads it: the projection, not
// the events. A terminal redraws the bill from this and nothing else.
type orderResponse struct {
	ID       string        `json:"id"`
	Status   order.Status  `json:"status"`
	Channel  order.Channel `json:"channel"`
	Currency string        `json:"currency"`
	// Service is how it is served: dine_in, takeaway or delivery (ADR-146).
	Service   order.ServiceType `json:"service"`
	TableID   int64             `json:"table_id,omitempty"`
	Joined    []int64           `json:"joined,omitempty"` // tables joined for one party (ADR-088)
	Covers    int64             `json:"covers"`
	OpenedBy  int64             `json:"opened_by,omitempty"`
	OpenedAt  time.Time         `json:"opened_at"`
	ClosedAt  *time.Time        `json:"closed_at,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`

	// LastSeq is the order's last event. A client sends it back on a
	// WebSocket reconnect to be told what it missed.
	LastSeq  int64 `json:"last_seq"`
	Reopens  int64 `json:"reopens,omitempty"`
	Invoiced bool  `json:"invoiced"`

	Lines     []lineResponse     `json:"lines"`
	Notes     []noteResponse     `json:"notes"`
	Tickets   []ticketResponse   `json:"tickets"`
	Tenders   []tenderResponse   `json:"tenders"`
	Refunds   []refundResponse   `json:"refunds"`
	Invoices  []invoiceResponse  `json:"invoices"`
	Credits   []creditResponse   `json:"credits"`
	Discounts []discountResponse `json:"discounts"`

	Totals totalsResponse `json:"totals"`
	Tax    *taxResponse   `json:"tax,omitempty"`
}

// lineResponse is one line of the bill. The money fields are what the line
// is worth now: Due is what the guest pays for it, and is what splitting by
// item divides.
type lineResponse struct {
	ID        int64            `json:"id"`
	ItemID    int64            `json:"item_id"`
	Name      string           `json:"name"`
	Qty       int64            `json:"qty"`
	UnitPrice money.Amount     `json:"unit_price"`
	Modifiers []order.Modifier `json:"modifiers,omitempty"`
	Course    int64            `json:"course,omitempty"`
	Station   string           `json:"station,omitempty"`

	Status       order.LineStatus `json:"status"`
	ReasonCode   string           `json:"reason_code,omitempty"`
	AuthorisedBy int64            `json:"authorised_by,omitempty"`
	CourseFired  bool             `json:"course_fired"`
	Sent         bool             `json:"sent"`

	Gross    money.Amount `json:"gross"`
	Discount money.Amount `json:"discount"`
	Tax      money.Amount `json:"tax"`
	Due      money.Amount `json:"due"`

	// What credit notes have taken off the line so far (ADR-083): whole
	// units, and value. The refund sheet steps up to Qty - CreditedQty.
	CreditedQty int64        `json:"credited_qty"`
	Credited    money.Amount `json:"credited"`
}

type noteResponse struct {
	Seq    int64       `json:"seq"`
	Scope  order.Scope `json:"scope"`
	LineID int64       `json:"line_id,omitempty"`
	Text   string      `json:"text"`
}

type ticketResponse struct {
	Seq     int64     `json:"seq"`
	Station string    `json:"station"`
	LineIDs []int64   `json:"line_ids"`
	At      time.Time `json:"at"`
	// Status is where it is at its station, ready once bumped (ADR-090).
	Status  kitchen.TicketStatus `json:"status"`
	ReadyAt *time.Time           `json:"ready_at,omitempty"`
}

type tenderResponse struct {
	ID         int64               `json:"id"`
	Method     order.PaymentMethod `json:"method"`
	Amount     money.Amount        `json:"amount"`
	Tip        money.Amount        `json:"tip"`
	Reference  string              `json:"reference,omitempty"`
	Status     order.PaymentStatus `json:"status"`
	Refunded   money.Amount        `json:"refunded"`
	VoidReason string              `json:"void_reason,omitempty"`
}

type refundResponse struct {
	Seq              int64        `json:"seq"`
	Amount           money.Amount `json:"amount"`
	AgainstPaymentID int64        `json:"against_payment_id"`
	Reason           string       `json:"reason"`
}

type invoiceResponse struct {
	Seq       int64  `json:"seq"`
	InvoiceNo string `json:"invoice_no"`
	Series    string `json:"series"`
	Reprints  int    `json:"reprints"`
}

type discountResponse struct {
	Seq    int64              `json:"seq"`
	Scope  order.Scope        `json:"scope"`
	LineID int64              `json:"line_id,omitempty"`
	Kind   order.DiscountKind `json:"kind"`
	Reason string             `json:"reason"`
	Value  money.Amount       `json:"value"`
}

// totalsResponse is the money of the order. BalanceDue is what the charge
// button shows.
type totalsResponse struct {
	Gross          money.Amount `json:"gross"`
	LineDiscounts  money.Amount `json:"line_discounts"`
	OrderDiscounts money.Amount `json:"order_discounts"`
	Subtotal       money.Amount `json:"subtotal"`
	Taxable        money.Amount `json:"taxable"`
	Tax            money.Amount `json:"tax"`
	Total          money.Amount `json:"total"`
	Paid           money.Amount `json:"paid"`
	Refunded       money.Amount `json:"refunded"`
	NetPaid        money.Amount `json:"net_paid"`
	Credited       money.Amount `json:"credited"`
	BalanceDue     money.Amount `json:"balance_due"`
	Tips           money.Amount `json:"tips"`
	Voided         money.Amount `json:"voided"`
	Comped         money.Amount `json:"comped"`
}

// taxResponse is the rate-wise summary a receipt prints. It is absent for an
// untaxed venue rather than present and zero.
type taxResponse struct {
	Profile    string               `json:"profile"`
	Pricing    tax.Pricing          `json:"pricing"`
	Components []tax.ComponentTotal `json:"components"`
	Taxable    money.Amount         `json:"taxable"`
	Tax        money.Amount         `json:"tax"`
	Total      money.Amount         `json:"total"`
}

// commandResponse is what a mutating endpoint answers: the order as it now
// stands, and the events the command appended. Replayed says the command
// carried an idempotency key that had already been applied, so this is the
// earlier answer and nothing was written this time.
type commandResponse struct {
	Order    orderResponse `json:"order"`
	Events   []order.Event `json:"events"`
	Replayed bool          `json:"replayed,omitempty"`
}

// toCommandResponse renders a service result.
func toCommandResponse(res svcorder.Result) commandResponse {
	events := res.Events
	if events == nil {
		events = []order.Event{}
	}
	return commandResponse{Order: toOrderResponse(res), Events: events, Replayed: res.Replayed}
}

// toOrderResponse renders the projection.
func toOrderResponse(res svcorder.Result) orderResponse {
	s := res.State
	out := orderResponse{
		ID: res.PublicID, Status: s.Status, Channel: s.Channel, Currency: s.Currency, Service: s.Service,
		TableID: s.TableID, Joined: s.Joined, Covers: s.Covers, OpenedBy: s.OpenedBy, OpenedAt: s.OpenedAt.UTC(),
		UpdatedAt: s.UpdatedAt.UTC(), LastSeq: s.LastSeq, Reopens: s.Reopens, Invoiced: s.Invoiced,
		Totals: totalsResponse(s.Totals),
	}
	if !s.ClosedAt.IsZero() {
		closed := s.ClosedAt.UTC()
		out.ClosedAt = &closed
	}

	out.Lines = make([]lineResponse, len(s.Lines))
	for i, l := range s.Lines {
		out.Lines[i] = lineResponse{
			ID: l.ID, ItemID: l.ItemID, Name: l.Name, Qty: l.Qty, UnitPrice: l.UnitPrice,
			Modifiers: l.Modifiers, Course: l.Course, Station: l.Station,
			Status: l.Status, ReasonCode: l.ReasonCode, AuthorisedBy: l.AuthorisedBy,
			CourseFired: l.CourseFired, Sent: l.Sent,
			Gross: l.Gross, Discount: l.Discount, Tax: l.Tax, Due: l.Due,
		}
		c := s.CreditedFor(l.ID)
		out.Lines[i].CreditedQty, out.Lines[i].Credited = c.Qty, c.Amount
	}
	out.Notes = make([]noteResponse, len(s.Notes))
	for i, n := range s.Notes {
		out.Notes[i] = noteResponse{Seq: n.Seq, Scope: n.Scope, LineID: n.LineID, Text: n.Text}
	}
	out.Tickets = make([]ticketResponse, len(s.Tickets))
	for i, t := range s.Tickets {
		out.Tickets[i] = ticketResponse{Seq: t.Seq, Station: t.Station, LineIDs: t.LineIDs, At: t.At.UTC(), Status: t.Status}
		if !t.ReadyAt.IsZero() {
			ready := t.ReadyAt.UTC()
			out.Tickets[i].ReadyAt = &ready
		}
	}
	out.Tenders = toTenderResponses(s.Tenders)
	out.Credits = toCreditResponses(s.Credits)
	out.Refunds = make([]refundResponse, len(s.Refunds))
	for i, r := range s.Refunds {
		out.Refunds[i] = refundResponse{Seq: r.Seq, Amount: r.Amount, AgainstPaymentID: r.AgainstPaymentID, Reason: r.Reason}
	}
	out.Invoices = make([]invoiceResponse, len(s.Invoices))
	for i, in := range s.Invoices {
		out.Invoices[i] = invoiceResponse{Seq: in.Seq, InvoiceNo: in.InvoiceNo, Series: in.Series, Reprints: in.Reprints}
	}
	out.Discounts = make([]discountResponse, len(s.Discounts))
	for i, d := range s.Discounts {
		out.Discounts[i] = discountResponse{
			Seq: d.Seq, Scope: d.Scope, LineID: d.LineID, Kind: d.Kind,
			Reason: d.Reason, Value: d.Value,
		}
	}
	out.Tax = toTaxResponse(s.TaxDetail)
	return out
}

func toTenderResponses(tenders []order.Tender) []tenderResponse {
	out := make([]tenderResponse, len(tenders))
	for i, t := range tenders {
		out[i] = tenderResponse{
			ID: t.ID, Method: t.Method, Amount: t.Amount, Tip: t.Tip, Reference: t.Reference,
			Status: t.Status, Refunded: t.Refunded, VoidReason: t.VoidReason,
		}
	}
	return out
}

// toTaxResponse is nil for an untaxed venue.
func toTaxResponse(t tax.TaxResult) *taxResponse {
	if t.Profile == "" {
		return nil
	}
	return &taxResponse{
		Profile: t.Profile, Pricing: t.Pricing, Components: t.Components,
		Taxable: t.Taxable, Tax: t.Tax, Total: t.Total,
	}
}

// ticketSummary is one open ticket, for the till's ticket list: enough to
// tell tickets apart and see which is late, not the bill itself.
type ticketSummary struct {
	ID         string            `json:"id"`
	Status     order.Status      `json:"status"`
	Channel    order.Channel     `json:"channel"`
	Service    order.ServiceType `json:"service"`
	TableID    int64             `json:"table_id,omitempty"`
	Joined     []int64           `json:"joined,omitempty"`
	Covers     int64             `json:"covers"`
	OpenedBy   int64             `json:"opened_by,omitempty"`
	OpenedAt   time.Time         `json:"opened_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Lines      int64             `json:"lines"` // active lines
	Total      money.Amount      `json:"total"`
	BalanceDue money.Amount      `json:"balance_due"`
}

// openTicketsResponse is every ticket not yet closed, oldest first.
type openTicketsResponse struct {
	// LateAfterMinutes is how long a ticket may run before the till shows
	// its time in vermilion: the venue's terminal.ticket_late_minutes.
	LateAfterMinutes int64           `json:"late_after_minutes"`
	Orders           []ticketSummary `json:"orders"`
}

func toOpenTicketsResponse(open svcorder.OpenTickets) openTicketsResponse {
	out := openTicketsResponse{
		LateAfterMinutes: int64(open.LateAfter / time.Minute),
		Orders:           make([]ticketSummary, len(open.Orders)),
	}
	for i, r := range open.Orders {
		st := r.State
		var active int64
		for _, l := range st.Lines {
			if l.Status == order.LineStatusActive {
				active++
			}
		}
		out.Orders[i] = ticketSummary{
			ID: r.PublicID, Status: st.Status, Channel: st.Channel, Service: st.Service, TableID: st.TableID, Joined: st.Joined, Covers: st.Covers,
			OpenedBy: st.OpenedBy, OpenedAt: st.OpenedAt.UTC(), UpdatedAt: st.UpdatedAt.UTC(), Lines: active,
			Total: st.Totals.Total, BalanceDue: st.Totals.BalanceDue,
		}
	}
	return out
}

// nameResponse names a staff member or a device by the internal id events
// carry: actor_id, device_id, authorised_by and the like.
type nameResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// historyResponse is an order's events, oldest first, and every name they
// can mention. time_zone is the venue's, to show the times in.
type historyResponse struct {
	Events   []order.Event  `json:"events"`
	Staff    []nameResponse `json:"staff"`
	Devices  []nameResponse `json:"devices"`
	TimeZone string         `json:"time_zone"`
}

func toHistoryResponse(h svcorder.History) historyResponse {
	names := func(m map[int64]string) []nameResponse {
		out := make([]nameResponse, 0, len(m))
		for id, name := range m {
			out = append(out, nameResponse{ID: id, Name: name})
		}
		slices.SortFunc(out, func(a, b nameResponse) int { return cmp.Compare(a.ID, b.ID) })
		return out
	}
	events := h.Events
	if events == nil {
		events = []order.Event{}
	}
	return historyResponse{Events: events, Staff: names(h.Staff), Devices: names(h.Devices), TimeZone: h.TimeZone}
}
