package api

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/byjoellobo/mez/internal/auth"
	"github.com/byjoellobo/mez/internal/domain/kitchen"
	"github.com/byjoellobo/mez/internal/domain/order"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// The kitchen display's API (phase 08, Task 4, ADR-090): a station's board,
// and bump and recall. A device paired as a kitchen display uses it with no
// staff sign-in, since a wall-mounted screen has nobody to type a PIN; the
// events it appends carry the device, and actor 0.

// Kitchen is what the kitchen display needs of the orders. *order.Service
// is the implementation; it is part of Orders.
type Kitchen interface {
	Bump(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.BumpTicket) (svcorder.Result, error)
	Recall(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.BumpTicket) (svcorder.Result, error)
	Start(ctx context.Context, meta svcorder.Meta, orderID int64, cmd svcorder.BumpTicket) (svcorder.Result, error)
	Board(ctx context.Context, venueID int64, station string) (svcorder.Board, error)
}

// kitchenLineResponse is a line as the pass reads it: what to make.
type kitchenLineResponse struct {
	ID        int64            `json:"id"`
	Name      string           `json:"name"`
	Qty       int64            `json:"qty"`
	Course    int64            `json:"course"`
	Modifiers []string         `json:"modifiers"`
	Notes     []string         `json:"notes"`
	Status    order.LineStatus `json:"status"` // a line voided after it was sent shows struck
}

type kitchenTicketResponse struct {
	OrderID   string                `json:"order_id"`
	TicketSeq int64                 `json:"ticket_seq"`
	Station   string                `json:"station"`
	Status    kitchen.TicketStatus  `json:"status"`
	SentAt    time.Time             `json:"sent_at"`
	StartedAt *time.Time            `json:"started_at,omitempty"` // when a cook picked it up, if one has
	ReadyAt   *time.Time            `json:"ready_at,omitempty"`
	Channel   order.Channel         `json:"channel"`
	Service   order.ServiceType     `json:"service"` // dine_in, takeaway or delivery: what the pass boxes up
	TableID   int64                 `json:"table_id,omitempty"`
	Table     string                `json:"table,omitempty"` // its name, and any joined to it: "T4 + T7"
	Covers    int64                 `json:"covers"`
	Notes     []string              `json:"notes"` // the order's own notes: "birthday, candle on the cake"
	Lines     []kitchenLineResponse `json:"lines"`
}

// kitchenStatsResponse is a station's trading day so far (D10, "Today's
// stats"): the tickets it was sent, those still in the kitchen and those put
// on the pass, and how long from sent to ready.
type kitchenStatsResponse struct {
	Sent      int64 `json:"sent"`
	InKitchen int64 `json:"in_kitchen"`
	Ready     int64 `json:"ready"`
	// AverageSeconds is the mean from sent to ready. Zero until one is ready.
	AverageSeconds int64 `json:"average_seconds"`
	// Slowest is the ready ticket that took longest, named by its first dish.
	Slowest *kitchenSlowestResponse `json:"slowest,omitempty"`
}

type kitchenSlowestResponse struct {
	Name    string `json:"name"`
	More    int64  `json:"more"` // the other dishes on the same ticket
	Seconds int64  `json:"seconds"`
}

type kitchenBoardResponse struct {
	Station string                  `json:"station"`
	Tickets []kitchenTicketResponse `json:"tickets"`
	// Stats is the station's trading day so far.
	Stats kitchenStatsResponse `json:"stats"`
	// RecallMinutes is how long a bumped ticket stays to be recalled.
	RecallMinutes int64 `json:"recall_minutes"`
	// LateAfterMinutes is when a ticket is late: its timer turns vermilion.
	LateAfterMinutes int64 `json:"late_after_minutes"`
}

func (s *Server) mountKitchen(mux *http.ServeMux) {
	if s.devices == nil || s.orders == nil {
		return
	}
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(requireKind(s, auth.KindKitchen, h)))
	}
	handle("GET /api/v1/kitchen/{station}/tickets", s.kitchenBoard)
	handle("POST /api/v1/kitchen/orders/{id}/tickets/{seq}/start", s.kitchenMove(s.orders.Start))
	handle("POST /api/v1/kitchen/orders/{id}/tickets/{seq}/bump", s.kitchenMove(s.orders.Bump))
	handle("POST /api/v1/kitchen/orders/{id}/tickets/{seq}/recall", s.kitchenMove(s.orders.Recall))
}

// requireKind lets through only a device of the kind named.
func requireKind(s *Server, kind auth.DeviceKind, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if dev, ok := DeviceFrom(r.Context()); !ok || dev.Kind != kind {
			writeError(w, s.log, http.StatusForbidden, CodeForbidden, "This is for a device paired as a kitchen display.")
			return
		}
		next(w, r)
	})
}

func (s *Server) kitchenBoard(w http.ResponseWriter, r *http.Request) {
	s.writeBoard(w, r, r.PathValue("station"))
}

// kitchenMove starts a ticket, bumps it or recalls it, and answers with
// its station's board as it now stands.
func (s *Server) kitchenMove(move func(context.Context, svcorder.Meta, int64, svcorder.BumpTicket) (svcorder.Result, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderID, ok := s.resolveOrder(w, r)
		if !ok {
			return
		}
		seq, ok := s.pathID(w, r, "seq")
		if !ok {
			return
		}
		res, err := move(r.Context(), Meta(r), orderID, svcorder.BumpTicket{TicketSeq: seq})
		if err != nil {
			s.writeOrderError(w, r, err)
			return
		}
		station := ""
		for _, t := range res.State.Tickets {
			if t.Seq == seq {
				station = t.Station
			}
		}
		s.writeBoard(w, r, station)
	}
}

func (s *Server) writeBoard(w http.ResponseWriter, r *http.Request, station string) {
	if station == "" {
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, "Name the station.")
		return
	}
	board, err := s.orders.Board(r.Context(), Meta(r).VenueID, station)
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	out := kitchenBoardResponse{
		Station: station, Tickets: make([]kitchenTicketResponse, 0, len(board.Tickets)),
		RecallMinutes:    int64(svcorder.RecallWindow / time.Minute),
		LateAfterMinutes: int64(board.LateAfter / time.Minute),
	}
	for _, bt := range board.Tickets {
		out.Tickets = append(out.Tickets, kitchenTicket(bt, board.Tables))
	}
	day := board.Day
	out.Stats = kitchenStatsResponse{Sent: day.Sent, InKitchen: day.InKitchen, Ready: day.Ready, AverageSeconds: int64(day.Average / time.Second)}
	if slow := day.SlowestDish; slow != nil {
		out.Stats.Slowest = &kitchenSlowestResponse{Name: slow.Name, More: slow.More, Seconds: int64(slow.Took / time.Second)}
	}
	writeJSON(w, s.log, http.StatusOK, out)
}

func kitchenTicket(bt svcorder.BoardTicket, tables map[int64]string) kitchenTicketResponse {
	t, o := bt.Ticket, bt.Order
	out := kitchenTicketResponse{
		OrderID: bt.PublicID, TicketSeq: t.Seq, Station: t.Station, Status: t.Status, SentAt: t.At.UTC(),
		Channel: o.Channel, Service: o.Service, TableID: o.TableID, Covers: o.Covers, Notes: []string{}, Lines: []kitchenLineResponse{},
	}
	if !t.StartedAt.IsZero() {
		started := t.StartedAt.UTC()
		out.StartedAt = &started
	}
	if !t.ReadyAt.IsZero() {
		ready := t.ReadyAt.UTC()
		out.ReadyAt = &ready
	}
	for i, id := range o.Tables() {
		name := tables[id]
		if name == "" {
			continue
		}
		if i > 0 {
			out.Table += " + "
		}
		out.Table += name
	}
	for _, n := range o.Notes {
		if n.Scope == order.ScopeOrder {
			out.Notes = append(out.Notes, n.Text)
		}
	}
	for _, l := range o.Lines {
		if !slices.Contains(t.LineIDs, l.ID) {
			continue
		}
		line := kitchenLineResponse{
			ID: l.ID, Name: l.Name, Qty: l.Qty, Course: l.Course, Status: l.Status,
			Modifiers: make([]string, 0, len(l.Modifiers)), Notes: []string{},
		}
		for _, m := range l.Modifiers {
			line.Modifiers = append(line.Modifiers, m.Name)
		}
		for _, n := range o.Notes {
			if n.Scope == order.ScopeLine && n.LineID == l.ID {
				line.Notes = append(line.Notes, n.Text)
			}
		}
		out.Lines = append(out.Lines, line)
	}
	return out
}
