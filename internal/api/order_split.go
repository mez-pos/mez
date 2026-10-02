package api

import (
	"net/http"

	"github.com/byjoellobo/mez/internal/domain/money"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// The split error codes (phase 07, Task 2).
const (
	CodeSplitInvalid       ErrorCode = "split_invalid"
	CodeSplitOverAllocated ErrorCode = "split_over_allocated"
	CodeSplitUnassigned    ErrorCode = "split_unassigned"
	CodeSplitMismatch      ErrorCode = "split_mismatch"
	CodeSplitAfterPayment  ErrorCode = "split_after_payment"
)

type splitRequest = svcorder.SplitRequest

// splitResponse is how a bill divides: one share per payer, whose amounts
// sum to Total exactly (payment.Split.Check). Total is the balance for an
// even or amount split, the whole bill for one by item or guest.
type splitResponse struct {
	Kind   svcorder.SplitKind `json:"kind"`
	Total  money.Amount       `json:"total"`
	Shares []shareResponse    `json:"shares"`
}

// shareResponse is one payer's part. Payer is 0-based; by guest, payer i is
// seat i+1. LineIDs are the lines it covers, whole or in part.
type shareResponse struct {
	Payer   int          `json:"payer"`
	Amount  money.Amount `json:"amount"`
	LineIDs []int64      `json:"line_ids"`
}

// splitOrder answers how a bill divides. It records nothing, so it takes
// no idempotency key: each share is paid by its own tender.
func (s *Server) splitOrder(w http.ResponseWriter, r *http.Request) {
	var req splitRequest
	if !s.decode(w, r, &req) {
		return
	}
	orderID, ok := s.resolveOrder(w, r)
	if !ok {
		return
	}
	sp, err := s.orders.Split(r.Context(), Meta(r).VenueID, orderID, req)
	if err != nil {
		s.writeOrderError(w, r, err)
		return
	}
	res := splitResponse{Kind: req.Kind, Total: sp.Total, Shares: make([]shareResponse, len(sp.Shares))}
	for i, sh := range sp.Shares {
		ids := sh.LineIDs
		if ids == nil {
			ids = []int64{}
		}
		res.Shares[i] = shareResponse{Payer: sh.Payer, Amount: sh.Amount, LineIDs: ids}
	}
	writeJSON(w, s.log, http.StatusOK, res)
}
