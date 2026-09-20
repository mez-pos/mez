package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/invoice"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// untaxed is a venue with no tax profile.
var untaxed = order.Taxes{}

// gstCafe is a Bengaluru cafe pricing GST exclusive.
func gstCafe() order.Taxes {
	return order.Taxes{Profile: tax.GST{}, Venue: tax.VenueContext{
		Currency: "INR", Pricing: tax.Exclusive, GSTIN: "29ABCDE1234F1Z5", StateCode: "29",
	}}
}

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

// ev is an event as a command would build it: the store numbers it.
func ev(p order.Payload) order.Event { return order.NewEvent(0, at, 1, 1, p) }

// emit is a command that appends its payloads whatever the order holds.
func emit(payloads ...order.Payload) Decide {
	return func(context.Context, *OrderTx) ([]order.Event, error) {
		events := make([]order.Event, len(payloads))
		for i, p := range payloads {
			events[i] = ev(p)
		}
		return events, nil
	}
}

func dish(n int) order.LineAdded {
	return order.LineAdded{
		ItemID: int64(n), Name: fmt.Sprintf("Dish %d", n), Qty: 1, UnitPrice: inr(int64(10000 + n)),
		Station: "kitchen", TaxRate: 5000, HSN: "996331",
	}
}

func openTestOrder(t *testing.T, d *DB, publicID string, taxes order.Taxes) int64 {
	t.Helper()
	id, _, err := d.OpenOrder(context.Background(), NewOrder{
		PublicID: publicID,
		VenueID:  1,
		Opened:   ev(order.OrderOpened{Channel: order.ChannelCounter, Currency: "INR", TableID: 4, Covers: 2, OpenedBy: 1}),
	}, taxes)
	if err != nil {
		t.Fatalf("OpenOrder: %v", err)
	}
	return id
}

func mustAppend(t *testing.T, d *DB, orderID int64, taxes order.Taxes, decide Decide) order.State {
	t.Helper()
	s, _, err := d.AppendOrder(context.Background(), orderID, taxes, decide)
	if err != nil {
		t.Fatalf("AppendOrder: %v", err)
	}
	return s
}

// checkLoads asserts that the snapshot path, the replay path and the state
// an append returned all agree.
func checkLoads(t *testing.T, d *DB, orderID int64, taxes order.Taxes, want order.State) {
	t.Helper()
	ctx := context.Background()
	loaded, err := d.LoadOrder(ctx, orderID, taxes)
	if err != nil {
		t.Fatalf("LoadOrder: %v", err)
	}
	rebuilt, err := d.RebuildOrder(ctx, orderID, taxes)
	if err != nil {
		t.Fatalf("RebuildOrder: %v", err)
	}
	if !reflect.DeepEqual(rebuilt, want) {
		t.Fatalf("at seq %d, replay from events disagrees with the append:\n got %+v\nwant %+v", want.LastSeq, rebuilt, want)
	}
	if !reflect.DeepEqual(loaded, rebuilt) {
		t.Fatalf("at seq %d, snapshot plus tail disagrees with replay:\n got %+v\nwant %+v", want.LastSeq, loaded, rebuilt)
	}
}

func TestOpenAppendAndReadAnOrder(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)

	orderID, opened, err := d.OpenOrder(ctx, NewOrder{
		PublicID: "ord_7K3M9QX2",
		VenueID:  1,
		Opened:   ev(order.OrderOpened{Channel: order.ChannelCounter, Currency: "INR", TableID: 4, Covers: 2, OpenedBy: 1}),
	}, untaxed)
	if err != nil {
		t.Fatalf("OpenOrder: %v", err)
	}
	if opened.LastSeq != 1 || opened.Status != order.StatusOpen || opened.TableID != 4 {
		t.Fatalf("opened state = %+v", opened)
	}

	state, appended, err := d.AppendOrder(ctx, orderID, untaxed, emit(
		order.LineAdded{ItemID: 1, Name: "Flat white", Qty: 2, UnitPrice: inr(22000), Station: "bar"},
		order.NoteAdded{Scope: order.ScopeLine, LineID: 2, Text: "oat milk"},
	))
	if err != nil {
		t.Fatalf("AppendOrder: %v", err)
	}
	if len(appended) != 2 || appended[0].Seq != 2 || appended[1].Seq != 3 {
		t.Fatalf("appended = %+v, want seqs 2 and 3", appended)
	}
	if state.LastSeq != 3 || state.Totals.Total != inr(44000) {
		t.Fatalf("state after append: last seq %d, total %+v", state.LastSeq, state.Totals.Total)
	}

	stored, err := d.OrderEvents(ctx, orderID, 1)
	if err != nil {
		t.Fatalf("OrderEvents: %v", err)
	}
	if !reflect.DeepEqual(stored, appended) {
		t.Errorf("events read back:\n got %+v\nwant %+v", stored, appended)
	}
	all, err := d.OrderEvents(ctx, orderID, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("OrderEvents from 0 = %d events, %v; want 3", len(all), err)
	}

	row, err := d.Queries().OrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("OrderByID: %v", err)
	}
	if row.LastSeq != 3 || row.Status != "open" || row.ClosedAt.Valid || row.Channel != "counter" {
		t.Errorf("orders row = %+v", row)
	}
	checkLoads(t, d, orderID, untaxed, state)
}

// TestSnapshotsAreEquivalentToReplay is the phase's equivalence test. An
// order is driven past several snapshot points in uneven batches, and after
// every append the snapshot path, the replay path and the append's own
// result must be the same State. Then it is settled, invoiced and closed.
func TestSnapshotsAreEquivalentToReplay(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	taxes := gstCafe()
	orderID := openTestOrder(t, d, "ord_LONGTAB", taxes)

	var state order.State
	n := 0
	for batch := 1; n < 3*SnapshotEvery; batch = batch%7 + 1 {
		payloads := make([]order.Payload, 0, batch)
		for range batch {
			n++
			switch {
			case n%5 == 0:
				payloads = append(payloads, order.NoteAdded{Scope: order.ScopeOrder, Text: fmt.Sprintf("round %d", n)})
			case n%11 == 0:
				payloads = append(payloads, order.DiscountApplied{Scope: order.ScopeOrder, Kind: order.DiscountPercent, BasisPoints: 500, Reason: "regular"})
			default:
				payloads = append(payloads, dish(n))
			}
		}
		state = mustAppend(t, d, orderID, taxes, emit(payloads...))
		checkLoads(t, d, orderID, taxes, state)
	}

	snap, err := d.Queries().OrderSnapshot(ctx, orderID)
	if err != nil {
		t.Fatalf("no snapshot after %d events: %v", state.LastSeq, err)
	}
	if want := state.LastSeq / SnapshotEvery * SnapshotEvery; snap.Seq > state.LastSeq || snap.Seq <= want-SnapshotEvery {
		t.Errorf("snapshot at seq %d, order at %d: want the one taken crossing %d", snap.Seq, state.LastSeq, want)
	}

	series, err := invoice.NewSeries("NF", "NF", invoice.IndiaFY, "{prefix}/{fy:short}/{seq:06d}", 16)
	if err != nil {
		t.Fatalf("NewSeries: %v", err)
	}
	due := state.Totals.BalanceDue
	state = mustAppend(t, d, orderID, taxes, emit(order.PaymentTendered{Method: order.PaymentCard, Amount: due, Tip: inr(0), TenderedBy: 1}))
	state = mustAppend(t, d, orderID, taxes, func(ctx context.Context, tx *OrderTx) ([]order.Event, error) {
		issued, err := invoice.Issue(ctx, tx.Invoices(), series, tx.VenueID, at, time.UTC)
		if err != nil {
			return nil, err
		}
		if err := tx.RecordInvoice(ctx, issued, at); err != nil {
			return nil, err
		}
		snapshot, err := json.Marshal(tx.State.TaxDetail)
		if err != nil {
			return nil, err
		}
		return []order.Event{
			ev(order.InvoiceIssued{InvoiceNo: issued.InvoiceNo, Series: series.Code, TaxSnapshot: snapshot}),
			ev(order.OrderClosed{ClosedBy: 1}),
		}, nil
	})
	if state.Status != order.StatusClosed || len(state.Invoices) != 1 || state.Invoices[0].InvoiceNo != "NF/26-27/000001" {
		t.Fatalf("closed state: status %s, invoices %+v", state.Status, state.Invoices)
	}
	checkLoads(t, d, orderID, taxes, state)

	row, err := d.Queries().OrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("OrderByID: %v", err)
	}
	if row.Status != "closed" || !row.ClosedAt.Valid || row.LastSeq != state.LastSeq {
		t.Errorf("orders row after close = %+v", row)
	}
}

// longOrder appends enough lines to take a snapshot and returns the state.
func longOrder(t *testing.T, d *DB, publicID string, taxes order.Taxes) (int64, order.State) {
	t.Helper()
	orderID := openTestOrder(t, d, publicID, taxes)
	payloads := make([]order.Payload, SnapshotEvery+3)
	for i := range payloads {
		payloads[i] = dish(i + 1)
	}
	return orderID, mustAppend(t, d, orderID, taxes, emit(payloads...))
}

func TestASnapshotUnderAnotherTaxRegimeIsIgnored(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	orderID, _ := longOrder(t, d, "ord_REGIME", gstCafe())

	// The guest turns out to be from Maharashtra: inter-state, so IGST
	// rather than CGST and SGST. The snapshot's tax detail is now wrong.
	interState := gstCafe()
	interState.Venue.PlaceOfSupply = "27"
	loaded, err := d.LoadOrder(ctx, orderID, interState)
	if err != nil {
		t.Fatalf("LoadOrder: %v", err)
	}
	rebuilt, err := d.RebuildOrder(ctx, orderID, interState)
	if err != nil {
		t.Fatalf("RebuildOrder: %v", err)
	}
	if !reflect.DeepEqual(loaded, rebuilt) {
		t.Fatal("a snapshot taken under intra-state GST was served for an inter-state load")
	}
	if len(loaded.TaxDetail.Components) == 0 || loaded.TaxDetail.Components[0].Name != "IGST" {
		t.Errorf("components = %+v, want IGST", loaded.TaxDetail.Components)
	}
}

func TestASnapshotThatDoesNotFoldIsReplacedByReplay(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	taxes := gstCafe()
	orderID, _ := longOrder(t, d, "ord_BADSNAP", taxes)

	snap, err := d.Queries().OrderSnapshot(ctx, orderID)
	if err != nil {
		t.Fatalf("OrderSnapshot: %v", err)
	}
	// A snapshot that claims the order was closed at its seq, which the
	// line appended after it contradicts.
	var bad order.State
	if err := json.Unmarshal([]byte(snap.State), &bad); err != nil {
		t.Fatalf("decoding snapshot: %v", err)
	}
	bad.Status = order.StatusClosed
	raw, err := json.Marshal(bad)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	if _, err := d.Writer().ExecContext(ctx, `UPDATE order_snapshots SET state = ? WHERE order_id = ?`, string(raw), orderID); err != nil {
		t.Fatalf("corrupting the snapshot: %v", err)
	}
	// With no events after it, the snapshot folds; it is the new line that
	// is refused. The append must see that the events disagree, retry from
	// them and heal the snapshot.
	state := mustAppend(t, d, orderID, taxes, emit(dish(99)))
	checkLoads(t, d, orderID, taxes, state)
	if healed, err := d.Queries().OrderSnapshot(ctx, orderID); err != nil || healed.Seq != state.LastSeq {
		t.Fatalf("snapshot after the retry: %+v, %v; want it retaken at seq %d", healed, err, state.LastSeq)
	}

	// With events after it, the read path cannot fold them onto the bad
	// snapshot and replays instead.
	state = mustAppend(t, d, orderID, taxes, emit(dish(100)))
	if _, err := d.Writer().ExecContext(ctx, `UPDATE order_snapshots SET state = ? WHERE order_id = ?`,
		strings.Replace(string(raw), fmt.Sprintf(`"LastSeq":%d`, bad.LastSeq), fmt.Sprintf(`"LastSeq":%d`, state.LastSeq-1), 1), orderID); err != nil {
		t.Fatalf("corrupting the snapshot: %v", err)
	}
	if _, err := d.Writer().ExecContext(ctx, `UPDATE order_snapshots SET seq = ? WHERE order_id = ?`, state.LastSeq-1, orderID); err != nil {
		t.Fatalf("corrupting the snapshot: %v", err)
	}
	checkLoads(t, d, orderID, taxes, state)

	// An unreadable snapshot is no snapshot at all.
	if _, err := d.Writer().ExecContext(ctx, `UPDATE order_snapshots SET state = '[]' WHERE order_id = ?`, orderID); err != nil {
		t.Fatalf("corrupting the snapshot: %v", err)
	}
	checkLoads(t, d, orderID, taxes, state)
}

func TestARefusedCommandWritesNothing(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	orderID := openTestOrder(t, d, "ord_REFUSED", untaxed)
	before := mustAppend(t, d, orderID, untaxed, emit(order.LineAdded{Name: "Flat white", Qty: 1, UnitPrice: inr(22000), Station: "bar"}))

	errDeclined := errors.New("card declined")
	tests := []struct {
		name   string
		decide Decide
		wants  error
	}{
		{"an event the projection refuses", emit(
			order.NoteAdded{Scope: order.ScopeOrder, Text: "fine on its own"},
			order.LineVoided{LineID: 99, ReasonCode: "wrong_item", AuthorisedBy: 1},
		), order.ErrLineNotFound},
		{"a command that fails after allocating", func(ctx context.Context, tx *OrderTx) ([]order.Event, error) {
			if _, err := tx.Invoices().Next(ctx, invoice.Key{VenueID: 1, Series: "NF", FYStart: 2026}); err != nil {
				return nil, err
			}
			return nil, errDeclined
		}, errDeclined},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := d.AppendOrder(ctx, orderID, untaxed, tt.decide)
			if !errors.Is(err, tt.wants) {
				t.Fatalf("err = %v, want %v", err, tt.wants)
			}
			events, err := d.OrderEvents(ctx, orderID, 0)
			if err != nil || int64(len(events)) != before.LastSeq {
				t.Fatalf("%d events after a refused command (%v), want %d", len(events), err, before.LastSeq)
			}
			checkLoads(t, d, orderID, untaxed, before)
		})
	}

	// The number the failed command took came back with its rollback.
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		seq, err := NewInvoiceAllocator(q).Next(ctx, invoice.Key{VenueID: 1, Series: "NF", FYStart: 2026})
		if err == nil && seq != 1 {
			err = fmt.Errorf("allocated %d, want 1: a failed command kept its number", seq)
		}
		return err
	})
	if err != nil {
		t.Error(err)
	}
}

// TestARefusalOnAGoodSnapshotStands is the other half of the snapshot
// check: when the events agree with the snapshot, the refusal is the order's
// and comes back as it is.
func TestARefusalOnAGoodSnapshotStands(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	taxes := gstCafe()
	orderID, state := longOrder(t, d, "ord_GOODSNAP", taxes)

	_, _, err := d.AppendOrder(ctx, orderID, taxes, emit(order.LineVoided{LineID: 999, ReasonCode: "wrong_item", AuthorisedBy: 1}))
	if !errors.Is(err, order.ErrLineNotFound) {
		t.Fatalf("err = %v, want ErrLineNotFound", err)
	}
	checkLoads(t, d, orderID, taxes, state)
}

func TestACommandWithNothingToSayAppendsNothing(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	orderID := openTestOrder(t, d, "ord_NOTHING", untaxed)
	before, err := d.LoadOrder(ctx, orderID, untaxed)
	if err != nil {
		t.Fatalf("LoadOrder: %v", err)
	}
	after, appended, err := d.AppendOrder(ctx, orderID, untaxed, emit())
	if err != nil || len(appended) != 0 || !reflect.DeepEqual(after, before) {
		t.Errorf("empty command: state changed %v, appended %d, err %v", !reflect.DeepEqual(after, before), len(appended), err)
	}
}

// TestASeqConflictIsRetriedFromTheRead forces the conflict the unique index
// exists for: another event lands at the seq a command was about to use.
// The retry reads again, decides again and appends after it.
func TestASeqConflictIsRetriedFromTheRead(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	orderID := openTestOrder(t, d, "ord_CONFLICT", untaxed)

	// interloper writes an event at the next seq behind the store's back,
	// as a second process on the same file would.
	interloper := func(ctx context.Context, tx *OrderTx) error {
		return tx.Queries().AppendOrderEvent(ctx, gen.AppendOrderEventParams{
			OrderID: orderID, Seq: tx.State.LastSeq + 1, Type: string(order.TypeNoteAdded),
			Payload: `{"scope":"order","text":"from elsewhere"}`, At: at,
		})
	}

	t.Run("once, then it succeeds", func(t *testing.T) {
		calls := 0
		state, appended, err := d.AppendOrder(ctx, orderID, untaxed, func(ctx context.Context, tx *OrderTx) ([]order.Event, error) {
			calls++
			if calls == 1 {
				if err := interloper(ctx, tx); err != nil {
					return nil, err
				}
			}
			return []order.Event{ev(order.NoteAdded{Scope: order.ScopeOrder, Text: "mine"})}, nil
		})
		if err != nil {
			t.Fatalf("AppendOrder: %v", err)
		}
		if calls != 2 || len(appended) != 1 || appended[0].Seq != 2 || len(state.Notes) != 1 {
			t.Fatalf("calls %d, appended %+v, notes %+v: want one retry and only my note at seq 2", calls, appended, state.Notes)
		}
		checkLoads(t, d, orderID, untaxed, state)
	})

	t.Run("every time, and it gives up", func(t *testing.T) {
		calls := 0
		_, _, err := d.AppendOrder(ctx, orderID, untaxed, func(ctx context.Context, tx *OrderTx) ([]order.Event, error) {
			calls++
			if err := interloper(ctx, tx); err != nil {
				return nil, err
			}
			return []order.Event{ev(order.NoteAdded{Scope: order.ScopeOrder, Text: "mine"})}, nil
		})
		if !errors.Is(err, ErrSeqConflict) || calls != maxAppendAttempts {
			t.Fatalf("err = %v after %d calls, want ErrSeqConflict after %d", err, calls, maxAppendAttempts)
		}
	})
}

// TestConcurrentAppendsLoseNothing has many terminals adding to one tab at
// once. Every line lands, the stream stays dense, and the state is the
// replay of it.
func TestConcurrentAppendsLoseNothing(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	orderID := openTestOrder(t, d, "ord_BUSYBAR", untaxed)

	const workers, each = 8, 10
	var wg sync.WaitGroup
	errs := make(chan error, workers*each)
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				line := order.LineAdded{ItemID: int64(w*100 + i), Name: "Pint", Qty: 1, UnitPrice: inr(30000), Station: "bar"}
				if _, _, err := d.AppendOrder(ctx, orderID, untaxed, emit(line)); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("AppendOrder: %v", err)
	}

	events, err := d.OrderEvents(ctx, orderID, 0)
	if err != nil {
		t.Fatalf("OrderEvents: %v", err)
	}
	if len(events) != 1+workers*each {
		t.Fatalf("%d events, want %d", len(events), 1+workers*each)
	}
	for i, e := range events {
		if e.Seq != int64(i+1) {
			t.Fatalf("event %d has seq %d: the stream is not dense", i, e.Seq)
		}
	}
	state, err := d.RebuildOrder(ctx, orderID, untaxed)
	if err != nil {
		t.Fatalf("RebuildOrder: %v", err)
	}
	if state.Totals.Total != inr(30000*workers*each) {
		t.Errorf("total %+v, want %d", state.Totals.Total, 30000*workers*each)
	}
	checkLoads(t, d, orderID, untaxed, state)
}

func TestOrderEventsAreAppendOnly(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	openTestOrder(t, d, "ord_ETCHED", untaxed)

	for _, stmt := range []string{
		`UPDATE order_events SET payload = '{"channel":"web"}'`,
		`DELETE FROM order_events`,
	} {
		if _, err := d.Writer().ExecContext(ctx, stmt); err == nil {
			t.Errorf("%s was allowed", stmt)
		}
	}
}

func TestOpenOrderRefusals(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	openTestOrder(t, d, "ord_TAKEN", untaxed)
	opened := ev(order.OrderOpened{Channel: order.ChannelCounter, Currency: "INR"})

	tests := []struct {
		name  string
		n     NewOrder
		taxes order.Taxes
		wants error
	}{
		{"a public id already in use", NewOrder{PublicID: "ord_TAKEN", VenueID: 1, Opened: opened}, untaxed, ErrPublicIDTaken},
		{"an opening the domain refuses", NewOrder{PublicID: "ord_BADCHAN", VenueID: 1,
			Opened: ev(order.OrderOpened{Channel: "fax", Currency: "INR"})}, untaxed, order.ErrInvalidPayload},
		{"an order in another currency than its taxes", NewOrder{PublicID: "ord_USD", VenueID: 1,
			Opened: ev(order.OrderOpened{Channel: order.ChannelCounter, Currency: "USD"})}, gstCafe(), order.ErrTaxConfig},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := d.OpenOrder(ctx, tt.n, tt.taxes); !errors.Is(err, tt.wants) {
				t.Errorf("err = %v, want %v", err, tt.wants)
			}
		})
	}

	t.Run("a first event that is not order.opened", func(t *testing.T) {
		n := NewOrder{PublicID: "ord_NOOPEN", VenueID: 1, Opened: ev(order.NoteAdded{Scope: order.ScopeOrder, Text: "hi"})}
		if _, _, err := d.OpenOrder(ctx, n, untaxed); err == nil {
			t.Error("an order was opened with a note")
		}
	})
	t.Run("a venue that does not exist", func(t *testing.T) {
		if _, _, err := d.OpenOrder(ctx, NewOrder{PublicID: "ord_NOVENUE", VenueID: 99, Opened: opened}, untaxed); err == nil {
			t.Error("an order was opened at venue 99")
		}
	})
}

func TestAnOrderThatDoesNotExist(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	if _, err := d.LoadOrder(ctx, 404, untaxed); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("LoadOrder: err = %v, want ErrOrderNotFound", err)
	}
	if _, _, err := d.AppendOrder(ctx, 404, untaxed, emit(order.NoteAdded{Scope: order.ScopeOrder, Text: "x"})); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("AppendOrder: err = %v, want ErrOrderNotFound", err)
	}
}

func TestOpenOrderWithinRunsInTheTransaction(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	opened := ev(order.OrderOpened{Channel: order.ChannelCounter, Currency: "INR"})

	errLater := errors.New("something after the open failed")
	var seen *OrderTx
	_, _, err := d.OpenOrder(ctx, NewOrder{PublicID: "ord_ROLLBACK", VenueID: 1, Opened: opened,
		Within: func(_ context.Context, tx *OrderTx) error {
			seen = tx
			return errLater
		}}, untaxed)
	if !errors.Is(err, errLater) {
		t.Fatalf("err = %v, want the Within error", err)
	}
	if seen == nil || seen.PublicID != "ord_ROLLBACK" || seen.State.LastSeq != 1 || seen.VenueID != 1 {
		t.Fatalf("Within saw %+v", seen)
	}
	if _, err := d.LoadOrder(ctx, seen.OrderID, untaxed); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("the order survived its Within failing: err = %v", err)
	}

	// The public ID was never taken, so it can be used again.
	if _, _, err := d.OpenOrder(ctx, NewOrder{PublicID: "ord_ROLLBACK", VenueID: 1, Opened: opened}, untaxed); err != nil {
		t.Errorf("reopening with the rolled back public ID: %v", err)
	}
}

func TestVenueStream(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedVenue(t, d)
	insertVenue(t, d, 2)

	first := openTestOrder(t, d, "ord_STREAM01", untaxed)
	mustAppend(t, d, first, untaxed, emit(dish(1), dish(2)))
	second := openTestOrder(t, d, "ord_STREAM02", untaxed)
	mustAppend(t, d, second, untaxed, emit(order.NoteAdded{Scope: order.ScopeOrder, Text: "hello"}))

	all, err := d.VenueEventsAfter(ctx, 1, 0, 0)
	if err != nil {
		t.Fatalf("VenueEventsAfter: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("%d events, want 5", len(all))
	}
	for i, e := range all {
		if i > 0 && e.ID <= all[i-1].ID {
			t.Fatalf("event %d has id %d after %d: the stream is not in order", i, e.ID, all[i-1].ID)
		}
		if e.Event.Payload == nil {
			t.Errorf("event %d has no payload", e.ID)
		}
	}
	if all[0].OrderPublicID != "ord_STREAM01" || all[0].TableID != 4 || all[0].Event.Type != order.TypeOrderOpened {
		t.Errorf("first event = %+v", all[0])
	}
	if last := all[len(all)-1]; last.OrderPublicID != "ord_STREAM02" || last.Event.Type != order.TypeNoteAdded {
		t.Errorf("last event = %+v", last)
	}

	// After a position, only what came later; and the limit is honoured.
	tail, err := d.VenueEventsAfter(ctx, 1, all[2].ID, 0)
	if err != nil || len(tail) != 2 || tail[0].ID != all[3].ID {
		t.Errorf("tail = %d events (%v)", len(tail), err)
	}
	one, err := d.VenueEventsAfter(ctx, 1, 0, 1)
	if err != nil || len(one) != 1 {
		t.Errorf("limit 1 gave %d events (%v)", len(one), err)
	}

	last, err := d.VenueLastEventID(ctx, 1)
	if err != nil || last != all[len(all)-1].ID {
		t.Errorf("VenueLastEventID = %d (%v), want %d", last, err, all[len(all)-1].ID)
	}

	// Another venue's stream is its own, and an empty one is position zero.
	other, err := d.VenueEventsAfter(ctx, 2, 0, 0)
	if err != nil || len(other) != 0 {
		t.Errorf("venue 2 = %d events (%v)", len(other), err)
	}
	if last, err := d.VenueLastEventID(ctx, 2); err != nil || last != 0 {
		t.Errorf("venue 2 last = %d (%v), want 0", last, err)
	}
}
