package api

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/service/charges"
	"github.com/mez-pos/mez/internal/service/tablecodes"
)

// Guests split a bill (phase 11, Task 4g2, ADR-233). A share is an amount of
// what is owed, paid online by one phone:
//
//	GET  /order/api/t/{code}/bill?phone=...   the bill, with how it splits
//	POST /order/api/t/{code}/bill/charge      with a body, a share; with
//	                                          none, the rest, as before
//
// A phone names itself: a string it makes up once and keeps. It says nothing
// of who holds the phone, and is only how the server tells one guest's share
// from another's of the same amount.

// What a share is refused with.
const (
	// CodeBillClaimed: others at the table are paying that part of the bill.
	CodeBillClaimed ErrorCode = "bill_claimed"
	// CodeBillMoved: the bill kept changing while the charge was started.
	CodeBillMoved ErrorCode = "bill_moved"
)

// maxSplitPeople is the most people a bill is split evenly among.
const maxSplitPeople = 20

// phoneName is what a phone may call itself.
var phoneName = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// guestShareRequest is the body of POST /order/api/t/{code}/bill/charge for
// a share: who is paying, and either a figure or how many people the bill is
// split evenly among. With people, the server works out the part.
type guestShareRequest struct {
	Phone  string  `json:"phone"`
	Amount *amount `json:"amount,omitempty"`
	People int     `json:"people,omitempty"`
}

// guestSplitPartResponse is one person's part of the bill split evenly among
// People, as it would be asked for now.
type guestSplitPartResponse struct {
	People int          `json:"people"`
	Amount money.Amount `json:"amount"`
}

// guestSplitResponse is how a bill can be paid online in parts.
type guestSplitResponse struct {
	// Rest is what paying the rest asks for: what is owed less what guests
	// are paying in shares now. It can be nothing while BalanceDue is not.
	Rest money.Amount `json:"rest"`
	// Unclaimed is the most the phone that asked may take as a share: what
	// is owed less what others are paying now.
	Unclaimed money.Amount `json:"unclaimed"`
	// Parts is an equal part for each number of people, from two.
	Parts []guestSplitPartResponse `json:"parts"`
}

// guestPhone is the name a phone gave itself in the bill's address, or
// empty when it gave none. ok is false when it gave one no phone has.
func guestPhone(r *http.Request) (string, bool) {
	phone := r.URL.Query().Get("phone")
	return phone, phone == "" || phoneName.MatchString(phone)
}

// guestSplit is how a table's bill splits for a phone. total is the bill's
// own total, which an equal part is a part of.
func (s *Server) guestSplit(r *http.Request, venueID, tableID int64, phone string, total money.Amount) (*guestSplitResponse, error) {
	rest, err := s.charges.Claims(r.Context(), venueID, tableID, "")
	if err != nil {
		return nil, err
	}
	mine := rest
	if phone != "" {
		if mine, err = s.charges.Claims(r.Context(), venueID, tableID, phone); err != nil {
			return nil, err
		}
	}
	out := &guestSplitResponse{Rest: rest.Unclaimed, Unclaimed: mine.Unclaimed, Parts: make([]guestSplitPartResponse, 0, maxSplitPeople-1)}
	for people := 2; people <= maxSplitPeople; people++ {
		part, err := charges.EqualPart(total, mine.Unclaimed, people)
		if err != nil {
			return nil, err
		}
		out.Parts = append(out.Parts, guestSplitPartResponse{People: people, Amount: part})
	}
	return out, nil
}

// guestStartShare starts a share from a request's body. written is true
// when the body was refused and the refusal has been sent.
func (s *Server) guestStartShare(w http.ResponseWriter, r *http.Request, table tablecodes.Table, description string) (c charges.Charge, written bool, err error) {
	var req guestShareRequest
	if !s.decode(w, r, &req) {
		return charges.Charge{}, true, nil
	}
	refuse := func(field, message string) (charges.Charge, bool, error) {
		writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, message, field)
		return charges.Charge{}, true, nil
	}
	switch {
	case !phoneName.MatchString(req.Phone):
		return refuse("phone", "phone is the name this phone gave itself: 8 to 64 letters, digits, - or _.")
	case (req.Amount == nil) == (req.People == 0):
		return refuse("amount", "A share is either an amount or a number of people, and not both.")
	case req.Amount != nil && (req.Amount.Minor <= 0 || req.Amount.Currency != table.Currency):
		return refuse("amount", "amount is more than nothing, in "+table.Currency+".")
	case req.Amount == nil && (req.People < 2 || req.People > maxSplitPeople):
		return refuse("people", "people is from 2 to 20.")
	}

	var share money.Amount
	if req.Amount != nil {
		share = toMoney(*req.Amount)
	} else {
		ticket, open, err := s.guestOrders.TableTicket(r.Context(), table.VenueID, table.TableID)
		if err != nil {
			return charges.Charge{}, false, err
		}
		if !open {
			return charges.Charge{}, false, charges.ErrNothingToPay
		}
		claims, err := s.charges.Claims(r.Context(), table.VenueID, table.TableID, req.Phone)
		if err != nil {
			return charges.Charge{}, false, err
		}
		if claims.Due.Minor <= 0 {
			return charges.Charge{}, false, charges.ErrNothingToPay
		}
		if share, err = charges.EqualPart(ticket.State.Totals.Total, claims.Unclaimed, req.People); err != nil {
			return charges.Charge{}, false, err
		}
		if share.Minor <= 0 {
			return charges.Charge{}, false, charges.ErrOverClaim
		}
	}
	c, err = s.charges.StartShare(r.Context(), table.VenueID, table.TableID, req.Phone, share, description, s.guestReturnURL(r))
	return c, false, err
}

// writeBillCharge answers a charge just started on a table's bill, the whole
// of what is left or a share, or why there is none.
func (s *Server) writeBillCharge(w http.ResponseWriter, r *http.Request, venueID int64, c charges.Charge, err error) {
	switch {
	case errors.Is(err, charges.ErrNothingToPay):
		writeError(w, s.log, http.StatusConflict, CodeNothingToPay, "There is nothing left to pay on this bill.")
	case errors.Is(err, charges.ErrOverClaim):
		writeError(w, s.log, http.StatusConflict, CodeBillClaimed, "Others at the table are paying that part of the bill. Read the bill again to see what is left.")
	case errors.Is(err, charges.ErrBillMoved):
		writeError(w, s.log, http.StatusConflict, CodeBillMoved, "The bill changed just now. Read it again, then pay.")
	case errors.Is(err, charges.ErrBadShare):
		writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, "That is not an amount of this bill.", "amount")
	default:
		s.writeStartedCharge(w, r, venueID, c, err)
	}
}
