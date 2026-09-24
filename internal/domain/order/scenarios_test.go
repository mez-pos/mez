package order

import (
	"testing"
)

// figures is the money of an order in paise, for comparing a whole Totals at
// a glance. These scenarios run untaxed, so subtotal equals total; the taxed
// ones are in tax_scenarios_test.go.
type figures struct {
	gross, lineDisc, orderDisc, total int64
	paid, refunded, balance, tips     int64
	voided, comped                    int64
}

// checkDues asserts the invariant split-by-item relies on: the active lines'
// Due adds up to the bill's Total.
func checkDues(t *testing.T, s State) {
	t.Helper()
	var dues int64
	for _, l := range s.Lines {
		if l.Status != LineStatusActive && !l.Due.IsZero() {
			t.Errorf("line %d is %s but due %d", l.ID, l.Status, l.Due.Minor)
		}
		dues += l.Due.Minor
	}
	if dues != s.Totals.Total.Minor {
		t.Errorf("lines are due %d, the bill is %d", dues, s.Totals.Total.Minor)
	}
}

func checkFigures(t *testing.T, got Totals, want figures) {
	t.Helper()
	have := figures{
		gross: got.Gross.Minor, lineDisc: got.LineDiscounts.Minor, orderDisc: got.OrderDiscounts.Minor,
		total: got.Total.Minor, paid: got.Paid.Minor, refunded: got.Refunded.Minor,
		balance: got.BalanceDue.Minor, tips: got.Tips.Minor, voided: got.Voided.Minor, comped: got.Comped.Minor,
	}
	if have != want {
		t.Errorf("totals\n got: %+v\nwant: %+v", have, want)
	}
	if got.Tax != inr(0) || got.Subtotal != got.Total {
		t.Errorf("tax %+v and subtotal %+v: want zero tax and subtotal == total", got.Tax, got.Subtotal)
	}
	if got.NetPaid.Minor != got.Paid.Minor-got.Refunded.Minor {
		t.Errorf("net paid %d is not paid %d less refunded %d", got.NetPaid.Minor, got.Paid.Minor, got.Refunded.Minor)
	}
}

// scenarios are whole orders as a cafe or bar would produce them. Each is a
// full stream with the exact figures it must project to. Line IDs and payment
// IDs are the seq of the event that created them, so the comments count.
var scenarios = []struct {
	name     string
	payloads []Payload
	want     figures
	status   Status
	check    func(t *testing.T, s State)
}{
	{
		name: "counter coffee paid in exact cash",
		payloads: []Payload{
			opened(),                // 1
			coffee(2),               // 2: 2 x 220.00
			pay(PaymentCash, 44000), // 3
			invoiceNo("NF/2026-27/000001"),
			closed,
			// The guest comes back for a copy: after the close, no new number.
			InvoiceReprinted{InvoiceNo: "NF/2026-27/000001", Reason: "guest_copy", AuthorisedBy: 2},
		},
		want:   figures{gross: 44000, total: 44000, paid: 44000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			if len(s.Invoices) != 1 || s.Invoices[0].InvoiceNo != "NF/2026-27/000001" || s.ClosedAt.IsZero() {
				t.Errorf("invoices %+v, closed at %v", s.Invoices, s.ClosedAt)
			}
			if s.Invoices[0].Reprints != 1 {
				t.Errorf("reprints = %d, want 1", s.Invoices[0].Reprints)
			}
		},
	},
	{
		name: "table dinner fired by course, card with a tip",
		payloads: []Payload{
			OrderOpened{Channel: ChannelCounter, Currency: "INR", TableID: 4, Covers: 2, OpenedBy: 11},               // 1
			LineAdded{ItemID: 10, Name: "Tomato soup", Qty: 2, UnitPrice: inr(18000), Course: 1, Station: "kitchen"}, // 2: 360.00
			LineAdded{ItemID: 11, Name: "Penne arrabbiata", Qty: 1, UnitPrice: inr(42000), Course: 2, Station: "kitchen",
				Modifiers: []Modifier{{ModifierID: 3, Name: "Extra cheese", Price: inr(6000)}}}, // 3: 480.00
			LineAdded{ItemID: 12, Name: "Sula Shiraz, glass", Qty: 2, UnitPrice: inr(35000), Station: "bar"}, // 4: 700.00
			CourseFired{Course: 1},
			TicketSent{Station: "kitchen", LineIDs: []int64{2}},
			TicketSent{Station: "bar", LineIDs: []int64{4}},
			NoteAdded{Scope: ScopeLine, LineID: 3, Text: "no chilli"},
			CourseFired{Course: 2},
			TicketSent{Station: "kitchen", LineIDs: []int64{3}},
			PaymentTendered{Method: PaymentCard, Amount: inr(154000), Tip: inr(15000), Reference: "AUTH 7731", TenderedBy: 11},
			invoiceNo("NF/2026-27/000002"),
			closed,
		},
		want:   figures{gross: 154000, total: 154000, paid: 154000, tips: 15000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			for _, l := range s.Lines {
				if !l.Sent {
					t.Errorf("line %d was never marked sent", l.ID)
				}
			}
			if !s.Lines[0].CourseFired || !s.Lines[1].CourseFired || s.Lines[2].CourseFired {
				t.Errorf("course fired flags: %v %v %v, want true true false", s.Lines[0].CourseFired, s.Lines[1].CourseFired, s.Lines[2].CourseFired)
			}
			if len(s.Tickets) != 3 || len(s.FiredCourses) != 2 || len(s.Notes) != 1 {
				t.Errorf("tickets %d, fired courses %v, notes %d", len(s.Tickets), s.FiredCourses, len(s.Notes))
			}
		},
	},
	{
		name: "split tender, cash and UPI",
		payloads: []Payload{
			opened(),
			item("Group brunch platter", 1, 100000), // 2
			pay(PaymentCash, 33334),                 // 3
			pay(PaymentUPI, 66666),                  // 4
			closed,
		},
		want:   figures{gross: 100000, total: 100000, paid: 100000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			if len(s.Tenders) != 2 || s.Tenders[1].ID != 4 || s.Tenders[1].Method != PaymentUPI {
				t.Errorf("tenders = %+v", s.Tenders)
			}
		},
	},
	{
		name: "void before payment",
		payloads: []Payload{
			opened(),
			coffee(1),                          // 2: 220.00
			item("Almond croissant", 1, 18000), // 3: 180.00
			LineVoided{LineID: 3, ReasonCode: "out_of_stock", AuthorisedBy: 3},
			pay(PaymentCash, 22000),
			closed,
		},
		want:   figures{gross: 22000, total: 22000, paid: 22000, voided: 18000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			l := s.Lines[1]
			if l.Status != LineStatusVoided || l.ReasonCode != "out_of_stock" || l.AuthorisedBy != 3 || !l.Net.IsZero() {
				t.Errorf("voided line = %+v", l)
			}
		},
	},
	{
		name: "void after payment, refunded before close",
		payloads: []Payload{
			opened(),
			coffee(1),                          // 2: 220.00
			item("Almond croissant", 1, 18000), // 3: 180.00
			pay(PaymentCard, 40000),            // 4
			LineVoided{LineID: 3, ReasonCode: "never_served", AuthorisedBy: 3},
			RefundIssued{Amount: inr(18000), AgainstPaymentID: 4, Reason: "croissant never served"},
			closed,
		},
		want:   figures{gross: 22000, total: 22000, paid: 40000, refunded: 18000, voided: 18000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			tn := s.Tenders[0]
			if tn.Status != PaymentStatusCaptured || tn.Refunded != inr(18000) || s.Totals.NetPaid != inr(22000) {
				t.Errorf("tender %+v, net paid %+v", tn, s.Totals.NetPaid)
			}
		},
	},
	{
		name: "comp for a long wait",
		payloads: []Payload{
			opened(),
			item("Eggs Benedict", 1, 30000), // 2
			coffee(1),                       // 3
			LineComped{LineID: 3, ReasonCode: "long_wait", AuthorisedBy: 3},
			pay(PaymentCash, 30000),
			closed,
		},
		want:   figures{gross: 30000, total: 30000, paid: 30000, comped: 22000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			if s.Lines[1].Status != LineStatusComped || s.Lines[1].Gross != inr(22000) {
				t.Errorf("comped line = %+v", s.Lines[1])
			}
		},
	},
	{
		name: "reopened to add a forgotten item",
		payloads: []Payload{
			opened(),
			coffee(1),               // 2
			pay(PaymentCash, 22000), // 3
			invoiceNo("NF/2026-27/000003"),
			closed,
			OrderReopened{Reason: "forgot the cookie", AuthorisedBy: 3},
			item("Choc chip cookie", 1, 12000), // 7
			pay(PaymentUPI, 12000),             // 8
			invoiceNo("NF/2026-27/000004"),
			closed,
		},
		want:   figures{gross: 34000, total: 34000, paid: 34000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			if s.Reopens != 1 || len(s.Invoices) != 2 || s.LastSeq != 10 {
				t.Errorf("reopens %d, invoices %+v, last seq %d", s.Reopens, s.Invoices, s.LastSeq)
			}
		},
	},
	{
		name: "partial refund after close, through a reopen",
		payloads: []Payload{
			opened(),
			LineAdded{Name: "Tomato soup", Qty: 2, UnitPrice: inr(18000), Station: "kitchen"},      // 2: 360.00
			LineAdded{Name: "Penne arrabbiata", Qty: 1, UnitPrice: inr(42000), Station: "kitchen"}, // 3: 420.00
			pay(PaymentCard, 78000), // 4
			invoiceNo("NF/2026-27/000005"),
			closed,
			OrderReopened{Reason: "one soup served cold", AuthorisedBy: 3},
			LineQtyChanged{LineID: 2, From: 2, To: 1},
			RefundIssued{Amount: inr(18000), AgainstPaymentID: 4, Reason: "soup served cold"},
			closed,
		},
		want:   figures{gross: 60000, total: 60000, paid: 78000, refunded: 18000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			if s.Tenders[0].Status != PaymentStatusCaptured || s.Tenders[0].Refunded != inr(18000) {
				t.Errorf("tender after a partial refund = %+v", s.Tenders[0])
			}
		},
	},
	{
		name: "declined card voided, paid by UPI",
		payloads: []Payload{
			opened(),
			item("Sourdough loaf", 1, 50000), // 2
			pay(PaymentCard, 50000),          // 3
			PaymentVoided{PaymentID: 3, Reason: "card declined after capture", AuthorisedBy: 3},
			pay(PaymentUPI, 50000), // 5
			closed,
		},
		want:   figures{gross: 50000, total: 50000, paid: 50000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			if s.Tenders[0].Status != PaymentStatusVoided || s.Tenders[0].VoidReason == "" || s.Tenders[1].Status != PaymentStatusCaptured {
				t.Errorf("tenders = %+v", s.Tenders)
			}
		},
	},
	{
		name: "line and order discounts round half to even",
		payloads: []Payload{
			opened(),
			item("Macaron", 3, 3350),   // 2: 100.50
			item("Gift box", 1, 12345), // 3: 123.45
			pct(ScopeLine, 2, 1500),    // 15% of 100.50 = 15.075 -> 15.08
			pct(ScopeLine, 3, 1000),    // 10% of 123.45 = 12.345 -> 12.34
			pct(ScopeOrder, 0, 1000),   // 10% of 196.53 = 19.653 -> 19.65
			off(ScopeOrder, 0, 500),    // 5.00 off
			pay(PaymentCard, 17188),
			closed,
		},
		want:   figures{gross: 22395, lineDisc: 2742, orderDisc: 2465, total: 17188, paid: 17188},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			values := []int64{}
			for _, d := range s.Discounts {
				values = append(values, d.Value.Minor)
			}
			want := []int64{1508, 1234, 1965, 500}
			if len(values) != len(want) {
				t.Fatalf("discount values %v, want %v", values, want)
			}
			for i := range want {
				if values[i] != want[i] {
					t.Errorf("discount values %v, want %v", values, want)
					break
				}
			}
		},
	},
	{
		name: "quantity changes carry a percentage discount",
		payloads: []Payload{
			opened(),
			item("Iced latte", 2, 25000),              // 2: 500.00
			pct(ScopeLine, 2, 1000),                   // 10%
			LineQtyChanged{LineID: 2, From: 2, To: 4}, // 1000.00, 100.00 off
			LineQtyChanged{LineID: 2, From: 4, To: 3}, // 750.00, 75.00 off
			pay(PaymentCash, 67500),
			closed,
		},
		want:   figures{gross: 75000, lineDisc: 7500, total: 67500, paid: 67500},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			if l := s.Lines[0]; l.Qty != 3 || l.Discount != inr(7500) || l.Net != inr(67500) {
				t.Errorf("line = %+v", l)
			}
		},
	},
	{
		name: "QR guest order comped in full closes at zero",
		payloads: []Payload{
			OrderOpened{Channel: ChannelQRTable, Currency: "INR", TableID: 9, Covers: 1}, // a guest, no staff
			item("Masala chai", 1, 15000), // 2
			LineComped{LineID: 2, ReasonCode: "birthday", AuthorisedBy: 3},
			closed,
		},
		want:   figures{comped: 15000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			if s.OpenedBy != 0 || len(s.Tenders) != 0 {
				t.Errorf("opened by %d with %d tenders, want a guest and none", s.OpenedBy, len(s.Tenders))
			}
		},
	},
	{
		name: "open tab mid-service, part paid",
		payloads: []Payload{
			opened(),
			item("Kingfisher pint", 3, 32000), // 2: 960.00
			pay(PaymentCash, 50000),
			item("Fries", 1, 19000), // 4
		},
		want:   figures{gross: 115000, total: 115000, paid: 50000, balance: 65000},
		status: StatusSettling,
	},
	{
		name: "held while the guest finds a friend, resumed, a note on hold",
		payloads: []Payload{
			opened(),
			coffee(1), // 2: 220.00
			OrderHeld{},
			NoteAdded{Scope: ScopeOrder, Text: "back in five"},
			OrderResumed{},
			coffee(1), // 6
			pay(PaymentCash, 44000),
			closed,
		},
		want:   figures{gross: 44000, total: 44000, paid: 44000},
		status: StatusClosed,
	},
	{
		name: "guest changes their milk before the coffee is made",
		payloads: []Payload{
			opened(),
			LineAdded{ItemID: 5, Name: "Flat white", Qty: 2, UnitPrice: inr(21000), Course: 1, Station: "bar",
				Modifiers: []Modifier{{ModifierID: 3, Name: "Dairy", Price: inr(0)}}}, // 2: 420.00
			LineModified{LineID: 2,
				From: []Modifier{{ModifierID: 3, Name: "Dairy", Price: inr(0)}},
				To:   []Modifier{{ModifierID: 4, Name: "Oat", Price: inr(5000)}}}, // 2 x 260.00
			TicketSent{Station: "bar", LineIDs: []int64{2}},
			pay(PaymentUPI, 52000),
			closed,
		},
		want:   figures{gross: 52000, total: 52000, paid: 52000},
		status: StatusClosed,
		check: func(t *testing.T, s State) {
			if m := s.Lines[0].Modifiers; len(m) != 1 || m[0].Name != "Oat" {
				t.Errorf("the ticket went out with %+v, want oat", m)
			}
		},
	},
}

func TestScenarios(t *testing.T) {
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			s := mustProject(t, sc.payloads...)
			checkFigures(t, s.Totals, sc.want)
			checkDues(t, s)
			if s.Status != sc.status {
				t.Errorf("status = %q, want %q", s.Status, sc.status)
			}
			if s.LastSeq != int64(len(sc.payloads)) {
				t.Errorf("last seq = %d, want %d", s.LastSeq, len(sc.payloads))
			}
			if sc.check != nil {
				sc.check(t, s)
			}
		})
	}
}

// TestScenariosCoverEveryType keeps the scenarios honest: every event type
// this build knows must appear in at least one realistic order.
func TestScenariosCoverEveryType(t *testing.T) {
	seen := map[Type]bool{}
	for _, sc := range scenarios {
		for _, p := range sc.payloads {
			seen[p.EventType()] = true
		}
	}
	for _, typ := range Types() {
		if !seen[typ] {
			t.Errorf("no scenario uses %q", typ)
		}
	}
	if len(scenarios) < 10 {
		t.Errorf("%d scenarios; phase 03 asks for at least ten", len(scenarios))
	}
}
