package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/payment"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// Orders is what the API needs from the order service.
// *service/order.Service is the implementation.
type Orders interface {
	Resolve(ctx context.Context, venueID int64, publicID string) (int64, error)
	Order(ctx context.Context, venueID, orderID int64) (svcorder.Result, error)
	OpenOrders(ctx context.Context, venueID int64) (svcorder.OpenTickets, error)
	OrderHistory(ctx context.Context, venueID, orderID int64) (svcorder.History, error)
	Split(ctx context.Context, venueID, orderID int64, req svcorder.SplitRequest) (payment.Split, error)
	Invoice(ctx context.Context, venueID, orderID, seq int64) (svcorder.IssuedInvoice, error)
	Reprint(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.ReprintInvoice) (svcorder.Result, error)
	PlanCredit(ctx context.Context, venueID, orderID int64, req order.CreditRequest) (order.CreditPlan, error)
	PastBills(ctx context.Context, venueID int64, query string) ([]svcorder.Result, error)
	Credit(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.IssueCredit) (svcorder.Result, error)
	Hold(ctx context.Context, meta svcorder.Meta, orderID int64) (svcorder.Result, error)
	MoveTable(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.MoveTable) (svcorder.Result, error)
	JoinTables(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.JoinTables) (svcorder.Result, error)
	PartTables(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.PartTables) (svcorder.Result, error)
	Resume(ctx context.Context, meta svcorder.Meta, orderID int64) (svcorder.Result, error)

	Open(ctx context.Context, meta svcorder.Meta, cmd svcorder.OpenOrder) (svcorder.Result, error)
	AddLine(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.AddLine) (svcorder.Result, error)
	ChangeQty(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.ChangeQty) (svcorder.Result, error)
	ModifyLine(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.ModifyLine) (svcorder.Result, error)
	VoidLine(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.VoidLine) (svcorder.Result, error)
	RemoveLine(ctx context.Context, meta svcorder.Meta, orderID, lineID int64) (svcorder.Result, error)
	CompLine(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.CompLine) (svcorder.Result, error)
	ApplyDiscount(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.ApplyDiscount) (svcorder.Result, error)
	AddNote(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.AddNote) (svcorder.Result, error)
	FireCourse(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.FireCourse) (svcorder.Result, error)
	Send(ctx context.Context, meta svcorder.Meta, orderID int64) (svcorder.Result, error)
	// The kitchen display's commands and read (ADR-090).
	Kitchen
	SendTicket(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.SendTicket) (svcorder.Result, error)
	TenderPayment(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.TenderPayment) (svcorder.Result, error)
	VoidPayment(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.VoidPayment) (svcorder.Result, error)
	Refund(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.Refund) (svcorder.Result, error)
	IssueInvoice(ctx context.Context, meta svcorder.Meta, orderID int64) (svcorder.Result, error)
	Close(ctx context.Context, meta svcorder.Meta, orderID int64) (svcorder.Result, error)
	Reopen(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.Reopen) (svcorder.Result, error)
}

// WithOrders mounts the order endpoints. They need WithDevices and
// WithStaff: every one of them is a signed-in staff member on a paired
// device, and without those the routes are not mounted at all.
func WithOrders(o Orders) Option {
	return func(s *Server) { s.orders = o }
}

// mountOrders routes /api/v1/orders.
//
// Every route is the same shape: the permission it needs, then a handler
// that decodes, calls one service method and encodes. The overrides are on
// the five actions a cashier must not do alone.
func (s *Server) mountOrders(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.orders == nil {
		return
	}
	post := func(pattern string, p auth.Permission, h http.HandlerFunc) {
		mux.Handle("POST "+pattern, s.requireDevice(s.requireStaff(s.requirePermission(p, h))))
		mux.HandleFunc(pattern, s.methodNotAllowed)
	}

	// The literal path wins over {id}: an order's id is never "closed".
	mux.Handle("GET /api/v1/orders/closed", s.requireDevice(s.requireStaff(
		s.requirePermission(auth.PermOrderOpen, http.HandlerFunc(s.listPastBills)))))
	mux.Handle("GET /api/v1/orders/{id}", s.requireDevice(s.requireStaff(
		s.requirePermission(auth.PermOrderOpen, http.HandlerFunc(s.readOrder)))))
	mux.Handle("POST /api/v1/orders", s.requireDevice(s.requireStaff(
		s.requirePermission(auth.PermOrderOpen, http.HandlerFunc(s.openOrder)))))
	mux.Handle("GET /api/v1/orders", s.requireDevice(s.requireStaff(
		s.requirePermission(auth.PermOrderOpen, http.HandlerFunc(s.listOpenOrders)))))
	mux.HandleFunc("/api/v1/orders", s.methodNotAllowed)
	mux.HandleFunc("/api/v1/orders/{id}", s.methodNotAllowed)
	mux.Handle("GET /api/v1/orders/{id}/events", s.requireDevice(s.requireStaff(
		s.requirePermission(auth.PermOrderOpen, http.HandlerFunc(s.orderHistory)))))
	mux.HandleFunc("/api/v1/orders/{id}/events", s.methodNotAllowed)
	mux.Handle("GET /api/v1/orders/{id}/invoices/{seq}", s.requireDevice(s.requireStaff(
		s.requirePermission(auth.PermPaymentTake, http.HandlerFunc(s.readInvoice)))))
	mux.HandleFunc("/api/v1/orders/{id}/invoices/{seq}", s.methodNotAllowed)

	// What the cashier at the till may do on their own.
	post("/api/v1/orders/{id}/lines", auth.PermOrderOpen, s.addLine)
	post("/api/v1/orders/{id}/lines/{line}/qty", auth.PermOrderEdit, s.changeQty)
	post("/api/v1/orders/{id}/lines/{line}/modifiers", auth.PermOrderEdit, s.modifyLine)
	post("/api/v1/orders/{id}/lines/{line}/remove", auth.PermOrderEdit, s.removeLine)
	post("/api/v1/orders/{id}/notes", auth.PermOrderEdit, s.addNote)
	post("/api/v1/orders/{id}/hold", auth.PermOrderEdit, s.holdOrder)
	// Where the party sits (ADR-088).
	post("/api/v1/orders/{id}/table", auth.PermOrderEdit, s.moveTable)
	post("/api/v1/orders/{id}/tables/join", auth.PermOrderEdit, s.joinTables)
	post("/api/v1/orders/{id}/tables/part", auth.PermOrderEdit, s.partTables)
	post("/api/v1/orders/{id}/resume", auth.PermOrderEdit, s.resumeOrder)
	post("/api/v1/orders/{id}/courses/{course}/fire", auth.PermOrderEdit, s.fireCourse)
	post("/api/v1/orders/{id}/tickets", auth.PermOrderEdit, s.sendTicket)
	// The till's Send: what is waiting, each station its own (ADR-089).
	post("/api/v1/orders/{id}/send", auth.PermOrderEdit, s.sendOrder)
	post("/api/v1/orders/{id}/split", auth.PermPaymentTake, s.splitOrder)
	post("/api/v1/orders/{id}/payments", auth.PermPaymentTake, s.tenderPayment)
	post("/api/v1/orders/{id}/invoice", auth.PermPaymentTake, s.issueInvoice)
	post("/api/v1/orders/{id}/close", auth.PermPaymentTake, s.closeOrder)

	// What takes a manager. These ask only that the caller is working the
	// register: the permission that matters is the authoriser's, checked
	// against their PIN inside the handler. Requiring it of the caller too
	// would mean a cashier could never void, manager or no manager, which
	// is the opposite of what an override is for.
	post("/api/v1/orders/{id}/lines/{line}/void", auth.PermOrderOpen, s.voidLine)
	post("/api/v1/orders/{id}/lines/{line}/comp", auth.PermOrderOpen, s.compLine)
	post("/api/v1/orders/{id}/discounts", auth.PermOrderOpen, s.applyDiscount)
	post("/api/v1/orders/{id}/payments/{payment}/void", auth.PermOrderOpen, s.voidPayment)
	post("/api/v1/orders/{id}/refunds", auth.PermOrderOpen, s.refund)
	post("/api/v1/orders/{id}/reopen", auth.PermOrderOpen, s.reopenOrder)
	post("/api/v1/orders/{id}/invoices/{seq}/reprint", auth.PermOrderOpen, s.reprintInvoice)
	// A credit note's figures before approval, and the credit note itself,
	// with its refunds (ADR-083). Issuing needs a manager's refund.give.
	post("/api/v1/orders/{id}/credit-notes/preview", auth.PermOrderOpen, s.previewCredit)
	post("/api/v1/orders/{id}/credit-notes", auth.PermOrderOpen, s.issueCredit)
}

// The request bodies. Those that are exactly a service command are that
// command; the rest add the manager's override, or take an ID from the path
// rather than the body.
type (
	openOrderRequest  = svcorder.OpenOrder
	addLineRequest    = svcorder.AddLine
	addNoteRequest    = svcorder.AddNote
	sendTicketRequest = svcorder.SendTicket
	tenderRequest     = svcorder.TenderPayment
)

type changeQtyRequest struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// modifyLineRequest replaces a line's modifiers. From is what the terminal
// believes the line has; a stale one is refused with modifiers_mismatch.
type modifyLineRequest struct {
	From []order.Modifier `json:"from"`
	To   []order.Modifier `json:"to"`
}

type voidLineRequest struct {
	Override
	ReasonCode string `json:"reason_code"`
}

type discountRequest struct {
	Override
	Scope       order.Scope        `json:"scope"`
	LineID      int64              `json:"line_id,omitempty"`
	Kind        order.DiscountKind `json:"kind"`
	BasisPoints int32              `json:"basis_points,omitempty"`
	Amount      *amount            `json:"amount,omitempty"`
	Reason      string             `json:"reason"`
}

// amount mirrors money.Amount for a request body, so a discount can leave
// it out entirely.
type amount struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
}

type voidPaymentRequest struct {
	Override
	Reason string `json:"reason"`
}

type refundRequest struct {
	Override
	Amount           amount `json:"amount"`
	AgainstPaymentID int64  `json:"against_payment_id"`
	Reason           string `json:"reason"`
}

type reopenRequest struct {
	Override
	Reason string `json:"reason"`
}

// readOrder is an order as it stands.
func (s *Server) readOrder(w http.ResponseWriter, r *http.Request) {
	orderID, ok := s.resolveOrder(w, r)
	if !ok {
		return
	}
	res, err := s.orders.Order(r.Context(), Meta(r).VenueID, orderID)
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, toOrderResponse(res))
}

// listOpenOrders answers with every ticket not yet closed, for the till's
// ticket list, and how long one may run before it shows as late.
func (s *Server) listOpenOrders(w http.ResponseWriter, r *http.Request) {
	open, err := s.orders.OpenOrders(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, toOpenTicketsResponse(open))
}

// orderHistory answers with an order's events and the names they mention,
// for the till's timeline. Read-only; the events are the record.
func (s *Server) orderHistory(w http.ResponseWriter, r *http.Request) {
	orderID, ok := s.resolveOrder(w, r)
	if !ok {
		return
	}
	h, err := s.orders.OrderHistory(r.Context(), Meta(r).VenueID, orderID)
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, toHistoryResponse(h))
}

// holdOrder and resumeOrder take no body (ADR-074).
func (s *Server) holdOrder(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		return s.orders.Hold(r.Context(), meta, orderID)
	})
}

func (s *Server) resumeOrder(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		return s.orders.Resume(r.Context(), meta, orderID)
	})
}

func (s *Server) openOrder(w http.ResponseWriter, r *http.Request) {
	var req openOrderRequest
	if !s.decode(w, r, &req) {
		return
	}
	res, err := s.orders.Open(r.Context(), Meta(r), req)
	s.writeCommand(w, r, res, err, http.StatusCreated)
}

func (s *Server) addLine(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req addLineRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.AddLine(r.Context(), meta, orderID, req)
	})
}

func (s *Server) changeQty(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req changeQtyRequest
		lineID, ok := s.pathID(w, r, "line")
		if !ok || !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.ChangeQty(r.Context(), meta, orderID, svcorder.ChangeQty{
			LineID: lineID, From: req.From, To: req.To,
		})
	})
}

func (s *Server) modifyLine(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req modifyLineRequest
		lineID, ok := s.pathID(w, r, "line")
		if !ok || !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.ModifyLine(r.Context(), meta, orderID, svcorder.ModifyLine{
			LineID: lineID, From: req.From, To: req.To,
		})
	})
}

// removeLine takes off a line rung up by mistake, before anything has
// happened to it, with no manager (ADR-073). It takes no body.
func (s *Server) removeLine(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		lineID, ok := s.pathID(w, r, "line")
		if !ok {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.RemoveLine(r.Context(), meta, orderID, lineID)
	})
}

func (s *Server) voidLine(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req voidLineRequest
		lineID, ok := s.pathID(w, r, "line")
		if !ok || !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		by, ok := s.authorise(w, r, req.Override, auth.PermLineVoid)
		if !ok {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.VoidLine(r.Context(), meta, orderID, svcorder.VoidLine{
			LineID: lineID, ReasonCode: req.ReasonCode, AuthorisedBy: by.ID,
		})
	})
}

func (s *Server) compLine(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req voidLineRequest
		lineID, ok := s.pathID(w, r, "line")
		if !ok || !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		by, ok := s.authorise(w, r, req.Override, auth.PermLineComp)
		if !ok {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.CompLine(r.Context(), meta, orderID, svcorder.CompLine{
			LineID: lineID, ReasonCode: req.ReasonCode, AuthorisedBy: by.ID,
		})
	})
}

func (s *Server) applyDiscount(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req discountRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		by, ok := s.authorise(w, r, req.Override, auth.PermDiscountGive)
		if !ok {
			return svcorder.Result{}, errAnswered
		}
		cmd := svcorder.ApplyDiscount{
			Scope: req.Scope, LineID: req.LineID, Kind: req.Kind,
			BasisPoints: req.BasisPoints, Reason: req.Reason, AuthorisedBy: by.ID,
		}
		if req.Amount != nil {
			value := toMoney(*req.Amount)
			cmd.Amount = &value
		}
		return s.orders.ApplyDiscount(r.Context(), meta, orderID, cmd)
	})
}

func (s *Server) addNote(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req addNoteRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.AddNote(r.Context(), meta, orderID, req)
	})
}

type (
	moveTableRequest  = svcorder.MoveTable
	joinTablesRequest = svcorder.JoinTables
	partTablesRequest = svcorder.PartTables
)

func (s *Server) moveTable(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req moveTableRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.MoveTable(r.Context(), meta, orderID, req)
	})
}

func (s *Server) joinTables(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req joinTablesRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.JoinTables(r.Context(), meta, orderID, req)
	})
}

func (s *Server) partTables(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req partTablesRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.PartTables(r.Context(), meta, orderID, req)
	})
}

func (s *Server) sendOrder(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		return s.orders.Send(r.Context(), meta, orderID)
	})
}

func (s *Server) fireCourse(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		course, ok := s.pathID(w, r, "course")
		if !ok {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.FireCourse(r.Context(), meta, orderID, svcorder.FireCourse{Course: course})
	})
}

func (s *Server) sendTicket(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req sendTicketRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.SendTicket(r.Context(), meta, orderID, req)
	})
}

func (s *Server) tenderPayment(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req tenderRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.TenderPayment(r.Context(), meta, orderID, req)
	})
}

func (s *Server) voidPayment(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req voidPaymentRequest
		paymentID, ok := s.pathID(w, r, "payment")
		if !ok || !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		by, ok := s.authorise(w, r, req.Override, auth.PermPaymentVoid)
		if !ok {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.VoidPayment(r.Context(), meta, orderID, svcorder.VoidPayment{
			PaymentID: paymentID, Reason: req.Reason, AuthorisedBy: by.ID,
		})
	})
}

func (s *Server) refund(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req refundRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		by, ok := s.authorise(w, r, req.Override, auth.PermRefundGive)
		if !ok {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.Refund(r.Context(), meta, orderID, svcorder.Refund{
			Amount: toMoney(req.Amount), AgainstPaymentID: req.AgainstPaymentID, Reason: req.Reason, AuthorisedBy: by.ID,
		})
	})
}

func (s *Server) issueInvoice(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		return s.orders.IssueInvoice(r.Context(), meta, orderID)
	})
}

func (s *Server) closeOrder(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		return s.orders.Close(r.Context(), meta, orderID)
	})
}

func (s *Server) reopenOrder(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, func(orderID int64, meta svcorder.Meta) (svcorder.Result, error) {
		var req reopenRequest
		if !s.decode(w, r, &req) {
			return svcorder.Result{}, errAnswered
		}
		by, ok := s.authorise(w, r, req.Override, auth.PermOrderReopen)
		if !ok {
			return svcorder.Result{}, errAnswered
		}
		return s.orders.Reopen(r.Context(), meta, orderID, svcorder.Reopen{
			Reason: req.Reason, AuthorisedBy: by.ID,
		})
	})
}

// errAnswered means a handler has already written the response. It never
// reaches a client: command checks for it and returns.
var errAnswered = errors.New("api: the client has been answered")

// command is the shape every mutating order endpoint shares: resolve the
// order from its public ID, run one service call, and encode the result.
func (s *Server) command(w http.ResponseWriter, r *http.Request, run func(orderID int64, meta svcorder.Meta) (svcorder.Result, error)) {
	orderID, ok := s.resolveOrder(w, r)
	if !ok {
		return
	}
	res, err := run(orderID, Meta(r))
	if errors.Is(err, errAnswered) {
		return
	}
	s.writeCommand(w, r, res, err, http.StatusOK)
}

// writeCommand encodes a service result, or maps its error.
func (s *Server) writeCommand(w http.ResponseWriter, r *http.Request, res svcorder.Result, err error, status int) {
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	// A replayed command created nothing this time, whatever it created the
	// first time, so it answers 200 rather than 201.
	if res.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, s.log, status, toCommandResponse(res))
}

// resolveOrder turns the {id} in the path into the order's internal ID, at
// the caller's venue.
func (s *Server) resolveOrder(w http.ResponseWriter, r *http.Request) (int64, bool) {
	orderID, err := s.orders.Resolve(r.Context(), Meta(r).VenueID, r.PathValue("id"))
	if err != nil {
		s.writeOrderError(w, r, err)
		return 0, false
	}
	return orderID, true
}

// pathID reads a positive integer out of the path.
func (s *Server) pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id < 1 {
		writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest,
			"that is not a "+name+" id", name)
		return 0, false
	}
	return id, true
}

// decode reads a JSON body, answering the client itself if it cannot.
func (s *Server) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := decodeJSON(w, r, dst); err != nil {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, err.Error())
		return false
	}
	return true
}

func toMoney(a amount) money.Amount {
	return money.Amount{Minor: a.Minor, Currency: a.Currency}
}
