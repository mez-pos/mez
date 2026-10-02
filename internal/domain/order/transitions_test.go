package order

import (
	"errors"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/fsm"
	"github.com/byjoellobo/mez/internal/domain/fsm/fsmtest"
)

// The tables below are restated from docs/02-domain.md and ADR-047, not
// copied from transitions.go, so the two must agree.

func TestOrderTransitionsAreExactlyTheDomainTable(t *testing.T) {
	fsmtest.Exhaustive(t, OrderMachine(),
		[]Status{StatusOpen, StatusHeld, StatusSettling, StatusClosed, StatusReopened},
		map[Status][]Status{
			StatusOpen:     {StatusHeld, StatusSettling, StatusClosed},
			StatusHeld:     {StatusOpen},
			StatusSettling: {StatusOpen, StatusClosed},
			StatusClosed:   {StatusReopened},
			StatusReopened: {StatusClosed},
		})
}

func TestPaymentTransitionsAreExactlyTheDomainTable(t *testing.T) {
	fsmtest.Exhaustive(t, PaymentMachine(),
		[]PaymentStatus{PaymentStatusPending, PaymentStatusCaptured, PaymentStatusVoided, PaymentStatusRefunded},
		map[PaymentStatus][]PaymentStatus{
			PaymentStatusPending:  {PaymentStatusCaptured},
			PaymentStatusCaptured: {PaymentStatusVoided, PaymentStatusRefunded},
		})
}

func TestTransitionFunctions(t *testing.T) {
	tests := []struct {
		name string
		err  error
		ok   bool
	}{
		{"order open to settling", errOf(TransitionOrder(StatusOpen, StatusSettling)), true},
		{"order held to settling skips a step", errOf(TransitionOrder(StatusHeld, StatusSettling)), false},
		{"order closed to open needs a reopen", errOf(TransitionOrder(StatusClosed, StatusOpen)), false},
		{"payment captured to refunded", errOf(TransitionPayment(PaymentStatusCaptured, PaymentStatusRefunded)), true},
		{"payment voided cannot be refunded", errOf(TransitionPayment(PaymentStatusVoided, PaymentStatusRefunded)), false},
		{"payment refunded cannot be voided", errOf(TransitionPayment(PaymentStatusRefunded, PaymentStatusVoided)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ok && tt.err != nil {
				t.Errorf("err = %v, want nil", tt.err)
			}
			if !tt.ok && !errors.Is(tt.err, fsm.ErrInvalidTransition) {
				t.Errorf("err = %v, want fsm.ErrInvalidTransition", tt.err)
			}
		})
	}
}

func errOf[S any](_ S, err error) error { return err }

// TestProjectionOnlyMakesLegalMoves replays every scenario and checks each
// status change between consecutive events against the order machine. The
// projection checks this itself; this test is the proof it is not vacuous.
func TestProjectionOnlyMakesLegalMoves(t *testing.T) {
	seen := map[[2]Status]bool{}
	for _, sc := range scenarios {
		prev := Status("")
		for i := range sc.payloads {
			s := mustProject(t, sc.payloads[:i+1]...)
			if prev != "" && s.Status != prev {
				if !OrderMachine().Can(prev, s.Status) {
					t.Errorf("%s: event %d moved the order %s -> %s", sc.name, i+1, prev, s.Status)
				}
				seen[[2]Status{prev, s.Status}] = true
			}
			prev = s.Status
		}
	}
	// Every move the projection can make today should appear in some
	// scenario. Moves involving held wait for its event.
	for _, from := range OrderMachine().States() {
		for _, to := range OrderMachine().Next(from) {
			if from == StatusHeld || to == StatusHeld {
				continue
			}
			if !seen[[2]Status{from, to}] {
				t.Errorf("no scenario moves an order %s -> %s", from, to)
			}
		}
	}
}

func TestProjectionRefusesAVoidOfARefundedPaymentThroughTheMachine(t *testing.T) {
	_, err := Project(stream(opened(),
		coffee(1),               // 2
		pay(PaymentCard, 22000), // 3
		void(2),
		RefundIssued{Amount: inr(22000), AgainstPaymentID: 3, Reason: "walked out"},
		PaymentVoided{PaymentID: 3, Reason: "x", AuthorisedBy: 3},
	), untaxed)
	// A fully refunded payment carries Refunded, so the explicit check fires
	// first; both errors name the same fault.
	if !errors.Is(err, ErrPaymentNotCaptured) {
		t.Errorf("err = %v, want ErrPaymentNotCaptured", err)
	}
}

func TestProjectionRefusesAVoidOfAVoidedPaymentWithBothErrors(t *testing.T) {
	_, err := Project(stream(opened(),
		coffee(1),               // 2
		pay(PaymentCard, 22000), // 3
		PaymentVoided{PaymentID: 3, Reason: "declined", AuthorisedBy: 3},
		PaymentVoided{PaymentID: 3, Reason: "declined", AuthorisedBy: 3},
	), untaxed)
	if !errors.Is(err, ErrPaymentNotCaptured) || !errors.Is(err, fsm.ErrInvalidTransition) {
		t.Errorf("err = %v, want both ErrPaymentNotCaptured and fsm.ErrInvalidTransition", err)
	}
}

// TestTheProjectionGuardRefusesAnIllegalMove drives the guard directly. No
// legal stream reaches it, which is the point of it; this proves it would
// catch a projection that derived a move the machine forbids.
func TestTheProjectionGuardRefusesAnIllegalMove(t *testing.T) {
	p := projector{s: State{Status: StatusClosed}}
	if err := p.moveTo(StatusSettling); !errors.Is(err, fsm.ErrInvalidTransition) {
		t.Errorf("closed -> settling: err = %v, want fsm.ErrInvalidTransition", err)
	}
	if p.s.Status != StatusClosed {
		t.Errorf("a refused move changed the status to %q", p.s.Status)
	}
	if err := p.moveTo(StatusClosed); err != nil {
		t.Errorf("staying closed is not a move and must pass: %v", err)
	}
}
