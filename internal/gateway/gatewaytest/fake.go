// Package gatewaytest is a payment gateway for tests: it takes no money,
// calls nobody, and does what the test tells it a provider did.
package gatewaytest

import (
	"context"
	"fmt"
	"sync"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/gateway"
)

// Fake is a gateway.Gateway held in memory.
type Fake struct {
	mu      sync.Mutex
	charges map[string]*charge
	next    int
	down    bool
	// Started is every charge Start was asked for, in order.
	Started []gateway.Charge
	// Asked counts the Status calls.
	Asked int
}

type charge struct {
	asked  gateway.Charge
	status gateway.Status
}

// New returns a fake with no charges.
func New() *Fake { return &Fake{charges: map[string]*charge{}} }

// Name implements gateway.Gateway.
func (f *Fake) Name() string { return "test" }

// Start implements gateway.Gateway.
func (f *Fake) Start(_ context.Context, c gateway.Charge) (gateway.Checkout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return gateway.Checkout{}, fmt.Errorf("%w: the fake is down", gateway.ErrUnavailable)
	}
	f.next++
	id := fmt.Sprintf("fake_%04d", f.next)
	f.charges[id] = &charge{asked: c, status: gateway.Status{State: gateway.Pending}}
	f.Started = append(f.Started, c)
	return gateway.Checkout{ID: id, URL: "https://pay.test/" + id}, nil
}

// Status implements gateway.Gateway.
func (f *Fake) Status(_ context.Context, id string) (gateway.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Asked++
	if f.down {
		return gateway.Status{}, fmt.Errorf("%w: the fake is down", gateway.ErrUnavailable)
	}
	c, ok := f.charges[id]
	if !ok {
		return gateway.Status{}, fmt.Errorf("gatewaytest: no charge %q", id)
	}
	return c.status, nil
}

// Pay has the guest pay a charge in full, by method.
func (f *Fake) Pay(id string, method gateway.Method) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.charges[id]
	c.status = gateway.Status{State: gateway.Paid, Amount: c.asked.Amount, Method: method, Reference: "pay_" + id}
}

// PayAmount has the provider report it took some other amount than asked.
func (f *Fake) PayAmount(id string, amount money.Amount) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.charges[id].status = gateway.Status{State: gateway.Paid, Amount: amount, Method: gateway.Card, Reference: "pay_" + id}
}

// Fail has a charge declined or given up on.
func (f *Fake) Fail(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.charges[id].status = gateway.Status{State: gateway.Failed}
}

// Down makes the provider unreachable, or reachable again.
func (f *Fake) Down(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}
