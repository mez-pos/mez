// Package tunnel keeps an outbound WebSocket open to the relay so guest phones
// can reach a server that has no public address.
//
// Outbound only, reconnecting with backoff. It needs no port forwarding, no
// static IP, and it works behind CGNAT. When it is down the counter keeps
// selling; see ADR-008.
//
// Imports: internal/config. Forwards framed requests into internal/api.
package tunnel
