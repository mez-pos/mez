package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// requestsWorld is orderingWorld with a member of staff signed in at the
// terminal, who can see and answer what guests ask.
type requestsWorld struct {
	*orderingWorld
	staffToken string
}

func newRequestsWorld(t *testing.T) *requestsWorld {
	t.Helper()
	o := newOrderingWorld(t)
	app := fstest.MapFS{"index.html": {Data: []byte("<title>the guest app</title>")}}
	o.rebuild(t, WithMenus(o.db), WithGuests(o.codes, "/order/"), WithGuestOrders(o.orders), WithGuestRequests(o.orders),
		WithStatic(map[string]fs.FS{"/order/": app}))
	return &requestsWorld{orderingWorld: o, staffToken: o.signIn(t, "2468").Token}
}

func (w *requestsWorld) staffCall(method, path string) *httptest.ResponseRecorder {
	return send(w.handler, method, path, "", w.asStaff(w.staffToken))
}

// ask places a request from table 4 and returns its id.
func (w *requestsWorld) ask(t *testing.T, key string) string {
	t.Helper()
	rec := w.place(key, 2)
	if rec.Code != http.StatusCreated {
		t.Fatalf("placing: %d %s", rec.Code, rec.Body)
	}
	return decodeRound(t, rec).ID
}

func (w *requestsWorld) waiting(t *testing.T) []guestRequestResponse {
	t.Helper()
	rec := w.staffCall(http.MethodGet, "/api/v1/guest-requests")
	if rec.Code != http.StatusOK {
		t.Fatalf("listing: %d %s", rec.Code, rec.Body)
	}
	var got guestRequestsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body, err)
	}
	return got.Requests
}

func decodeDecision(t *testing.T, rec *httptest.ResponseRecorder) guestDecisionResponse {
	t.Helper()
	var got guestDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body, err)
	}
	return got
}

func TestGuestRequestsAreListedAndAccepted(t *testing.T) {
	w := newRequestsWorld(t)
	if got := w.waiting(t); got == nil || len(got) != 0 {
		t.Fatalf("with nothing asked: %#v, want an empty list", got)
	}
	id := w.ask(t, "k1")

	got := w.waiting(t)
	if len(got) != 1 {
		t.Fatalf("%d waiting, want 1", len(got))
	}
	req := got[0]
	if req.ID != id || req.TableID != 4 || req.TableName != "4" || req.PlacedAt.IsZero() || req.Subtotal.Minor != 44000 ||
		len(req.Lines) != 1 || req.Lines[0].Name != "Flat white" || req.Lines[0].Qty != 2 || req.Lines[0].Choices[0] != "Oat" {
		t.Errorf("request = %+v", req)
	}

	rec := w.staffCall(http.MethodPost, "/api/v1/guest-requests/"+id+"/accept")
	if rec.Code != http.StatusOK {
		t.Fatalf("accepting: %d %s", rec.Code, rec.Body)
	}
	decision := decodeDecision(t, rec)
	if decision.ID != id || decision.Status != svcorder.RoundSent || decision.TableID != 4 || decision.OrderID == "" {
		t.Errorf("decision = %+v", decision)
	}
	if len(w.waiting(t)) != 0 {
		t.Error("the accepted request is still listed")
	}

	orderID, err := w.orders.Resolve(context.Background(), 1, decision.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.orders.Order(context.Background(), 1, orderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.State.Lines) != 1 || !res.State.Lines[0].Sent || res.State.OpenedBy == 0 {
		t.Errorf("the ticket: lines %+v, opened by %d", res.State.Lines, res.State.OpenedBy)
	}

	// Tapped twice, it is the same answer.
	again := w.staffCall(http.MethodPost, "/api/v1/guest-requests/"+id+"/accept")
	if again.Code != http.StatusOK || decodeDecision(t, again).OrderID != decision.OrderID {
		t.Errorf("accepting twice: %d %s", again.Code, again.Body)
	}
	declined := w.staffCall(http.MethodPost, "/api/v1/guest-requests/"+id+"/decline")
	if declined.Code != http.StatusConflict || errorOf(t, declined).Code != CodeRequestDecided {
		t.Errorf("declining an accepted request: %d %s", declined.Code, declined.Body)
	}
}

func TestGuestRequestIsDeclined(t *testing.T) {
	w := newRequestsWorld(t)
	id := w.ask(t, "k1")

	rec := w.staffCall(http.MethodPost, "/api/v1/guest-requests/"+id+"/decline")
	if rec.Code != http.StatusOK {
		t.Fatalf("declining: %d %s", rec.Code, rec.Body)
	}
	if got := decodeDecision(t, rec); got.Status != svcorder.RoundDeclined || got.OrderID != "" {
		t.Errorf("decision = %+v", got)
	}
	if len(w.waiting(t)) != 0 {
		t.Error("the declined request is still listed")
	}
	accepted := w.staffCall(http.MethodPost, "/api/v1/guest-requests/"+id+"/accept")
	if accepted.Code != http.StatusConflict || errorOf(t, accepted).Code != CodeRequestDecided {
		t.Errorf("accepting a declined request: %d %s", accepted.Code, accepted.Body)
	}
	// The guest's phone, asking again with its key, is told.
	if again := w.place("k1", 2); again.Code != http.StatusOK || decodeRound(t, again).Status != svcorder.RoundDeclined {
		t.Errorf("the guest's retry: %d %s", again.Code, again.Body)
	}
}

func TestGuestRequestsAreStaffOnly(t *testing.T) {
	w := newRequestsWorld(t)
	id := w.ask(t, "k1")

	tests := []struct {
		name   string
		method string
		path   string
		header map[string]string
		status int
	}{
		{"nobody lists", http.MethodGet, "/api/v1/guest-requests", nil, http.StatusUnauthorized},
		{"a device with nobody signed in lists", http.MethodGet, "/api/v1/guest-requests", w.asDevice(), http.StatusUnauthorized},
		{"nobody accepts", http.MethodPost, "/api/v1/guest-requests/" + id + "/accept", nil, http.StatusUnauthorized},
		{"a guest reaches for it under their own mount", http.MethodPost, "/order/api/v1/guest-requests/" + id + "/accept", nil, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := send(w.handler, tt.method, tt.path, "", tt.header); rec.Code != tt.status {
				t.Errorf("status %d, want %d: %s", rec.Code, tt.status, rec.Body)
			}
		})
	}
	if len(w.waiting(t)) != 1 {
		t.Error("the request was answered by someone who may not")
	}

	unknown := w.staffCall(http.MethodPost, "/api/v1/guest-requests/rnd_NOPE0000/accept")
	if unknown.Code != http.StatusNotFound || errorOf(t, unknown).Code != CodeRequestNotFound {
		t.Errorf("a request that does not exist: %d %s", unknown.Code, unknown.Body)
	}
}
