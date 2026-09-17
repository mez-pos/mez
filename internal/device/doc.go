// Package device drives the hardware: ESC/POS printers over TCP 9100, the cash
// drawer, and printer discovery.
//
// The print queue is durable and retries, because a printer that was off at
// 19:40 must not lose the ticket.
//
// Imports: internal/domain for what to print. Never internal/api.
package device
